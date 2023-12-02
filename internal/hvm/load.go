package hvm

import (
	"fmt"
	"path/filepath"
	"strings"

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
	runAs string,
) (*Hypervisor, error) {
	nodeBasePath := filepath.Join(nodeRoot, name)
	volatileBasePath := filepath.Join(nodeBasePath, volatileDirectory)

	pidPath := filepath.Join(volatileBasePath, pidFilename)
	apiPidPath := filepath.Join(volatileBasePath, apiPidFilename)
	apiSocketPath := filepath.Join(volatileBasePath, apiSocketFilename)

	virtiofsdSocketPathTemplate := filepath.Join(volatileBasePath, virtiofsdSocketFilenameTemplate)

	runAsUser, runAsGroup := "", ""
	if len(runAs) == 0 {
		// do nothing
	} else if splitted := strings.SplitN(runAs, ":", 2); len(splitted) != 2 {
		return nil, fmt.Errorf("runAs \"%s\" is invalid format", runAs)
	} else {
		runAsUser, runAsGroup = splitted[0], splitted[1]
	}

	h := &Hypervisor{
		name:                      name,
		pidPath:                   pidPath,
		cloudhypervisorBinaryPath: cloudhypervisorBinaryPath,
		cloudhypervisorPidPath:    apiPidPath,
		virtiofsdBinaryPath:       virtiofsdBinaryPath,
		runAsUser:                 runAsUser,
		runAsGroup:                runAsGroup,
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

	util.InfoLog.Printf("network interface(%s): %s", cfg.NetIfName, cfg.NetMacAddr)

	h.vmcfg = cfg.VMConfig(
		h.name,
		kernelPath, initramfsPath, rootfsPath,
		nodeBasePath, virtiofsdSocketPathTemplate,
		consoleRedirectToStd,
	)
	util.InfoLog.Printf("hypervisor config: %s", h.vmcfg)

	h.virtiofsdcfg = cfg.VirtiofsConfig(nodeBasePath, virtiofsdSocketPathTemplate, runAsGroup)
	util.InfoLog.Printf("virtiofs config: %s", h.virtiofsdcfg)

	return h, nil
}
