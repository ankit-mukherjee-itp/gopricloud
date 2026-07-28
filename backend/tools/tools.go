// Package tools holds small, dependency-free project-root helpers.
package tools

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindRootDir walks up from the working directory until it finds the directory
// holding go.mod, so files kept beside it (.env, clouds.yaml) can be located
// regardless of which subdirectory the process was started from.
func FindRootDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if isRootDir(cwd) {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			break
		}
		cwd = parent
	}

	return "", fmt.Errorf("root directory not found")
}

func isRootDir(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "go.mod"))
	return err == nil
}
