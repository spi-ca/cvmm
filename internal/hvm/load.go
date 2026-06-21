package hvm

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"amuz.es/src/spi-ca/cvmm/internal/model"
	"amuz.es/src/spi-ca/cvmm/internal/util"
	"amuz.es/src/spi-ca/cvmm/internal/util/sys"
)

var safeNodeNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// Load resolves node/image/runtime paths, loads the node manifest, prepares runas credentials, and assembles Hypervisor runtime configuration.
func Load(
	name,
	imageRoot, nodeRoot, volatileDirectory,
	manifestFilename,
	kernelFilename, initramfsFilename, rootfsFilename,
	pidFilename, apiPidFilename, apiSocketFilename,
	virtiofsdSocketFilenameTemplate, virtiofsdPidFilenameTemplate,
	cloudhypervisorBinaryPath, virtiofsdBinaryPath, passtBinaryPath string,
	consoleRedirectToStd bool,
	runAsUser string,
) (*Hypervisor, error) {
	nodeBasePath, err := resolveNodeBasePath(nodeRoot, name)
	if err != nil {
		return nil, err
	}
	volatileBasePath := filepath.Join(nodeBasePath, volatileDirectory)

	pidPath := filepath.Join(volatileBasePath, pidFilename)
	apiPidPath := filepath.Join(volatileBasePath, apiPidFilename)
	apiSocketPath := filepath.Join(volatileBasePath, apiSocketFilename)

	virtiofsdSocketPathTemplate := filepath.Join(volatileBasePath, virtiofsdSocketFilenameTemplate)
	virtiofsdPidPathTemplate := filepath.Join(volatileBasePath, virtiofsdPidFilenameTemplate)
	passtSocketPath := filepath.Join(volatileBasePath, "passt.sock")
	passtPidPath := filepath.Join(volatileBasePath, "passt.pid")

	var (
		runAs     *syscall.Credential
		groupName = ""
	)
	if len(runAsUser) == 0 {
		// An empty runas keeps child process credentials inherited from the manager.
	} else if runAs, err = sys.LookupCredentials(runAsUser); err != nil {
		return nil, err
	} else if groupName, err = sys.LookupGroupName(runAs.Gid); err != nil {
		return nil, err
	}

	h := &Hypervisor{
		name:                      name,
		volatileBasePath:          volatileBasePath,
		pidPath:                   pidPath,
		cloudhypervisorBinaryPath: cloudhypervisorBinaryPath,
		cloudhypervisorPidPath:    apiPidPath,
		virtiofsdBinaryPath:       virtiofsdBinaryPath,
		passtBinaryPath:           passtBinaryPath,
		passtSocketPath:           passtSocketPath,
		passtPidPath:              passtPidPath,
		runAs:                     runAs,
		managerUID:                uint32(os.Geteuid()),
		managerGID:                uint32(os.Getegid()),
	}

	h.cli = newClient(apiSocketPath)

	manifestFilePath := filepath.Join(nodeBasePath, manifestFilename)

	cfg, err := model.LoadConfig(manifestFilePath)
	if err != nil {
		return nil, err
	}
	if err := cfg.ValidateDirectoryBasenames(); err != nil {
		return nil, err
	}
	h.netBackend = cfg.Net.Backend

	imageBasePath := filepath.Join(imageRoot, cfg.Image)
	kernelPath := filepath.Join(imageBasePath, kernelFilename)
	initramfsPath := filepath.Join(imageBasePath, initramfsFilename)
	rootfsPath := filepath.Join(imageBasePath, rootfsFilename)

	if len(cfg.Net.MacAddr) == 0 {
		cfg.Net.MacAddr = util.GenerateKvmMACAddress()
	}
	if cfg.UsesTapNetwork() && len(cfg.Net.IfName) == 0 {
		cfg.Net.IfName = cfg.Net.MacAddr.GenerateIfName("vmtap-")
	}
	if len(initramfsPath) == 0 {
		// An empty initramfs path means the VM will boot without initramfs.
	} else if stat, err := os.Stat(initramfsPath); errors.Is(err, os.ErrNotExist) {
		initramfsPath = ""
	} else if err != nil {
		return nil, fmt.Errorf("failed to stat initramfs %q: %w", initramfsPath, err)
	} else if stat.IsDir() {
		initramfsPath = ""
	}

	if cfg.UsesTapNetwork() {
		util.InfoLog.Printf("network backend(%s) interface(%s): %s", cfg.Net.Backend, cfg.Net.IfName, cfg.Net.MacAddr)
	} else {
		util.InfoLog.Printf("network backend(%s) socket(%s): %s", cfg.Net.Backend, passtSocketPath, cfg.Net.MacAddr)
	}

	h.vmcfg = cfg.VMConfig(
		h.name,
		kernelPath, initramfsPath, rootfsPath,
		nodeBasePath, virtiofsdSocketPathTemplate, passtSocketPath,
		consoleRedirectToStd,
	)
	util.InfoLog.Printf("hypervisor config summary(base, pre-thp decision): cpus=%d memory_size=%d disks=%d nets=%d fs=%d", h.vmcfg.Cpus.BootVcpus, h.vmcfg.Memory.Size, len(h.vmcfg.Disks), len(h.vmcfg.Net), len(h.vmcfg.Fs))

	h.virtiofsdcfg = cfg.VirtiofsConfig(nodeBasePath, virtiofsdSocketPathTemplate, virtiofsdPidPathTemplate, groupName)
	util.InfoLog.Printf("virtiofs config summary: shares=%d socket_group_set=%t", len(h.virtiofsdcfg), groupName != "")

	return h, nil
}

func resolveNodeBasePath(nodeRoot, name string) (string, error) {
	if err := validateNodeName(name); err != nil {
		return "", err
	}

	nodeBasePath := filepath.Join(nodeRoot, name)
	rel, err := filepath.Rel(nodeRoot, nodeBasePath)
	if err != nil {
		return "", fmt.Errorf("invalid node name %q: %w", name, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid node name %q", name)
	}
	return nodeBasePath, nil
}

func validateNodeName(name string) error {
	if len(name) == 0 || name == "." || name == ".." {
		return fmt.Errorf("invalid node name %q", name)
	}
	if strings.Contains(name, "..") || !safeNodeNamePattern.MatchString(name) {
		return fmt.Errorf("invalid node name %q", name)
	}
	return nil
}
