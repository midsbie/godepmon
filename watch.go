package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	defaultDebounceDelay = 250 * time.Millisecond
)

type watcherOption func(w *watcher)

type watcher struct {
	debounceDelay time.Duration
	watcher       *fsnotify.Watcher
	timer         *time.Timer
	mu            sync.Mutex
	done          chan error
}

func NewWatcher(options ...watcherOption) *watcher {
	w := &watcher{
		debounceDelay: defaultDebounceDelay,
	}

	for _, setopt := range options {
		setopt(w)
	}

	return w
}

func WithDelay(delay time.Duration) watcherOption {
	return func(w *watcher) {
		w.debounceDelay = delay
	}
}

func (w *watcher) Watch(path string) error {
	if w.watcher != nil {
		return fmt.Errorf("already watching")
	}

	w.done = make(chan error)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create a new watcher: %s", err)
	}
	w.watcher = watcher

	walker := DepWalker{includeExternalDeps: flags.includeExternalDeps}
	deps, err := walker.List(path)
	if err != nil {
		return err
	}

	for _, p := range deps {
		err = watcher.Add(p)
		if err != nil {
			return fmt.Errorf("failed to add path to watcher: %s (%w)\n", p, err)
		}
	}

	fmt.Printf("watching %d files...\n", len(deps))
	go w.monitor()

	// Blocking until the first event comes through.
	if err = <-w.done; err != nil {
		return err
	}

	return nil
}

func (w *watcher) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.watcher == nil {
		return nil
	}

	defer func() {
		close(w.done)
		w.watcher = nil
		w.timer = nil
	}()

	w.stopTimer()
	return w.watcher.Close()
}

func (w *watcher) Wait() chan error {
	return w.done
}

func (w *watcher) monitor() {
	for {
		select {
		case err, ok := <-w.watcher.Errors:
			if !ok {
				w.end(nil)
				return
			}
			fmt.Printf("ERROR: %s\n", err)

		case e, ok := <-w.watcher.Events:
			if !ok {
				w.end(nil)
				return
			}

			// FIXME: must pass (or determine) the containing directories of every
			//	  package so that the Create event works.
			if !e.Has(fsnotify.Create) && !e.Has(fsnotify.Remove) &&
				!e.Has(fsnotify.Write) {
				continue
			}

			w.syncRun(func() {
				if w.timer != nil {
					w.stopTimer()
				}

				w.timer = time.AfterFunc(w.debounceDelay, func() {
					w.process(e)
				})
			})
		}
	}
}

func (w *watcher) process(e fsnotify.Event) {
	fmt.Println(e.String())

	w.syncRun(w.stopTimer)
	w.end(nil)
}

func (w *watcher) stopTimer() {
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}
}

func (w *watcher) end(err error) {
	select {
	case w.done <- err:
	default:
		// Handling the case where the error cannot be sent because the channel is full or
		// no receiver is ready.
	}
}

func (w *watcher) syncRun(f func()) {
	w.mu.Lock()
	defer w.mu.Unlock()

	f()
}
