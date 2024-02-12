package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
)

const (
	// defaultDebounceDelay specifies the default delay duration used for debouncing file system
	// events.
	defaultDebounceDelay = 250 * time.Millisecond
)

// WatcherAlreadyRunningError indicates an error when starting a watcher that is already running.
type WatcherAlreadyRunningError struct{}

func (e *WatcherAlreadyRunningError) Error() string {
	return "Watcher is already running"
}

// WatcherCreationError wraps an error encountered during the creation of a new file system watcher.
type WatcherCreationError struct {
	Err error
}

func (e *WatcherCreationError) Error() string {
	return fmt.Sprintf("Failed to create a new watcher\n%v", e.Err)
}

// WatcherDepWalkerError wraps an error encountered while determining dependencies to be watched.
type WatcherDepWalkerError struct {
	Err error
}

func (e *WatcherDepWalkerError) Error() string {
	return fmt.Sprintf("Failed to determine dependencies\n%v", e.Err)
}

// PathAdditionError wraps an error encountered when adding a path to the watcher.
type PathAdditionError struct {
	Path string
	Err  error
}

func (e *PathAdditionError) Error() string {
	return fmt.Sprintf("Failed to add path '%s' to watcher\n%v", e.Path, e.Err)
}

// WatcherEventError wraps an error related to an event during file watching.
type WatcherEventError struct {
	Err error
}

func (e *WatcherEventError) Error() string {
	return fmt.Sprintf("Error occurred while watching files\n%v", e.Err)
}

// watcherOption defines a function signature for options that configure a watcher instance.
type watcherOption func(w *watcher)

// watcher encapsulates the logic for watching file system events with debounce handling.
type watcher struct {
	debounceDelay time.Duration
	watcher       *fsnotify.Watcher
	timer         *time.Timer
	mu            sync.Mutex
	done          chan error
	closed        bool
}

// NewWatcher creates a new watcher instance configured with the provided options.
func NewWatcher(options ...watcherOption) *watcher {
	w := &watcher{
		debounceDelay: defaultDebounceDelay,
	}

	for _, setopt := range options {
		setopt(w)
	}

	return w
}

// WithDelay configures the debounce delay for a watcher instance.
func WithDelay(delay time.Duration) watcherOption {
	return func(w *watcher) {
		w.debounceDelay = delay
	}
}

// Watch starts the watcher on the specified path. It returns an error if the watcher is already
// running or fails to start.
func (w *watcher) Watch(path string) error {
	if w.watcher != nil {
		return &WatcherAlreadyRunningError{}
	}

	w.done = make(chan error)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return &WatcherCreationError{Err: err}
	}
	w.watcher = watcher

	walker := DepWalker{includeExternalDeps: flags.includeExternalDeps}
	deps, err := walker.List(path)
	if err != nil {
		return &WatcherDepWalkerError{Err: err}
	}

	for _, p := range deps {
		err = watcher.Add(p)
		if err != nil {
			return &PathAdditionError{Path: p, Err: err}
		}
	}

	log.Info().Msgf("watching %d files...", len(deps))
	go w.monitor()

	// Blocking until the first event comes through.
	if err = <-w.done; err != nil {
		return err
	}

	return nil
}

// Close terminates the watcher, ensuring all resources are properly released.
func (w *watcher) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.watcher == nil {
		log.Trace().Msg("not closing watcher: not running")
		return nil
	} else if w.closed {
		panic("watcher should not be closed")
	}

	log.Trace().Msg("closing watcher")

	tw := w.watcher

	w.stopTimer()
	close(w.done)
	w.closed = true
	w.watcher = nil

	return tw.Close()
}

// Wait returns a channel that is closed when the watcher stops watching.
func (w *watcher) Wait() chan error {
	return w.done
}

// monitor starts the event monitoring loop, processing file system events.
func (w *watcher) monitor() {
	for {
		select {
		case err, ok := <-w.watcher.Errors:
			if !ok {
				log.Trace().Msg("watcher error received but channel closed")
				w.end(nil)
				return
			}
			log.Error().Msgf("error occurred while watching files: %v", err)

		case e, ok := <-w.watcher.Events:
			if !ok {
				log.Warn().Msg("event received but channel closed")
				w.end(nil)
				return
			}

			// FIXME: must pass (or determine) the containing directories of every
			//	  package so that the Create event works.
			if !e.Has(fsnotify.Create) && !e.Has(fsnotify.Remove) &&
				!e.Has(fsnotify.Write) {
				log.Trace().Msgf("ignoring event: %s %s", e.Op.String(), e.Name)
				continue
			}

			log.Trace().Msgf("processing event: %s %s", e.Op.String(), e.Name)
			w.syncRun(func() {
				if w.timer != nil {
					w.stopTimer()
				}

				log.Trace().Msgf("setting up timer")
				w.timer = time.AfterFunc(w.debounceDelay, func() {
					w.syncRun(func() {
						w.process(e)
					})
				})
			})
		}
	}
}

// process handles a single file system event.
func (w *watcher) process(e fsnotify.Event) {
	log.Info().Msgf("%s %s", e.Op.String(), e.Name)
	w.stopTimer()
	w.end(nil)
}

// stopTimer stops the debounce timer if it is running.
func (w *watcher) stopTimer() {
	if w.timer != nil {
		log.Debug().Msg("stopping timer")
		w.timer.Stop()
		w.timer = nil
	}
}

// end signals the completion of event processing, optionally with an error.
func (w *watcher) end(err error) {
	if w.closed {
		log.Trace().Msg("not ending: channel closed")
		return
	}

	select {
	case w.done <- err:
		if err == nil {
			log.Debug().Msg("ended without errors")
		} else {
			log.Debug().Msgf("ended with error: %s", err.Error())
		}
	default:
		// Handling the case where the error cannot be sent because the channel is full or
		// no receiver is ready.
	}
}

// syncRun executes a function within the watcher's mutex lock.
func (w *watcher) syncRun(f func()) {
	w.mu.Lock()
	defer w.mu.Unlock()

	f()
}
