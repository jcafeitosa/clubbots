//go:build !linux && !darwin

package cliinstall

import (
	"errors"
	"runtime"
)

var errUnsupportedOS = errors.New("cliinstall: binary validation not supported on " + runtime.GOOS)

// validateBinary is a stub for unsupported platforms.
func validateBinary(content []byte) error {
	return errUnsupportedOS
}
