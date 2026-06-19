package hvm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"amuz.es/src/spi-ca/cvmm/internal/model"
)

func TestClientImplStatusMappings(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		call     func(*clientImpl) error
		wantErr  error
		wantText string
	}{
		{
			name:    "VmBootNotCreated",
			status:  http.StatusNotFound,
			call:    func(c *clientImpl) error { return c.VmBoot(context.Background()) },
			wantErr: ErrVmNotCreated,
		},
		{
			name:    "VmPauseNotBooted",
			status:  http.StatusMethodNotAllowed,
			call:    func(c *clientImpl) error { return c.VmPause(context.Background()) },
			wantErr: ErrVmNotBooted,
		},
		{
			name:    "VmResumeNotPaused",
			status:  http.StatusMethodNotAllowed,
			call:    func(c *clientImpl) error { return c.VmResume(context.Background()) },
			wantErr: ErrVmNotPaused,
		},
		{
			name:    "VmShutdownNotStarted",
			status:  http.StatusMethodNotAllowed,
			call:    func(c *clientImpl) error { return c.VmShutdown(context.Background()) },
			wantErr: ErrVmNotStarted,
		},
		{
			name:    "VmPowerButtonNotBooted",
			status:  http.StatusMethodNotAllowed,
			call:    func(c *clientImpl) error { return c.VmPowerButton(context.Background()) },
			wantErr: ErrVmNotBooted,
		},
		{
			name:    "VmRestoreAlreadyCreated",
			status:  http.StatusNotFound,
			call:    func(c *clientImpl) error { return c.VmRestore(context.Background(), model.RestoreConfig{}) },
			wantErr: ErrVmAlreadyCreated,
		},
		{
			name:   "VmReceiveMigrationServerError",
			status: http.StatusInternalServerError,
			call: func(c *clientImpl) error {
				return c.VmReceiveMigration(context.Background(), model.ReceiveMigrationData{})
			},
			wantText: "migration could not be received",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			socketPath, _ := withUnixHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				if tc.body != "" {
					_, _ = w.Write([]byte(tc.body))
				}
			})

			client := newClient(socketPath)
			defer client.Close()

			err := tc.call(client)
			if err == nil {
				t.Fatal("call error = nil, want error")
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantText != "" && !strings.Contains(err.Error(), tc.wantText) {
				t.Fatalf("error = %v, want text %q", err, tc.wantText)
			}
		})
	}
}

func TestClientImplVmAddDeviceReturnsJSONAndNoContent(t *testing.T) {
	t.Run("JSON", func(t *testing.T) {
		socketPath, records := withUnixHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(model.PciDeviceInfo{ID: "device0", Bdf: "0000:00:05.0"})
		})

		client := newClient(socketPath)
		defer client.Close()

		resp, err := client.VmAddDevice(context.Background(), model.DeviceConfig{Path: "/dev/test0"})
		if err != nil {
			t.Fatalf("VmAddDevice() error = %v", err)
		}
		if resp == nil || resp.ID != "device0" || resp.Bdf != "0000:00:05.0" {
			t.Fatalf("VmAddDevice() = %#v, want decoded pci info", resp)
		}
		got := <-records
		if got.Path != "/api/v1/vm.add-device" || got.Method != http.MethodPut {
			t.Fatalf("request = %s %s, want PUT /api/v1/vm.add-device", got.Method, got.Path)
		}
		if !strings.Contains(got.Body, `"path":"/dev/test0"`) {
			t.Fatalf("request body = %s, want encoded device path", got.Body)
		}
	})

	t.Run("NoContent", func(t *testing.T) {
		socketPath, _ := withUnixHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})

		client := newClient(socketPath)
		defer client.Close()

		resp, err := client.VmAddDevice(context.Background(), model.DeviceConfig{Path: "/dev/test0"})
		if err != nil {
			t.Fatalf("VmAddDevice() error = %v", err)
		}
		if resp != nil {
			t.Fatalf("VmAddDevice() = %#v, want nil for 204 response", resp)
		}
	})
}

func TestClientImplCheckRedirectRejectsRedirects(t *testing.T) {
	client := newClient(filepath.Join(t.TempDir(), "missing.sock"))
	defer client.Close()

	if err := client.checkRedirect(&http.Request{}, nil); !errors.Is(err, ErrRedirectionForbidded) {
		t.Fatalf("checkRedirect() error = %v, want %v", err, ErrRedirectionForbidded)
	}
}
