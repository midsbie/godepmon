package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	// defaultTerminationTimeout specifies the default timeout duration for the termination of
	// the command process via SIGTERM signalling.
	defaultTerminationTimeout = 250 * time.Millisecond
)

// EmptyCommandError represents an error that occurs when an attempt is made to start a commander
// with an empty command string.
type EmptyCommandError struct{}

func (e *EmptyCommandError) Error() string {
	return "Command is empty"
}

// StartCommandError represents an error that occurs when starting the command fails.
type StartCommandError struct {
	Command string
	Err     error
}

func (e *StartCommandError) Error() string {
	return fmt.Sprintf("Failed to start command '%s'\n%v", e.Command, e.Err)
}

// ForceKillError represents an error that occurs when force-killing the process group fails.
type ForceKillError struct {
	Pid int
	Err error
}

func (e *ForceKillError) Error() string {
	return fmt.Sprintf("Error force-killing the process group (PID %d)\n%v", e.Pid, e.Err)
}

// commanderOption defines a function signature for options that can be passed to NewCommander to
// configure a commander instance.
type commanderOption func(c *commander)

// commander encapsulates command execution logic, allowing for starting and terminating system
// commands.
type commander struct {
	terminationTimeout time.Duration
	cwd                string
	command            string
	cmd                *exec.Cmd
	mu                 sync.Mutex
}

// NewCommander creates a new commander instance with the specified working directory and
// command. It returns a pointer to the created commander instance.
func NewCommander(cwd string, command string) *commander {
	return &commander{terminationTimeout: defaultTerminationTimeout, cwd: cwd, command: command}
}

// WithTerminationTimeout is an option function for NewCommander that configures a custom
// termination timeout for a commander instance.
func WithTerminationTimeout(timeout time.Duration) commanderOption {
	return func(c *commander) {
		c.terminationTimeout = timeout
	}
}

// Start initiates the execution of the commander's command. It locks the commander instance,
// prepares the command for execution, and starts it. An error is returned if the command fails to
// start.
func (c *commander) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	args := strings.Fields(c.command)
	if len(args) == 0 {
		return &EmptyCommandError{}
	}

	c.cmd = exec.Command(args[0], args[1:]...)
	c.cmd.Dir = c.cwd
	c.cmd.Stdout = os.Stdout
	c.cmd.Stderr = os.Stderr
	c.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	log.Info().Msgf("running program: %s", c.cmd)
	if err := c.cmd.Start(); err != nil {
		return &StartCommandError{Command: c.command, Err: err}
	}

	log.Info().Msgf("program running (PID %d)", c.cmd.Process.Pid)
	return nil
}

// Terminate attempts to gracefully terminate the command process. If SIGTERM fails, it falls back
// to force-killing the process group.  An error is returned if force-killing the process group
// fails.
func (c *commander) Terminate() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cmd == nil || c.cmd.Process == nil {
		log.Debug().Msgf("not terminating program: not running")
		return nil
	}

	log.Info().Msgf("terminating process group (PID %d)", c.cmd.Process.Pid)
	if err := syscall.Kill(-c.cmd.Process.Pid, syscall.SIGTERM); err != nil {
		log.Warn().Msgf("error sending SIGTERM to process group (PID %d): %v",
			c.cmd.Process.Pid, err.Error())
		return c.forceKill()
	}

	// FIXME: improve this so as to receive a signal when the process group terminates and not
	//	  have to always sleep here.
	time.Sleep(c.terminationTimeout)

	if c.cmd.ProcessState != nil && c.cmd.ProcessState.Exited() {
		return nil
	}

	return c.forceKill()
}

// forceKill forcefully terminates the process group associated with the commander's command. An
// error is returned if the operation fails.
func (c *commander) forceKill() error {
	if c.cmd == nil || c.cmd.Process == nil {
		log.Debug().Msgf("not forcefully killing program: not running")
		return nil
	}

	log.Info().Msgf("forcefully killing process group (PID %d)", c.cmd.Process.Pid)
	if err := syscall.Kill(-c.cmd.Process.Pid, syscall.SIGKILL); err != nil {
		return &ForceKillError{Pid: c.cmd.Process.Pid, Err: err}
	}

	return nil
}
