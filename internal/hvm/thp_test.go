package hvm

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"amuz.es/src/spi-ca/cvmm/internal/model"
)

func TestProbeHostTHP(t *testing.T) {
	tests := []struct {
		name        string
		enabled     string
		shmem       string
		prctlValue  int
		prctlErr    error
		omitShmem   bool
		wantEnabled bool
		wantReason  string
		wantWarning string
	}{
		{
			name:        "shared-memory supported",
			enabled:     "always [madvise] never\n",
			shmem:       "always within_size [advise] never deny force\n",
			wantEnabled: true,
			wantReason:  "allows THP",
		},
		{
			name:        "global policy disabled",
			enabled:     "always madvise [never]\n",
			shmem:       "always within_size [advise] never deny force\n",
			wantEnabled: false,
			wantReason:  "host THP policy disables THP",
		},
		{
			name:        "shared-memory policy disabled",
			enabled:     "always [madvise] never\n",
			shmem:       "always within_size advise [never] deny force\n",
			wantEnabled: false,
			wantReason:  "shared-memory THP policy disables THP",
		},
		{
			name:        "missing shmem probe disables THP",
			enabled:     "always [madvise] never\n",
			omitShmem:   true,
			wantEnabled: false,
			wantReason:  "shared-memory THP probe failed",
			wantWarning: "shmem_enabled",
		},
		{
			name:        "malformed shmem probe disables THP",
			enabled:     "always [madvise] never\n",
			shmem:       "always never\n",
			wantEnabled: false,
			wantReason:  "shared-memory THP probe failed",
			wantWarning: "failed to parse",
		},
		{
			name:        "process THP disabled via prctl",
			enabled:     "always [madvise] never\n",
			shmem:       "always within_size [advise] never deny force\n",
			prctlValue:  1,
			wantEnabled: false,
			wantReason:  "PR_GET_THP_DISABLE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "enabled"), []byte(tt.enabled), 0o644); err != nil {
				t.Fatal(err)
			}
			if !tt.omitShmem {
				if err := os.WriteFile(filepath.Join(root, "shmem_enabled"), []byte(tt.shmem), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			decision := probeHostTHP(true, root, func() (int, error) {
				return tt.prctlValue, tt.prctlErr
			})
			if decision.enabled != tt.wantEnabled {
				t.Fatalf("probeHostTHP().enabled = %v, want %v", decision.enabled, tt.wantEnabled)
			}
			if !strings.Contains(decision.reason, tt.wantReason) {
				t.Fatalf("probeHostTHP().reason = %q, want substring %q", decision.reason, tt.wantReason)
			}
			if tt.wantWarning != "" {
				joined := strings.Join(decision.warnings, "\n")
				if !strings.Contains(joined, tt.wantWarning) {
					t.Fatalf("probeHostTHP().warnings = %q, want substring %q", joined, tt.wantWarning)
				}
			}
		})
	}
}

func TestStartUsesTHPDecisionForVmCreatePayload(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific process identity and ambient-capability behavior")
	}

	tests := []struct {
		name        string
		decision    thpDecision
		wantPayload string
	}{
		{name: "enabled", decision: thpDecision{enabled: true, reason: "test"}, wantPayload: `"thp":true`},
		{name: "disabled", decision: thpDecision{enabled: false, reason: "test"}, wantPayload: `"thp":false`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldProbe := hostTHPProbe
			hostTHPProbe = func(shared bool) thpDecision { return tt.decision }
			defer func() { hostTHPProbe = oldProbe }()

			tmp := t.TempDir()
			helperDir := makeTestHelperDir(t, "cloud-hypervisor-helper-*")
			helperPath := buildCloudHypervisorStubBinary(t, helperDir)
			ensureCloudHypervisorHelperCanStart(t, helperPath)

			createBodyPath := filepath.Join(tmp, "create-body.json")
			t.Setenv("TEST_CH_CREATE_BODY_FILE", createBodyPath)
			t.Setenv("TEST_CH_READY_FILE", filepath.Join(tmp, "ready"))
			t.Setenv("TEST_CH_BOOT_FILE", filepath.Join(tmp, "boot"))
			t.Setenv("TEST_CH_POWER_FILE", filepath.Join(tmp, "power"))
			t.Setenv("TEST_CH_TERM_FILE", filepath.Join(tmp, "term"))
			t.Setenv("TEST_CH_EXIT_FILE", filepath.Join(tmp, "exit"))

			h := &Hypervisor{
				name:                      "thp-payload-node",
				pidPath:                   filepath.Join(tmp, "cvmm.pid"),
				cloudhypervisorBinaryPath: helperPath,
				cloudhypervisorPidPath:    filepath.Join(tmp, "cloudhypervisor.pid"),
				vmcfg:                     modelConfigWithSharedMemory(),
				cli:                       newClient(filepath.Join(tmp, "api.sock")),
			}
			defer h.Close()

			ctx, cancel := context.WithCancel(context.Background())
			errCh := make(chan error, 1)
			go func() { errCh <- h.Start(ctx) }()

			waitForFile(t, filepath.Join(tmp, "boot"), 5*time.Second)
			cancel()

			select {
			case err := <-errCh:
				if err != nil {
					t.Fatalf("Start() error = %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("timed out waiting for Start() to return")
			}

			body, err := os.ReadFile(createBodyPath)
			if err != nil {
				t.Fatalf("ReadFile(create body) error = %v", err)
			}
			bodyText := string(body)
			if !strings.Contains(bodyText, tt.wantPayload) {
				t.Fatalf("vm.create body = %s, want substring %s", bodyText, tt.wantPayload)
			}
			if strings.Contains(bodyText, `"mergeable":true`) {
				t.Fatalf("vm.create body = %s, want default shared memory without mergeable:true", bodyText)
			}
		})
	}
}

func TestStartRetriesVmCreateWithTHPDisabledAfterTHPError(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific process identity and ambient-capability behavior")
	}

	oldProbe := hostTHPProbe
	hostTHPProbe = func(shared bool) thpDecision {
		return thpDecision{enabled: true, reason: "test enabled"}
	}
	defer func() { hostTHPProbe = oldProbe }()

	tmp := t.TempDir()
	helperDir := makeTestHelperDir(t, "cloud-hypervisor-helper-*")
	helperPath := buildCloudHypervisorStubBinary(t, helperDir)
	ensureCloudHypervisorHelperCanStart(t, helperPath)

	createBodiesPath := filepath.Join(tmp, "create-bodies.jsonl")
	t.Setenv("TEST_CH_CREATE_BODY_FILE", createBodiesPath)
	t.Setenv("TEST_CH_CREATE_STATUS_SEQUENCE", "500|204")
	t.Setenv("TEST_CH_CREATE_BODY_SEQUENCE", "THP unsupported|")
	t.Setenv("TEST_CH_READY_FILE", filepath.Join(tmp, "ready"))
	t.Setenv("TEST_CH_BOOT_FILE", filepath.Join(tmp, "boot"))
	t.Setenv("TEST_CH_POWER_FILE", filepath.Join(tmp, "power"))
	t.Setenv("TEST_CH_TERM_FILE", filepath.Join(tmp, "term"))
	t.Setenv("TEST_CH_EXIT_FILE", filepath.Join(tmp, "exit"))

	h := &Hypervisor{
		name:                      "thp-retry-node",
		pidPath:                   filepath.Join(tmp, "cvmm.pid"),
		cloudhypervisorBinaryPath: helperPath,
		cloudhypervisorPidPath:    filepath.Join(tmp, "cloudhypervisor.pid"),
		vmcfg:                     modelConfigWithSharedMemory(),
		cli:                       newClient(filepath.Join(tmp, "api.sock")),
	}
	defer h.Close()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- h.Start(ctx) }()

	waitForFile(t, filepath.Join(tmp, "boot"), 5*time.Second)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Start() to return")
	}

	bodies, err := os.ReadFile(createBodiesPath)
	if err != nil {
		t.Fatalf("ReadFile(create bodies) error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(bodies)), "\n")
	if len(lines) != 2 {
		t.Fatalf("vm.create body count = %d, want 2 (%q)", len(lines), string(bodies))
	}
	if !strings.Contains(lines[0], `"thp":true`) {
		t.Fatalf("first vm.create body = %s, want thp:true", lines[0])
	}
	if !strings.Contains(lines[1], `"thp":false`) {
		t.Fatalf("second vm.create body = %s, want thp:false", lines[1])
	}
}

func modelConfigWithSharedMemory() model.VmConfig {
	return model.VmConfig{Memory: &model.MemoryConfig{Size: 1024, Shared: true}}
}
