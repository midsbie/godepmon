package main

import (
	"fmt"
	"os"
)

func Error(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func Fatal(format string, args ...interface{}) {
	Error(format, args...)
	os.Exit(1)
}
