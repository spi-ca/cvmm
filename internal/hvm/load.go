package hvm

import (
	"fmt"
	"path/filepath"

	"amuz.es/src/spi-ca/cvmm/internal/model"
	"amuz.es/src/spi-ca/cvmm/internal/util"
)

func Load(
	name,
	imageRoot, nodeRoot, volatileDirectory,
	manifestFilename,
	kernelFilename, initramfsFilename, rootfsFilename,
	pidFilename, apiPidFilename, apiSocketFilename,
	virtiofsdSocketFilenameTemplate,
	cloudhypervisorBinaryPath, virtiofsdBinaryPath string,
	consoleRedirectToStd bool,
) (*Hypervisor, error) {

	nodeBasePath := filepath.Join(nodeRoot, name)
	volatileBasePath := filepath.Join(nodeBasePath, volatileDirectory)

	pidPath := filepath.Join(volatileBasePath, pidFilename)
	apiPidPath := filepath.Join(volatileBasePath, apiPidFilename)
	apiSocketPath := filepath.Join(volatileBasePath, apiSocketFilename)

	virtiofsdSocketPathTemplate := filepath.Join(volatileBasePath, virtiofsdSocketFilenameTemplate)

	h := &Hypervisor{
		name:                      name,
		pidPath:                   pidPath,
		cloudhypervisorBinaryPath: cloudhypervisorBinaryPath,
		cloudhypervisorPidPath:    apiPidPath,
		virtiofsdBinaryPath:       virtiofsdBinaryPath,
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

	if len(cfg.NetIfName) == 0 {
		cfg.NetIfName = fmt.Sprintf("vmtap-%s", name)
	}

	if len(cfg.NetMacAddr) == 0 {
		kvmAddr := util.GenerateKvmMACAddress()
		cfg.NetMacAddr = kvmAddr
	}
	util.InfoLog.Printf("network interface(%s): %s", cfg.NetIfName, cfg.NetMacAddr)

	h.vmcfg = cfg.VMConfig(
		h.name,
		kernelPath, initramfsPath, rootfsPath,
		nodeBasePath, virtiofsdSocketPathTemplate,
		consoleRedirectToStd,
	)
	util.InfoLog.Printf("hypervisor config: %s", h.vmcfg)

	h.virtiofsdcfg = cfg.VirtiofsConfig(nodeBasePath, virtiofsdSocketPathTemplate)
	util.InfoLog.Printf("virtiofs config: %s", h.virtiofsdcfg)

	return h, nil
}
