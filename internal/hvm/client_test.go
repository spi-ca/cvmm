package hvm

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"amuz.es/src/spi-ca/cvmm/internal/model"
)

type recordedRequest struct {
	Method string
	Path   string
	Body   string
}

func withUnixHTTPServer(t *testing.T, handler http.HandlerFunc) (socketPath string, records <-chan recordedRequest) {
	t.Helper()

	socketPath = filepath.Join(t.TempDir(), "cloudhypervisor.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}

	recordCh := make(chan recordedRequest, 8)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		recordCh <- recordedRequest{Method: r.Method, Path: r.URL.Path, Body: string(body)}
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

func TestClientImplVmmPingUsesUnixSocket(t *testing.T) {
	socketPath, records := withUnixHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(model.VmmPingResponse{Version: "1.0", PID: 42, Features: []string{"test"}})
	})

	client := newClient(socketPath)
	defer client.Close()

	resp, err := client.VmmPing(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if resp.Version != "1.0" || resp.PID != 42 || len(resp.Features) != 1 || resp.Features[0] != "test" {
		t.Fatalf("VmmPing() = %#v", resp)
	}
	got := <-records
	if got.Method != http.MethodGet || got.Path != "/api/v1/vmm.ping" {
		t.Fatalf("request = %s %s, want GET /api/v1/vmm.ping", got.Method, got.Path)
	}
}

func TestClientImplVmCreateSendsJSONBody(t *testing.T) {
	socketPath, records := withUnixHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	client := newClient(socketPath)
	defer client.Close()

	thpDisabled := false
	cfg := model.VmConfig{
		Payload: model.PayloadConfig{Kernel: "/kernel"},
		Memory:  &model.MemoryConfig{Size: 1024, Thp: &thpDisabled},
	}
	if err := client.VmCreate(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	got := <-records
	if got.Method != http.MethodPut || got.Path != "/api/v1/vm.create" {
		t.Fatalf("request = %s %s, want PUT /api/v1/vm.create", got.Method, got.Path)
	}
	if !strings.Contains(got.Body, `"kernel":"/kernel"`) {
		t.Fatalf("request body = %s, want kernel JSON", got.Body)
	}
	if !strings.Contains(got.Body, `"thp":false`) {
		t.Fatalf("request body = %s, want explicit thp:false", got.Body)
	}
}

func TestClientImplVmInfoStatusErrors(t *testing.T) {
	socketPath, _ := withUnixHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not booted", http.StatusMethodNotAllowed)
	})

	client := newClient(socketPath)
	defer client.Close()

	_, err := client.VmInfo(context.Background())
	if err == nil {
		t.Fatal("VmInfo() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "VmInfo") || !strings.Contains(err.Error(), "not booted") {
		t.Fatalf("VmInfo() error = %v, want method and response details", err)
	}
}

func TestClientImplDialFailure(t *testing.T) {
	client := newClient(filepath.Join(t.TempDir(), "missing.sock"))
	defer client.Close()

	if _, err := client.VmmPing(context.Background()); err == nil {
		t.Fatal("VmmPing() error = nil for missing socket, want error")
	}
}
