//go:build linux

package cliinstall

import "github.com/nextlevelbuilder/goclaw/internal/skills"

// validateBinary validates an ELF binary on Linux.
func validateBinary(content []byte) error {
	return skills.ValidateELF(content)
}
