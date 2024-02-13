// This package implements godepmon, a tool for automatically monitoring Go packages and their
// dependencies for changes, and executing a specified command upon detection of any changes. It is
// designed to streamline the development workflow by providing real-time feedback.
package main

import (
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/spf13/cobra"
)

const (
	// defaultCommand defines the default command to execute when changes are detected and no
	// specific command has been provided by the user.
	defaultCommand = "go run ."
)

// rootCmd defines the base command of godepmon.
var rootCmd = &cobra.Command{
	Use:   "godepmon [flags] [path] [--] [command]",
	Short: "Automatically monitors a Go package along with its dependencies for any changes, triggering a specified command upon detection.",
	Long: `Godepmon provides a real-time monitoring solution for Go packages, observing any changes in the package itself or its dependencies. Upon detecting a change, it automatically executes a command, facilitating immediate feedback and actions like rebuilding or testing. This tool is especially useful for developers looking to streamline their development workflow by automating reaction to changes in their project environment.

The tool accepts an optional PATH as an argument, which specifies the Go package to monitor; and a COMMAND, which specifies the command to run when a change is detected. Flags can be used to customize the monitoring and execution behavior, making Godepmon a flexible tool for various development scenarios.

If PATH is not specified, the current working directory is assumed.  If COMMAND is not specified, 'go run .' is executed.  If intending to specify COMMAND, make sure PATH is given.`,
	// Args: cobra.MaximumNArgs(2),
	Run: run,
}

// programFlags defines the flags that can be passed to godepmon via the command line.  It allows
// users to customize the behavior of the tool, such as including external dependencies in the
// monitoring process and adjusting verbosity.
type programFlags struct {
	includeExternalDeps bool
	verbose             int
}

// flags holds the actual values of the command line flags after they have been parsed.
var flags programFlags = programFlags{}

// init initializes the command line interface, setting up flags and adjusting the logging
// configuration based on user input.
func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:             os.Stdout,
		FormatTimestamp: func(i interface{}) string { return "" },
		NoColor:         false,
	})

	f := rootCmd.Flags()
	f.BoolVar(&flags.includeExternalDeps, "include-external-deps", false,
		"Also include external dependencies (default: include module imports only)")

	rootCmd.PersistentFlags().
		CountVarP(&flags.verbose, "verbose", "v",
			"Increase verbosity. Use multiple times for more verbose output (up to three levels; e.g., -vvv).")

	cobra.OnInitialize(func() {
		// Adjust the global logging level based on the verbosity count
		switch flags.verbose {
		case 0:
			zerolog.SetGlobalLevel(zerolog.WarnLevel)
		case 1:
			zerolog.SetGlobalLevel(zerolog.InfoLevel)
		default:
			zerolog.SetGlobalLevel(zerolog.TraceLevel)
		}
	})
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		Fatal("Fatal error occurred:\n%v", err)
	}
}

// run is the main execution logic of the root command. It sets up signal handling for graceful
// shutdown and orchestrates the monitoring and command execution process.
func run(cmd *cobra.Command, args []string) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	path, command := processArgs(args)
	runner := NewCommander(path, command)
	defer runner.Terminate()

	go func() {
		<-signals
		log.Info().Msg("received interrupt signal, terminating...")
		if err := runner.Terminate(); err != nil {
			Fatal(err.Error())
		}
		os.Exit(0)
	}()

	for {
		runOnce(path, runner)
	}
}

// runOnce performs a single cycle of monitoring and command execution.  It starts the monitoring
// process, waits for changes, and then executes the specified command.
func runOnce(path string, runner *commander) {
	watcher := NewWatcher()
	go watcher.Watch(path)
	defer watcher.Close()

	if err := runner.Start(); err != nil {
		Fatal(err.Error())
	}

	err := <-watcher.Wait()
	log.Debug().Msg("terminating program")
	if terr := runner.Terminate(); terr != nil {
		Error(terr.Error())
	}
	if err != nil {
		Fatal(err.Error())
	}
}

// processArgs processes the command line arguments to determine the path to monitor and the command
// to execute. It handles default values and argument parsing logic.
func processArgs(args []string) (string, string) {
	// Attempt to find index of "--" arg
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
			Fatal("Unable to obtain current directory\n%v", err)
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
		Fatal("Path does not exist: %s", path)
	} else if !stat.IsDir() {
		path = filepath.Dir(path)
	}

	return path, command
}
