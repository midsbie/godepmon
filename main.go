package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

const (
	defaultCommand = "go run ."
)

var rootCmd = &cobra.Command{
	Use:   "godepmon [flags] [path] [--] [command]",
	Short: "Automatically monitors a Go package along with its dependencies for any changes, triggering a specified command upon detection.",
	Long: `Godepmon provides a real-time monitoring solution for Go packages, observing any changes in the package itself or its dependencies. Upon detecting a change, it automatically executes a command, facilitating immediate feedback and actions like rebuilding or testing. This tool is especially useful for developers looking to streamline their development workflow by automating reaction to changes in their project environment.

The tool accepts an optional PATH as an argument, which specifies the Go package to monitor; and a COMMAND, which specifies the command to run when a change is detected. Flags can be used to customize the monitoring and execution behavior, making Godepmon a flexible tool for various development scenarios.

If PATH is not specified, the current working directory is assumed.  If COMMAND is not specified, 'go run .' is executed.`,
	// Args: cobra.MaximumNArgs(2),
	Run: run,
}

type programFlags struct {
	includeExternalDeps bool
	verbose             bool
}

var flags programFlags = programFlags{}

func init() {
	f := rootCmd.Flags()
	f.BoolVar(&flags.includeExternalDeps, "include-external-deps", false,
		"Also include external dependencies (default: include module imports only)")
	f.BoolVarP(&flags.verbose, "verbose", "v", false,
		"Verbose mode")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	path, command := processArgs(args)
	runner := NewCommander(path, command)
	defer runner.Terminate()

	go func() {
		<-signals
		fmt.Println("\nReceived interrupt signal, terminating...")
		if err := runner.Terminate(); err != nil {
			fmt.Fprintf(os.Stderr, "Error terminating command: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()

	for {
		runOnce(path, runner)
	}
}

func runOnce(path string, runner *commander) {
	watcher := NewWatcher()
	go watcher.Watch(path)
	defer watcher.Close()

	if err := runner.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	err := <-watcher.Wait()
	if terr := runner.Terminate(); terr != nil {
		fmt.Fprintln(os.Stderr, terr.Error())
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func processArgs(args []string) (string, string) {
	// Find the index of "--" and remove it if present
	sepidx := -1
	for i, arg := range args {
		if arg == "--" {
			sepidx = i
			break
		}
	}

	// Remove "--" from args if found
	if sepidx >= 0 {
		args = append(args[:sepidx], args[sepidx+1:]...)
	}

	var path, command string
	if len(args) < 1 {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error obtaining current directory: %v\n", err)
			os.Exit(1)
		}

		return cwd, command
	}

	for i, s := range args {
		args[i] = strings.TrimSpace(s)
	}

	path = args[0]
	if len(args) > 1 {
		parts := args[1:]
		command = strings.Join(parts, " ")
	} else {
		command = defaultCommand
	}

	if stat, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "path does not exist: %s\n", path)
		os.Exit(1)
	} else if !stat.IsDir() {
		path = filepath.Dir(path)
	}

	return path, command
}
