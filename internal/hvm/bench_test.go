package hvm

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"amuz.es/src/spi-ca/cvmm/internal/model"
	"amuz.es/src/spi-ca/cvmm/internal/util"
)

func BenchmarkClientVmmPingRoundTrip(b *testing.B) {
	socketPath := filepath.Join(b.TempDir(), "cloudhypervisor.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		b.Fatalf("Listen(unix) error = %v", err)
	}
	defer listener.Close()

	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(model.VmmPingResponse{Version: "1.0", PID: 42, Features: []string{"bench"}})
	})}
	defer func() {
		_ = server.Close()
		_ = os.Remove(socketPath)
	}()
	go func() { _ = server.Serve(listener) }()

	client := newClient(socketPath)
	defer client.Close()

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.VmmPing(ctx)
		if err != nil {
			b.Fatalf("VmmPing() error = %v", err)
		}
		if resp.Version != "1.0" {
			b.Fatalf("Version = %q, want 1.0", resp.Version)
		}
	}
}

func BenchmarkLoadPathAssembly(b *testing.B) {
	tmp := b.TempDir()
	imageRoot := filepath.Join(tmp, "images")
	nodeRoot := filepath.Join(tmp, "nodes")
	nodeName := "bench-node"
	nodeBasePath := filepath.Join(nodeRoot, nodeName)
	imageBasePath := filepath.Join(imageRoot, "bench-image")

	if err := os.MkdirAll(nodeBasePath, 0o755); err != nil {
		b.Fatal(err)
	}
	if err := os.MkdirAll(imageBasePath, 0o755); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nodeBasePath, "config.yaml"), []byte("cpus: 2\nmem: 2G\nuuid: 87773d86-0030-4db4-9e90-e5a4314ff11b\nimage: bench-image\ndirectory:\n  - share0\n  - share1\n"), 0o644); err != nil {
		b.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(imageBasePath, "vmlinuz"),
		filepath.Join(imageBasePath, "root.img"),
	} {
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			b.Fatal(err)
		}
	}

	oldInfoOut := util.InfoLog.Writer()
	oldErrOut := util.ErrLog.Writer()
	util.InfoLog.SetOutput(io.Discard)
	util.ErrLog.SetOutput(io.Discard)
	defer util.InfoLog.SetOutput(oldInfoOut)
	defer util.ErrLog.SetOutput(oldErrOut)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h, err := Load(
			nodeName,
			imageRoot, nodeRoot, "run",
			"config.yaml",
			"vmlinuz", "initramfs.img", "root.img",
			"cvmm.pid", "cloudhypervisor.pid", "api.sock",
			"virtiofs.sock",
			"virtiofs.pid",
			"/usr/bin/cloud-hypervisor", "/usr/bin/virtiofsd",
			false,
			"",
		)
		if err != nil {
			b.Fatalf("Load() error = %v", err)
		}
		h.Close()
	}
}

func BenchmarkInvokeExecutionResultFormatting(b *testing.B) {
	scriptPath := filepath.Join(b.TempDir(), "invoke-helper.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho stderr-line >&2\nexit 7\n"), 0o755); err != nil {
		b.Fatal(err)
	}

	h := &Hypervisor{}
	oldInfoOut := util.InfoLog.Writer()
	oldErrOut := util.ErrLog.Writer()
	util.InfoLog.SetOutput(io.Discard)
	util.ErrLog.SetOutput(io.Discard)
	defer util.InfoLog.SetOutput(oldInfoOut)
	defer util.ErrLog.SetOutput(oldErrOut)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := h.invoke(exec.Command(scriptPath), "", nil)
		if err == nil {
			b.Fatal("invoke() error = nil, want exit error")
		}
	}
}
