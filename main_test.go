package main

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"amuz.es/src/spi-ca/cvmm/internal/hvm"
	"amuz.es/src/spi-ca/cvmm/internal/model"
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

func TestMainCLIEnvOverrideAndBinding(t *testing.T) {
	binaryPath := buildTestBinary(t)
	rt := setupMainClientRuntime(t, "env-images", "env-nodes", "volatile-env", "manifest-env.yaml", "api-env.sock")
	_, _ = withMainUnixHTTPServer(t, rt.socketPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(model.VmmPingResponse{Version: "1.0", PID: 42})
	})

	cmd := exec.Command(binaryPath, "client", hvm.ClientActionVmmPing.String(), rt.nodeName)
	cmd.Env = append(os.Environ(),
		"IMAGE_ROOT="+rt.imageRoot,
		"NODE_ROOT="+rt.nodeRoot,
		"VOLATILE_DIRECTORY="+rt.volatileDirectory,
		"MANIFEST_FILENAME="+rt.manifestFilename,
		"CLOUDHYPERVISOR_API_FILENAME="+rt.apiFilename,
		"IMAGE_KERNEL_FILENAME=vmlinuz",
		"IMAGE_INITRAMFS_FILENAME=initramfs.img",
		"IMAGE_ROOTFS_FILENAME=root.img",
		"CLOUDHYPERVISOR_PATH=sh",
		"VIRTIOFSD_PATH=sh",
		"VIRTIOFS_SOCKET_FILENAME_TEMPLATE=virtiofs-env.sock",
		"VIRTIOFS_PID_FILENAME_TEMPLATE=virtiofs-env.pid",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command error = %v\n%s", err, output)
	}
	text := string(output)
	for _, want := range []string{
		"image.root=" + rt.imageRoot,
		"node.root=" + rt.nodeRoot,
		"cloudhypervisor.api.filename=" + rt.apiFilename,
		"virtiofs.socket.filename.template=virtiofs-env.sock",
		"virtiofs.pid.filename.template=virtiofs-env.pid",
		"cloudhypervisor.path=sh",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output = %s, want %q", text, want)
		}
	}
}

func TestMainCLIFlagsOverrideEnv(t *testing.T) {
	binaryPath := buildTestBinary(t)
	rt := setupMainClientRuntime(t, "flag-images", "flag-nodes", "run", "config.yaml", "cloudhypervisor.sock")
	_, _ = withMainUnixHTTPServer(t, rt.socketPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(model.VmmPingResponse{Version: "1.0", PID: 7})
	})

	cmd := exec.Command(binaryPath,
		"--image-root", rt.imageRoot,
		"--node-root", rt.nodeRoot,
		"client", hvm.ClientActionVmmPing.String(), rt.nodeName,
	)
	cmd.Env = append(os.Environ(),
		"IMAGE_ROOT="+filepath.Join(t.TempDir(), "wrong-images"),
		"NODE_ROOT="+filepath.Join(t.TempDir(), "wrong-nodes"),
		"CLOUDHYPERVISOR_PATH=sh",
		"VIRTIOFSD_PATH=sh",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command error = %v\n%s", err, output)
	}
	text := string(output)
	for _, want := range []string{
		"image.root=" + rt.imageRoot,
		"node.root=" + rt.nodeRoot,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output = %s, want %q", text, want)
		}
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

type mainRecordedRequest struct {
	Method string
	Path   string
	Body   string
}

type mainClientRuntime struct {
	nodeName          string
	imageRoot         string
	nodeRoot          string
	volatileDirectory string
	manifestFilename  string
	apiFilename       string
	socketPath        string
}

func setupMainClientRuntime(t *testing.T, imageRootName, nodeRootName, volatileDirectory, manifestFilename, apiFilename string) *mainClientRuntime {
	t.Helper()

	tmp := t.TempDir()
	rt := &mainClientRuntime{
		nodeName:          "main-client-node",
		imageRoot:         filepath.Join(tmp, imageRootName),
		nodeRoot:          filepath.Join(tmp, nodeRootName),
		volatileDirectory: volatileDirectory,
		manifestFilename:  manifestFilename,
		apiFilename:       apiFilename,
	}
	rt.socketPath = filepath.Join(rt.nodeRoot, rt.nodeName, rt.volatileDirectory, rt.apiFilename)

	nodeBasePath := filepath.Join(rt.nodeRoot, rt.nodeName)
	imageBasePath := filepath.Join(rt.imageRoot, "test-image")
	writeMainTestFile(t, filepath.Join(nodeBasePath, rt.manifestFilename), []byte(`cpus: 1
mem: 1G
uuid: 87773d86-0030-4db4-9e90-e5a4314ff11b
image: test-image
`))
	writeMainTestFile(t, filepath.Join(imageBasePath, "vmlinuz"), nil)
	writeMainTestFile(t, filepath.Join(imageBasePath, "root.img"), nil)
	return rt
}

func withMainUnixHTTPServer(t *testing.T, socketPath string, handler http.HandlerFunc) (string, <-chan mainRecordedRequest) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(socketPath), 0o755); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}

	recordCh := make(chan mainRecordedRequest, 8)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		recordCh <- mainRecordedRequest{Method: r.Method, Path: r.URL.Path, Body: string(body)}
		handler(w, r)
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Close()
		_ = os.Remove(socketPath)
		close(recordCh)
	})
	return socketPath, recordCh
}

func writeMainTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}
