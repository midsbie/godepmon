# Godepmon

Godepmon is a real-time monitoring tool designed to enhance the Go development workflow by
automatically observing changes in Go packages and their dependencies. Upon detecting any
modifications, it executes a predefined command, offering immediate feedback and streamlining the
development process.

## Features

* Watches over your Go package and its dependencies for any changes, ensuring your project is always
  up-to-date with the latest modifications.
* Ideal for Docker development, Godepmon automates rebuilds and eliminates manual container restarts
  with every code change.
* Unlike simplistic file watchers, it smartly scans for dependencies, ensuring that only relevant
  changes trigger the execution command. This approach avoids unnecessary builds or tests when
  unrelated files are modified.
* Executes a specified command (e.g., `go run .`, `go build`, `go test`) automatically upon
  detecting changes, facilitating a smooth and efficient development experience.
* Provides the flexibility to include or exclude external dependencies in the monitoring process.

## Getting Started

### Installing

```bash
go install github.com/midsbie/godepmon@latest
```

### Usage

To start monitoring your Go package and execute a command when changes are made, simply run:

```bash
godepmon [flags] [path] [--] [command]
```

Positional arguments:

* `path`: Optional. Specifies the Go package path to monitor. Defaults to the current directory if
  not provided.
* `command`: Optional. Specifies the command to execute when changes are detected. Defaults to `go
  run .` at given path.

Flags:

* `--include-external-deps`: Include external dependencies in the monitoring process.
* `-v`, `--verbose`: Increase verbosity. Use multiple times for more verbose output (up to three
   levels; e.g. `-vvv`).

### Examples

Monitor the current directory and execute go test upon detecting changes:

```bash
godepmon -- ./go test
```

Monitor a specific package and include external dependencies:

```bash
godepmon --include-external-deps ./path/to/package -- go build -v
```

## Contributing

Contributions are what make the open-source community such an amazing place to learn, inspire, and
create. All contributions are greatly appreciated.

## License

Distributed under the MIT License. See LICENSE for more information.