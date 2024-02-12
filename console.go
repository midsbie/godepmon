package main

import (
	"fmt"
	"os"
)

// Error writes an error message formatted according to a format specifier and arguments to the
// standard error stream. It appends a newline to the output.
func Error(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// Fatal is similar to Error but additionally exits the program with a status code of 1, indicating
// an abnormal termination.
func Fatal(format string, args ...interface{}) {
	Error(format, args...)
	os.Exit(1)
}
