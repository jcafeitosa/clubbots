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
		filepath.Join(home, "Library", "Application Support", "GoClaw", "runtime", "bin"),
		"/opt/homebrew/bin",
		"/usr/local/bin",
	}
}
