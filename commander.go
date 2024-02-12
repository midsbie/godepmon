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
	defaultTerminationTimeout = 250 * time.Millisecond
)

type EmptyCommandError struct{}

func (e *EmptyCommandError) Error() string {
	return "Command is empty"
}

type StartCommandError struct {
	Command string
	Err     error
}

func (e *StartCommandError) Error() string {
	return fmt.Sprintf("Failed to start command '%s'\n%v", e.Command, e.Err)
}

type ForceKillError struct {
	Pid int
	Err error
}

func (e *ForceKillError) Error() string {
	return fmt.Sprintf("Error force-killing the process group (PID %d)\n%v", e.Pid, e.Err)
}

type commanderOption func(c *commander)

type commander struct {
	terminationTimeout time.Duration
	cwd                string
	command            string
	cmd                *exec.Cmd
	mu                 sync.Mutex
}

func NewCommander(cwd string, command string) *commander {
	return &commander{terminationTimeout: defaultTerminationTimeout, cwd: cwd, command: command}
}

func WithTerminationTimeout(timeout time.Duration) commanderOption {
	return func(c *commander) {
		c.terminationTimeout = timeout
	}
}

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

// forceKill forcefully terminates the process group.
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
