package hvm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"amuz.es/src/spi-ca/cvmm/internal/model"
	"golang.org/x/sys/unix"
)

const linuxTHPSysfsRoot = "/sys/kernel/mm/transparent_hugepage"

var hostTHPProbe = func(shared bool) thpDecision {
	return probeHostTHP(shared, linuxTHPSysfsRoot, func() (int, error) {
		return unix.PrctlRetInt(unix.PR_GET_THP_DISABLE, 0, 0, 0, 0)
	})
}

type thpDecision struct {
	enabled  bool
	reason   string
	warnings []string
}

func (d thpDecision) state() string {
	if d.enabled {
		return "enabled"
	}
	return "disabled"
}

func applyTHPDecision(cfg model.VmConfig, enabled bool) model.VmConfig {
	if cfg.Memory == nil {
		return cfg
	}
	memory := *cfg.Memory
	memory.Thp = boolPtr(enabled)
	cfg.Memory = &memory
	return cfg
}

func boolPtr(value bool) *bool { return &value }

func probeHostTHP(shared bool, sysfsRoot string, getTHPDisable func() (int, error)) thpDecision {
	decision := thpDecision{}

	if getTHPDisable != nil {
		value, err := getTHPDisable()
		switch {
		case err != nil:
			decision.warnings = append(decision.warnings, fmt.Sprintf("PR_GET_THP_DISABLE probe unavailable: %v", err))
		case value != 0:
			decision.reason = fmt.Sprintf("process THP disabled by PR_GET_THP_DISABLE=%d", value)
			return decision
		}
	}

	globalPolicy, err := selectedTHPPolicy(filepath.Join(sysfsRoot, "enabled"))
	if err != nil {
		decision.reason = "host THP probe failed"
		decision.warnings = append(decision.warnings, err.Error())
		return decision
	}
	if !globalTHPEnabled(globalPolicy) {
		decision.reason = fmt.Sprintf("host THP policy disables THP (enabled=%s)", globalPolicy)
		return decision
	}

	if !shared {
		decision.enabled = true
		decision.reason = fmt.Sprintf("host THP policy allows THP (enabled=%s)", globalPolicy)
		return decision
	}

	shmemPolicy, err := selectedTHPPolicy(filepath.Join(sysfsRoot, "shmem_enabled"))
	if err != nil {
		decision.reason = "shared-memory THP probe failed"
		decision.warnings = append(decision.warnings, err.Error())
		return decision
	}
	if !shmemTHPEnabled(shmemPolicy, globalPolicy) {
		decision.reason = fmt.Sprintf("shared-memory THP policy disables THP (enabled=%s shmem=%s)", globalPolicy, shmemPolicy)
		return decision
	}

	decision.enabled = true
	decision.reason = fmt.Sprintf("shared-memory THP policy allows THP (enabled=%s shmem=%s)", globalPolicy, shmemPolicy)
	return decision
}

func selectedTHPPolicy(path string) (string, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", path, err)
	}
	policy, err := parseSelectedTHPPolicy(string(buf))
	if err != nil {
		return "", fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return policy, nil
}

func parseSelectedTHPPolicy(content string) (string, error) {
	var selected string
	for _, field := range strings.Fields(content) {
		if strings.HasPrefix(field, "[") && strings.HasSuffix(field, "]") {
			if selected != "" {
				return "", fmt.Errorf("multiple selected policies in %q", strings.TrimSpace(content))
			}
			selected = strings.TrimSuffix(strings.TrimPrefix(field, "["), "]")
		}
	}
	if selected == "" {
		return "", fmt.Errorf("missing selected policy in %q", strings.TrimSpace(content))
	}
	return selected, nil
}

func globalTHPEnabled(policy string) bool {
	switch policy {
	case "always", "madvise":
		return true
	default:
		return false
	}
}

func shmemTHPEnabled(policy, globalPolicy string) bool {
	switch policy {
	case "always", "within_size", "advise", "force", "madvise":
		return true
	case "inherit":
		return globalTHPEnabled(globalPolicy)
	default:
		return false
	}
}

func isTHPRelatedVmCreateError(err error) bool {
	message := strings.ToLower(err.Error())
	for _, fragment := range []string{"thp", "transparent hugepage", "transparent huge page"} {
		if strings.Contains(message, fragment) {
			return true
		}
	}
	return false
}
