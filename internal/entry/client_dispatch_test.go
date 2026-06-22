package entry

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"amuz.es/src/spi-ca/cvmm/internal/hvm"
	"amuz.es/src/spi-ca/cvmm/internal/model"
	"amuz.es/src/spi-ca/cvmm/internal/util"
	"github.com/spf13/viper"
)

type clientRecordedRequest struct {
	Method string
	Path   string
	Body   string
}

func TestClientHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_ENTRY_CLIENT_HELPER") != "1" {
		return
	}

	setupClientViperFromEnv()
	Client("cvmm", os.Getenv("TEST_CLIENT_NODE_NAME"), hvm.ClientAction(os.Getenv("TEST_CLIENT_ACTION")))
}

func TestClientDispatchVmCreateReadsYAMLAndCallsUnixSocket(t *testing.T) {
	discardClientLogs(t)
	rt := setupClientRuntime(t)
	_, records := withClientUnixHTTPServer(t, rt.socketPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	stdinPath := filepath.Join(t.TempDir(), "stdin.yaml")
	if err := os.WriteFile(stdinPath, []byte("payload:\n  kernel: /custom/kernel\nmemory:\n  size: 1024\n  thp: false\n"), 0o644); err != nil {
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

	Client("cvmm", rt.nodeName, hvm.ClientActionVmCreate)

	got := <-records
	if got.Method != http.MethodPut || got.Path != "/api/v1/vm.create" {
		t.Fatalf("request = %s %s, want PUT /api/v1/vm.create", got.Method, got.Path)
	}
	if !strings.Contains(got.Body, `"kernel":"/custom/kernel"`) {
		t.Fatalf("request body = %s, want encoded payload kernel", got.Body)
	}
	if !strings.Contains(got.Body, `"thp":false`) {
		t.Fatalf("request body = %s, want explicit thp:false", got.Body)
	}
}

func TestClientDispatchVmInfoEncodesYAMLResponse(t *testing.T) {
	discardClientLogs(t)
	rt := setupClientRuntime(t)
	_, _ = withClientUnixHTTPServer(t, rt.socketPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&model.VmInfo{
			State: model.NodeStatusRunning,
			Config: model.VmConfig{
				Console: &model.ConsoleConfig{File: "/tmp/console", Mode: model.ConsoleModeFile},
			},
		})
	})

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdoutReader.Close()
	previousStdout := os.Stdout
	os.Stdout = stdoutWriter
	defer func() { os.Stdout = previousStdout }()

	Client("cvmm", rt.nodeName, hvm.ClientActionVmInfo)
	_ = stdoutWriter.Close()

	output, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatalf("ReadAll(stdout) error = %v", err)
	}
	if !strings.Contains(string(output), "state: Running") {
		t.Fatalf("stdout = %s, want YAML state", string(output))
	}
	if !strings.Contains(string(output), "file: /tmp/console") {
		t.Fatalf("stdout = %s, want YAML console file", string(output))
	}
}

func TestClientDispatchMalformedStdinStopsBeforeAPICall(t *testing.T) {
	rt := setupClientRuntime(t)
	_, records := withClientUnixHTTPServer(t, rt.socketPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	stdinPath := filepath.Join(t.TempDir(), "stdin.yaml")
	if err := os.WriteFile(stdinPath, []byte("payload: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestClientHelperProcess")
	cmd.Env = append(os.Environ(),
		"GO_WANT_ENTRY_CLIENT_HELPER=1",
		"TEST_CLIENT_ACTION="+hvm.ClientActionVmCreate.String(),
		"TEST_CLIENT_NODE_NAME="+rt.nodeName,
		"TEST_CLIENT_IMAGE_ROOT="+rt.imageRoot,
		"TEST_CLIENT_NODE_ROOT="+rt.nodeRoot,
		"TEST_CLIENT_VOLATILE_DIR=run",
		"TEST_CLIENT_MANIFEST_FILENAME=config.yaml",
		"TEST_CLIENT_KERNEL_FILENAME=vmlinuz",
		"TEST_CLIENT_INITRAMFS_FILENAME=initramfs.img",
		"TEST_CLIENT_ROOTFS_FILENAME=root.img",
		"TEST_CLIENT_PID_FILENAME=cvmm.pid",
		"TEST_CLIENT_CLOUD_PID_FILENAME=cloudhypervisor.pid",
		"TEST_CLIENT_API_FILENAME=api.sock",
		"TEST_CLIENT_VIRTIOFS_TEMPLATE=virtiofs.sock",
		"TEST_CLIENT_CLOUD_PATH=sh",
		"TEST_CLIENT_VIRTIOFSD_PATH=sh",
		"TEST_CLIENT_RUNAS=",
	)
	stdin, err := os.Open(stdinPath)
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	cmd.Stdin = stdin
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	err = cmd.Run()
	if err == nil {
		t.Fatal("helper client run error = nil, want decode failure exit")
	}

	select {
	case got := <-records:
		t.Fatalf("unexpected API request before decode failure: %#v", got)
	case <-time.After(200 * time.Millisecond):
	}
}

func withClientUnixHTTPServer(t *testing.T, socketPath string, handler http.HandlerFunc) (string, <-chan clientRecordedRequest) {
	t.Helper()

	ensureClientTestRuntimeDir(t, socketPath)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}

	recordCh := make(chan clientRecordedRequest, 8)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		recordCh <- clientRecordedRequest{Method: r.Method, Path: r.URL.Path, Body: string(body)}
		handler(w, r)
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		_ = os.Remove(socketPath)
		close(recordCh)
	})
	return socketPath, recordCh
}

type clientRuntime struct {
	nodeName   string
	imageRoot  string
	nodeRoot   string
	socketPath string
}

func setupClientRuntime(t *testing.T) *clientRuntime {
	t.Helper()

	tmp := t.TempDir()
	rt := &clientRuntime{
		nodeName:   "n",
		imageRoot:  filepath.Join(tmp, "i"),
		nodeRoot:   filepath.Join(tmp, "n"),
		socketPath: filepath.Join(tmp, "n", "n", "run", "api.sock"),
	}
	nodeBasePath := filepath.Join(rt.nodeRoot, rt.nodeName)
	imageBasePath := filepath.Join(rt.imageRoot, "test-image")
	writeClientTestFile(t, filepath.Join(nodeBasePath, "config.yaml"), []byte(`cpus: 1
mem: 1G
uuid: 87773d86-0030-4db4-9e90-e5a4314ff11b
image: test-image
`))
	writeClientTestFile(t, filepath.Join(imageBasePath, "vmlinuz"), nil)
	writeClientTestFile(t, filepath.Join(imageBasePath, "root.img"), nil)
	setupClientViper(rt.imageRoot, rt.nodeRoot)
	return rt
}

func setupClientViper(imageRoot, nodeRoot string) {
	viper.Set("image.root", imageRoot)
	viper.Set("node.root", nodeRoot)
	viper.Set("volatile.directory", "run")
	viper.Set("manifest.filename", "config.yaml")
	viper.Set("image.kernel.filename", "vmlinuz")
	viper.Set("image.initramfs.filename", "initramfs.img")
	viper.Set("image.rootfs.filename", "root.img")
	viper.Set("pid.filename", "cvmm.pid")
	viper.Set("cloudhypervisor.pid.filename", "cloudhypervisor.pid")
	viper.Set("cloudhypervisor.api.filename", "api.sock")
	viper.Set("virtiofs.socket.filename.template", "virtiofs.sock")
	viper.Set("cloudhypervisor.path", "sh")
	viper.Set("virtiofsd.path", "sh")
	viper.Set("console", false)
	viper.Set("runas", "")
}

func setupClientViperFromEnv() {
	setupClientViper(os.Getenv("TEST_CLIENT_IMAGE_ROOT"), os.Getenv("TEST_CLIENT_NODE_ROOT"))
	viper.Set("volatile.directory", os.Getenv("TEST_CLIENT_VOLATILE_DIR"))
	viper.Set("manifest.filename", os.Getenv("TEST_CLIENT_MANIFEST_FILENAME"))
	viper.Set("image.kernel.filename", os.Getenv("TEST_CLIENT_KERNEL_FILENAME"))
	viper.Set("image.initramfs.filename", os.Getenv("TEST_CLIENT_INITRAMFS_FILENAME"))
	viper.Set("image.rootfs.filename", os.Getenv("TEST_CLIENT_ROOTFS_FILENAME"))
	viper.Set("pid.filename", os.Getenv("TEST_CLIENT_PID_FILENAME"))
	viper.Set("cloudhypervisor.pid.filename", os.Getenv("TEST_CLIENT_CLOUD_PID_FILENAME"))
	viper.Set("cloudhypervisor.api.filename", os.Getenv("TEST_CLIENT_API_FILENAME"))
	viper.Set("virtiofs.socket.filename.template", os.Getenv("TEST_CLIENT_VIRTIOFS_TEMPLATE"))
	viper.Set("cloudhypervisor.path", os.Getenv("TEST_CLIENT_CLOUD_PATH"))
	viper.Set("virtiofsd.path", os.Getenv("TEST_CLIENT_VIRTIOFSD_PATH"))
	viper.Set("runas", os.Getenv("TEST_CLIENT_RUNAS"))
}

func discardClientLogs(t *testing.T) {
	t.Helper()
	util.InfoLog.SetOutput(io.Discard)
	util.ErrLog.SetOutput(io.Discard)
	t.Cleanup(func() {
		util.InfoLog.SetOutput(os.Stdout)
		util.ErrLog.SetOutput(os.Stderr)
	})
}

func ensureClientTestDir(t *testing.T, dir string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(dir, mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, mode); err != nil {
		t.Fatal(err)
	}
}

func ensureClientTestRuntimeDir(t *testing.T, path string) {
	t.Helper()
	runtimeDir := filepath.Dir(path)
	ensureClientTestDir(t, filepath.Dir(runtimeDir), 0o755)
	ensureClientTestDir(t, runtimeDir, 0o700)
}

func writeClientTestFile(t *testing.T, path string, content []byte) {
	t.Helper()

	dir := filepath.Dir(path)
	if filepath.Base(dir) == "run" {
		ensureClientTestRuntimeDir(t, path)
	} else {
		ensureClientTestDir(t, dir, 0o755)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}
