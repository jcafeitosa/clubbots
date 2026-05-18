package cliinstall

import (
	"encoding/binary"
	"errors"
	"fmt"
	"runtime"
)

// Mach-O validation errors.
var (
	ErrNotMachO          = errors.New("not a Mach-O binary")
	ErrUnsupportedMachO  = errors.New("only 64-bit Mach-O supported")
	ErrMachOArchMismatch = errors.New("Mach-O architecture mismatch")
)

// validateBinary validates a Mach-O binary on macOS.
func validateBinary(content []byte) error {
	return validateMachO(content)
}

// validateMachO checks magic bytes, 64-bit class, and CPU type matches runtime.
func validateMachO(content []byte) error {
	if len(content) < 28 {
		return ErrNotMachO
	}

	magic := binary.BigEndian.Uint32(content[:4])
	switch magic {
	case 0xFEEDFACF: // 64-bit Mach-O
		// OK
	case 0xFEEDFACE: // 32-bit Mach-O
		return ErrUnsupportedMachO
	case 0xCAFEBABE: // fat binary (big-endian)
		return validateFatBinary(content, binary.BigEndian)
	case 0xBEBAFECA: // fat binary (little-endian)
		return validateFatBinary(content, binary.LittleEndian)
	default:
		return ErrNotMachO
	}

	cpuType := binary.LittleEndian.Uint32(content[4:8])
	wantCPU := uint32(0x01000007) // CPU_TYPE_X86_64
	if runtime.GOARCH == "arm64" {
		wantCPU = 0x0100000C // CPU_TYPE_ARM64
	}
	if cpuType != wantCPU {
		return fmt.Errorf("%w (binary=0x%X, runtime=%s)", ErrMachOArchMismatch, cpuType, runtime.GOARCH)
	}
	return nil
}

// validateFatBinary checks a fat (universal) binary contains a slice matching the runtime arch.
func validateFatBinary(content []byte, bo binary.ByteOrder) error {
	if len(content) < 12 {
		return ErrNotMachO
	}
	numArchs := bo.Uint32(content[4:8])
	if numArchs == 0 || numArchs > 16 {
		return ErrNotMachO
	}
	wantCPU := uint32(0x01000007)
	if runtime.GOARCH == "arm64" {
		wantCPU = 0x0100000C
	}
	// Each fat_arch entry is 20 bytes: cpu_type(4) cpu_subtype(4) offset(4) size(4) align(4)
	for i := range numArchs {
		off := 8 + int(i)*20
		if off+20 > len(content) {
			return ErrNotMachO
		}
		cpuType := bo.Uint32(content[off : off+4])
		if cpuType == wantCPU {
			return nil
		}
	}
	return fmt.Errorf("%w: fat binary missing %s slice", ErrMachOArchMismatch, runtime.GOARCH)
}
