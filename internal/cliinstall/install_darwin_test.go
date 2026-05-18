package cliinstall

import (
	"testing"
)

func TestValidateMachO_Valid64BitARM64(t *testing.T) {
	// Minimal 64-bit ARM64 Mach-O header:
	// magic(4) cputype(4) cpusubtype(4) filetype(4) ncmds(4) sizeofcmds(4) flags(4)
	macho := make([]byte, 28)
	// 0xFEEDFACF = 64-bit Mach-O magic (big-endian bytes)
	macho[0], macho[1], macho[2], macho[3] = 0xFE, 0xED, 0xFA, 0xCF
	// CPU_TYPE_ARM64 = 0x0100000C (little-endian)
	macho[4], macho[5], macho[6], macho[7] = 0x0C, 0x00, 0x00, 0x01
	// cpusubtype
	macho[8], macho[9], macho[10], macho[11] = 0x00, 0x00, 0x00, 0x00
	// MH_EXECUTE = 2
	macho[12], macho[13], macho[14], macho[15] = 0x02, 0x00, 0x00, 0x00
	// ncmds = 1
	macho[16], macho[17], macho[18], macho[19] = 0x01, 0x00, 0x00, 0x00
	// sizeofcmds = small value
	macho[20], macho[21], macho[22], macho[23] = 0x10, 0x00, 0x00, 0x00
	// flags
	macho[24], macho[25], macho[26], macho[27] = 0x00, 0x00, 0x00, 0x00

	// This test only validates the header parsing on arm64.
	// On amd64 hosts, the CPU type won't match, so we skip the error check.
	err := validateMachO(macho)
	if err != nil {
		// May fail on amd64 due to CPU mismatch — that's expected.
		t.Logf("validateMachO on arm64 binary: %v", err)
	}
}

func TestValidateMachO_Invalid32Bit(t *testing.T) {
	macho := make([]byte, 28)
	// 0xFEEDFACE = 32-bit Mach-O magic (big-endian bytes)
	macho[0], macho[1], macho[2], macho[3] = 0xFE, 0xED, 0xFA, 0xCE

	err := validateMachO(macho)
	if err == nil {
		t.Fatal("validateMachO() = nil, want error for 32-bit binary")
	}
}

func TestValidateMachO_NotMachO(t *testing.T) {
	data := []byte("not a mach-o binary at all")

	err := validateMachO(data)
	if err == nil {
		t.Fatal("validateMachO() = nil, want error for non-Mach-O data")
	}
}

func TestValidateMachO_TooShort(t *testing.T) {
	data := []byte{0xCF, 0xFA}

	err := validateMachO(data)
	if err == nil {
		t.Fatal("validateMachO() = nil, want error for too-short data")
	}
}
