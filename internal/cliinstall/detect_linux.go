//go:build linux

package cliinstall

import (
	"os"
	"path/filepath"
)

func commonInstallPaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}
	return []string{
		filepath.Join(home, ".goclaw", "data", ".runtime", "bin"),
		"/usr/local/bin",
		"/usr/bin",
	}
}
