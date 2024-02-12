package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type GoMod struct {
	// The absolute path to the go.mod file
	path string
	// The module path as specified in the go.mod file
	module string
}

// NewGoMod initializes a GoMod struct with the path to the go.mod file.
// It takes a directory path as input and finds the go.mod file by traversing up the directory tree.
func NewGoMod(path string) (*GoMod, error) {
	goModPath, err := FindGoModFile(path)
	if err != nil {
		return nil, err
	}

	return &GoMod{path: goModPath}, nil
}

// Path returns the absolute path of the go.mod file.
func (gm *GoMod) Path() string {
	return gm.path
}

// Module reads the go.mod file to extract and return the module path.
// It caches the result for subsequent calls.
func (gm *GoMod) Module() (string, error) {
	if gm.module != "" {
		return gm.module, nil
	}

	file, err := os.Open(gm.path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "module ") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid 'module' directive: %s", gm.path)
		}

		gm.module = parts[1]
		return gm.module, nil
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", fmt.Errorf("'module' directive not found: %s", gm.path)
}

// FindGoModFile searches for a go.mod file starting from the specified directory path and moving
// upwards through the directory tree until the file is found or the root of the file system is
// reached.  The function returns the absolute path to the go.mod file if found, or an error if not
// found.
func FindGoModFile(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	for {
		goModPath := filepath.Join(path, "go.mod")
		if _, err := os.Stat(goModPath); os.IsNotExist(err) {
			parentDir := filepath.Dir(path)
			if parentDir == path {
				return "", fmt.Errorf("go.mod file not found")
			}
			path = parentDir
			continue
		}

		file, err := os.Open(goModPath)
		if err != nil {
			return "", err
		}
		defer file.Close()
		return goModPath, nil
	}
}
