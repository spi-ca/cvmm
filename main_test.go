package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMainCLIUsageAndValidation(t *testing.T) {
	binaryPath := buildTestBinary(t)

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "NoArgs",
			args:    nil,
			wantErr: "not enough arguments",
		},
		{
			name:    "InvalidAction",
			args:    []string{"invalid-action"},
			wantErr: "invalid action invalid-action",
		},
		{
			name:    "InvalidConsoleFile",
			args:    []string{"console-file", "abc"},
			wantErr: "invalid pty_id abc",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, tc.args...)
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatal("command error = nil, want non-zero exit")
			}
			if !strings.Contains(string(output), tc.wantErr) {
				t.Fatalf("output = %s, want %q", string(output), tc.wantErr)
			}
			if !strings.Contains(string(output), "usage:") {
				t.Fatalf("output = %s, want usage text", string(output))
			}
		})
	}
}

func buildTestBinary(t *testing.T) string {
	t.Helper()

	binaryPath := filepath.Join(t.TempDir(), "cvmm-test")
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Env = os.Environ()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build error = %v\n%s", err, out)
	}
	return binaryPath
}
