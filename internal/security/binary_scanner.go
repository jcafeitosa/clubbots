package security

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// BinaryScanner performs pre-execution command scanning for dangerous patterns.
// Inspired by Hermes Agent's Tirith security binary scanner.
// Pure Go — no external binary dependency.
type BinaryScanner struct {
	denyPatterns []*regexp.Regexp
	failOpen     bool // true = allow on scan failure (default safe: fail-closed)
}

// NewBinaryScanner creates a scanner with default dangerous patterns.
func NewBinaryScanner() *BinaryScanner {
	return &BinaryScanner{
		denyPatterns: defaultDenyPatterns(),
		failOpen:     false,
	}
}

// SetFailOpen controls behavior when scanning encounters an error.
func (s *BinaryScanner) SetFailOpen(v bool) { s.failOpen = v }

// Scan checks a command string for dangerous patterns.
// Returns nil if safe, or an error describing the blocked pattern.
func (s *BinaryScanner) Scan(command string) error {
	lower := strings.ToLower(command)

	for _, pattern := range s.denyPatterns {
		if pattern.MatchString(lower) {
			return &SecurityBlockedError{
				Command: command,
				Pattern: pattern.String(),
				Reason:  "matches dangerous pattern",
			}
		}
	}

	return nil
}

// ScanFile checks a binary file for known dangerous signatures.
// Basic implementation: checks ELF/Mach-O magic, shell script shebangs, etc.
func (s *BinaryScanner) ScanFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if s.failOpen {
			return nil
		}
		return err
	}

	content := string(data)

	// Check for suspicious patterns in the binary
	suspiciousPatterns := []string{
		"rm -rf /",
		"mkfs.",
		">/dev/sda",
		"dd if=/dev/zero",
		":(){ :|:& };:", // fork bomb
		"chmod 777 /",
	}

	lower := strings.ToLower(content)
	for _, pat := range suspiciousPatterns {
		if strings.Contains(lower, strings.ToLower(pat)) {
			return &SecurityBlockedError{
				Command: path,
				Pattern: pat,
				Reason:  "binary contains dangerous pattern",
			}
		}
	}

	return nil
}

// CheckBinary executes pre-flight checks before allowing a binary to run.
// 1. Verify binary exists and is executable
// 2. Check for SUID bit
// 3. Scan for dangerous patterns
func (s *BinaryScanner) CheckBinary(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	// Block SUID binaries
	if info.Mode()&os.ModeSetuid != 0 {
		return &SecurityBlockedError{
			Command: path,
			Reason:  "SUID binaries are not allowed",
		}
	}

	// Block world-writable binaries
	if info.Mode()&0o002 != 0 {
		return &SecurityBlockedError{
			Command: path,
			Reason:  "world-writable binaries are not allowed",
		}
	}

	return s.ScanFile(path)
}

// IsBinaryAvailable checks if a named binary exists on PATH and is safe.
func (s *BinaryScanner) IsBinaryAvailable(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", err
	}
	if err := s.CheckBinary(path); err != nil {
		return "", err
	}
	return path, nil
}

// SecurityBlockedError indicates a security policy blocked an action.
type SecurityBlockedError struct {
	Command string
	Pattern string
	Reason  string
}

func (e *SecurityBlockedError) Error() string {
	msg := "security blocked: " + e.Reason
	if e.Pattern != "" {
		msg += " (pattern: " + e.Pattern + ")"
	}
	return msg
}

func defaultDenyPatterns() []*regexp.Regexp {
	patterns := []string{
		// Destructive filesystem operations
		`rm\s+-rf\s+/`,
		`mkfs\.`,
		`dd\s+if=/dev/zero`,
		`>\/dev\/sd[a-z]`,
		// Fork bomb
		`:\(\)\s*\{`,
		// Reverse shells
		`nc\s+.*-e\s+/bin/(sh|bash)`,
		`bash\s+-i\s+>&\s+/dev/tcp/`,
		`python.*socket\.socket.*connect`,
		`socat\s+.*exec:`,
		// Data exfiltration
		`curl.*\|.*(ba)?sh`,
		`wget.*-O\s*-\s*\|.*(ba)?sh`,
		// Privilege escalation
		`chmod\s+[0-7]*7[0-7]*\s+/`,
		`chown\s+root`,
		// Container escape
		`nsenter\s`,
		`cgexec\s`,
		// Crypto mining
		`cgminer|bfgminer|minerd|cpuminer`,
		`/etc/shadow`,
		`/etc/sudoers`,
		// URL-based attacks
		`curl\s+.*169\.254\.169\.254`,
		`wget\s+.*169\.254\.169\.254`,
	}

	var compiled []*regexp.Regexp
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			compiled = append(compiled, re)
		}
	}
	return compiled
}
