package hvm

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"amuz.es/src/spi-ca/cvmm/internal/model"
)

func TestOpenConsoleRejectsNonPTYPathFromVmInfo(t *testing.T) {
	nonPTYPath := filepath.Join(t.TempDir(), "not-a-pty")
	if err := os.WriteFile(nonPTYPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	socketPath, _ := withUnixHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&model.VmInfo{
			State: model.NodeStatusRunning,
			Config: model.VmConfig{
				Console: &model.ConsoleConfig{Mode: model.ConsoleModePty, File: nonPTYPath},
			},
		})
	})

	h := &Hypervisor{cli: newClient(socketPath)}
	defer h.Close()

	err := h.OpenConsole(context.Background())
	if err == nil {
		t.Fatal("OpenConsole() error = nil, want invalid PTY path rejection")
	}
	if !strings.Contains(err.Error(), "invalid console PTY path") {
		t.Fatalf("OpenConsole() error = %v, want invalid PTY path context", err)
	}
}
