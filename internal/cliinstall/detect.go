package cliinstall

import (
	"os"
	"os/exec"
	"path/filepath"
)

// Detect checks if a binary is available on PATH or at common install paths.
// Returns the resolved absolute path and true if found.
func Detect(binary string) (string, bool) {
	for _, dir := range commonInstallPaths() {
		path := filepath.Join(dir, binary)
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() && isExecutable(fi) {
			return path, true
		}
	}
	if path, err := exec.LookPath(binary); err == nil {
		return path, true
	}
	return "", false
}

func isExecutable(fi os.FileInfo) bool {
	return fi.Mode()&0o111 != 0
}
