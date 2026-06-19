package hvm

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"amuz.es/src/spi-ca/cvmm/internal/model"
	"amuz.es/src/spi-ca/cvmm/internal/util"
	"amuz.es/src/spi-ca/cvmm/internal/util/sys"
)

// Load resolves node/image/runtime paths, loads the node manifest, prepares runas credentials, and assembles Hypervisor runtime configuration.
func Load(
	name,
	imageRoot, nodeRoot, volatileDirectory,
	manifestFilename,
	kernelFilename, initramfsFilename, rootfsFilename,
	pidFilename, apiPidFilename, apiSocketFilename,
	virtiofsdSocketFilenameTemplate,
	cloudhypervisorBinaryPath, virtiofsdBinaryPath string,
	consoleRedirectToStd bool,
	runAsUser string,
) (*Hypervisor, error) {
	nodeBasePath := filepath.Join(nodeRoot, name)
	volatileBasePath := filepath.Join(nodeBasePath, volatileDirectory)

	pidPath := filepath.Join(volatileBasePath, pidFilename)
	apiPidPath := filepath.Join(volatileBasePath, apiPidFilename)
	apiSocketPath := filepath.Join(volatileBasePath, apiSocketFilename)

	virtiofsdSocketPathTemplate := filepath.Join(volatileBasePath, virtiofsdSocketFilenameTemplate)

	var (
		runAs     *syscall.Credential
		groupName = ""

		err error
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
		pidPath:                   pidPath,
		cloudhypervisorBinaryPath: cloudhypervisorBinaryPath,
		cloudhypervisorPidPath:    apiPidPath,
		virtiofsdBinaryPath:       virtiofsdBinaryPath,
		runAs:                     runAs,
	}

	h.cli = newClient(apiSocketPath)

	manifestFilePath := filepath.Join(nodeBasePath, manifestFilename)

	cfg, err := model.LoadConfig(manifestFilePath)
	if err != nil {
		return nil, err
	}

	imageBasePath := filepath.Join(imageRoot, cfg.Image)
	kernelPath := filepath.Join(imageBasePath, kernelFilename)
	initramfsPath := filepath.Join(imageBasePath, initramfsFilename)
	rootfsPath := filepath.Join(imageBasePath, rootfsFilename)

	if len(cfg.NetMacAddr) == 0 {
		cfg.NetMacAddr = util.GenerateKvmMACAddress()
	}

	if len(cfg.NetIfName) == 0 {
		cfg.NetIfName = cfg.NetMacAddr.GenerateIfName("vmtap-")
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

	util.InfoLog.Printf("network interface(%s): %s", cfg.NetIfName, cfg.NetMacAddr)

	h.vmcfg = cfg.VMConfig(
		h.name,
		kernelPath, initramfsPath, rootfsPath,
		nodeBasePath, virtiofsdSocketPathTemplate,
		consoleRedirectToStd,
	)
	util.InfoLog.Printf("hypervisor config: %s", h.vmcfg)

	h.virtiofsdcfg = cfg.VirtiofsConfig(nodeBasePath, virtiofsdSocketPathTemplate, groupName)
	util.InfoLog.Printf("virtiofs config: %s", h.virtiofsdcfg)

	return h, nil
}
