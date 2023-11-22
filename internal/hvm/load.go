package hvm

import (
	"path/filepath"

	"amuz.es/src/spi-ca/chmgr/internal/model"
	"amuz.es/src/spi-ca/chmgr/internal/util"
)

func Load(
	name,
	imageRoot, nodeRoot, volatileDirectory,
	manifestFilename,
	kernelFilename, initramfsFilename, rootfsFilename,
	apiPidFilename, apiSocketFilename,
	virtiofsdSocketFilenameTemplate,
	cloudhypervisorBinaryPath, virtiofsdBinaryPath string) (*Hypervisor, error) {

	nodeBasePath := filepath.Join(nodeRoot, name)
	volatileBasePath := filepath.Join(nodeBasePath, volatileDirectory)

	apiPidPath := filepath.Join(volatileBasePath, apiPidFilename)
	apiSocketPath := filepath.Join(volatileBasePath, apiSocketFilename)

	virtiofsdSocketPathTemplate := filepath.Join(volatileBasePath, virtiofsdSocketFilenameTemplate)

	h := &Hypervisor{
		name:                      name,
		cloudhypervisorBinaryPath: cloudhypervisorBinaryPath,
		cloudhypervisorPidPath:    apiPidPath,
		virtiofsdBinaryPath:       virtiofsdBinaryPath,
	}

	h.cli = newClient(apiSocketPath)

	manifestFilePath := filepath.Join(nodeBasePath, manifestFilename)

	args, err := model.LoadConfig(manifestFilePath)
	if err != nil {
		return nil, err
	}

	imageBasePath := filepath.Join(imageRoot, args.Image)
	kernelPath := filepath.Join(imageBasePath, kernelFilename)
	initramfsPath := filepath.Join(imageBasePath, initramfsFilename)
	rootfsPath := filepath.Join(imageBasePath, rootfsFilename)

	h.vmcfg = args.VMConfig(
		h.name,
		kernelPath, initramfsPath, rootfsPath,
		nodeBasePath, virtiofsdSocketPathTemplate,
	)
	util.InfoLog.Printf("hypervisor config: %s", h.vmcfg)

	h.virtiofsdcfg = args.VirtiofsConfig(nodeBasePath, virtiofsdSocketPathTemplate)
	util.InfoLog.Printf("virtiofs config: %s", h.virtiofsdcfg)

	return h, nil
}
