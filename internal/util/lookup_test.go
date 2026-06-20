package util

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLookupBinaryHandlesEmptyMissingAndRelativePaths(t *testing.T) {
	oldErrOut := ErrLog.Writer()
	ErrLog.SetOutput(io.Discard)
	defer ErrLog.SetOutput(oldErrOut)

	if got := LookupBinary(""); got != "" {
		t.Fatalf("LookupBinary(empty) = %q, want empty", got)
	}
	if got := LookupBinary("cvmm-test-binary-that-does-not-exist"); got != "" {
		t.Fatalf("LookupBinary(missing) = %q, want empty", got)
	}

	tmp := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(cwd) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	relativePath := "./helper-bin"
	if err := os.WriteFile(relativePath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := LookupBinary(relativePath)
	want := filepath.Join(tmp, "helper-bin")
	if got != want {
		t.Fatalf("LookupBinary(%q) = %q, want %q", relativePath, got, want)
	}
}
