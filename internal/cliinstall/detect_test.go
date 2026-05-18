package cliinstall

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetect_FoundOnPATH(t *testing.T) {
	// Create a temp dir with a fake binary
	dir := t.TempDir()
	binary := filepath.Join(dir, "testcli")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\necho ok"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	path, found := Detect("testcli")
	if !found {
		t.Fatal("Detect() = false, want true for binary on PATH")
	}
	if path == "" {
		t.Fatal("Detect() returned empty path")
	}
}

func TestDetect_NotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, found := Detect("nonexistent-cli-tool-xyz")
	if found {
		t.Fatal("Detect() = true, want false for missing binary")
	}
}

func TestDetect_KnownCLIsNotEmpty(t *testing.T) {
	if len(KnownCLIs) == 0 {
		t.Fatal("KnownCLIs is empty")
	}
}

func TestDetect_AllCLIsHaveRequiredFields(t *testing.T) {
	for i, spec := range KnownCLIs {
		if spec.ProviderType == "" {
			t.Errorf("KnownCLIs[%d].ProviderType is empty", i)
		}
		if spec.Binary == "" {
			t.Errorf("KnownCLIs[%d].Binary is empty", i)
		}
		if spec.Owner == "" {
			t.Errorf("KnownCLIs[%d].Owner is empty", i)
		}
		if spec.Repo == "" {
			t.Errorf("KnownCLIs[%d].Repo is empty", i)
		}
		if spec.DefaultModel == "" {
			t.Errorf("KnownCLIs[%d].DefaultModel is empty", i)
		}
	}
}
