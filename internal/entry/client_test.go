package entry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"amuz.es/src/spi-ca/cvmm/internal/hvm"
)

func TestDecodeYAMLRequestReturnsContextualError(t *testing.T) {
	stdinPath := filepath.Join(t.TempDir(), "stdin.yaml")
	if err := os.WriteFile(stdinPath, []byte("bad: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdin, err := os.Open(stdinPath)
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()

	previousStdin := os.Stdin
	os.Stdin = stdin
	defer func() { os.Stdin = previousStdin }()

	action := hvm.ClientActionVmResize
	var req map[string]any
	err = decodeYAMLRequest(action, &req)
	if err == nil {
		t.Fatal("decodeYAMLRequest() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "failed to decode request for "+action.String()) {
		t.Fatalf("decodeYAMLRequest() error = %v, want action context", err)
	}
}

func TestEncodeYAMLResponseReturnsContextualError(t *testing.T) {
	stdoutPath := filepath.Join(t.TempDir(), "stdout.yaml")
	if err := os.WriteFile(stdoutPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, err := os.Open(stdoutPath)
	if err != nil {
		t.Fatal(err)
	}
	defer stdout.Close()

	previousStdout := os.Stdout
	os.Stdout = stdout
	defer func() { os.Stdout = previousStdout }()

	action := hvm.ClientActionVmInfo
	err = encodeYAMLResponse(action, map[string]string{"status": "ok"})
	if err == nil {
		t.Fatal("encodeYAMLResponse() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "failed to encode response for "+action.String()) {
		t.Fatalf("encodeYAMLResponse() error = %v, want action context", err)
	}
}
