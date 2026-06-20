package entry

import (
	"bytes"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"amuz.es/src/spi-ca/cvmm/internal/hvm"
)

func TestClientSignalCancelsInFlightRequest(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX signal semantics required")
	}

	rt := setupClientRuntime(t)
	requestStarted := make(chan struct{}, 1)
	_, _ = withClientUnixHTTPServer(t, rt.socketPath, func(w http.ResponseWriter, r *http.Request) {
		select {
		case requestStarted <- struct{}{}:
		default:
		}
		<-r.Context().Done()
	})

	cmd := exec.Command(os.Args[0], "-test.run=TestClientHelperProcess")
	cmd.Env = append(os.Environ(),
		"GO_WANT_ENTRY_CLIENT_HELPER=1",
		"TEST_CLIENT_ACTION="+hvm.ClientActionVmmPing.String(),
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
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("helper start error = %v", err)
	}

	select {
	case <-requestStarted:
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		t.Fatal("timed out waiting for client request to start")
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("Signal(SIGTERM) error = %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("helper wait error = nil, want canceled request exit")
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("timed out waiting for client helper to exit after signal")
	}

	text := stderr.String() + stdout.String()
	if !strings.Contains(text, syscall.SIGTERM.String()) || !strings.Contains(text, "received") {
		t.Fatalf("output = %s, want signal receipt log", text)
	}
}
