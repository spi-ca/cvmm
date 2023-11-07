package hvm

import (
	"amuz.es/src/spi-ca/chmgr/internal/model"
	"amuz.es/src/spi-ca/chmgr/internal/util"
	"path/filepath"
)

func Load(
	name,
	imageRoot, nodeRoot, volatileDirectory,
	manifestFilename,
	kernelFilename, initramfsFilename, rootfsFilename,
	apiSocketFilename, virtiofsdSocketFilenameTemplate,
	cloudhypervisorBinaryPath, virtiofsdBinaryPath string) (*Hypervisor, error) {
	h := &Hypervisor{
		name:                      name,
		cloudhypervisorBinaryPath: cloudhypervisorBinaryPath,
		virtiofsdBinaryPath:       virtiofsdBinaryPath,
	}

	nodeBasePath := filepath.Join(nodeRoot, name)
	volatileBasePath := filepath.Join(nodeBasePath, volatileDirectory)
	virtiofsdSocketPathTemplate := filepath.Join(volatileBasePath, virtiofsdSocketFilenameTemplate)
	apiSocketPath := filepath.Join(volatileBasePath, apiSocketFilename)

	h.cli = newClient(apiSocketPath)

	manifestFilePath := filepath.Join(volatileBasePath, manifestFilename)

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
