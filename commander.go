package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	defaultTerminationTimeout = 250 * time.Millisecond
)

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
		return fmt.Errorf("command is empty")
	}

	c.cmd = exec.Command(args[0], args[1:]...)
	c.cmd.Dir = c.cwd
	c.cmd.Stdout = os.Stdout
	c.cmd.Stderr = os.Stderr
	c.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start: %s: %w", c.command, err)
	}

	return nil
}
func (c *commander) Terminate() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cmd == nil || c.cmd.Process == nil {
		return nil
	}

	if err := syscall.Kill(-c.cmd.Process.Pid, syscall.SIGTERM); err != nil {
		fmt.Fprintf(os.Stderr, "error sending SIGTERM: %v; attempting to kill\n", err)
		return c.forceKill()
	}

	time.Sleep(c.terminationTimeout)

	if c.cmd.ProcessState != nil && c.cmd.ProcessState.Exited() {
		return nil // Process has exited, no need to kill
	}

	return c.forceKill()
}

// forceKill forcefully terminates the process group.
func (c *commander) forceKill() error {
	if c.cmd == nil || c.cmd.Process == nil {
		return nil
	}

	fmt.Println("Force killing the process group")
	return syscall.Kill(-c.cmd.Process.Pid, syscall.SIGKILL)
}
