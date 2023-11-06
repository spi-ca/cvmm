package hvm

import (
	"path/filepath"
)

func Load(
	name,
	imageRoot, nodeRoot, volatileDirectory,
	manifestFilename, socketFilename,
	cloudhypervisorBinaryPath, virtiofsdBinaryPath string) (*Hypervisor, error) {
	h := &Hypervisor{
		name:                      name,
		imageRoot:                 imageRoot,
		nodeHome:                  filepath.Join(nodeRoot, name),
		volatileDirectory:         filepath.Join(nodeRoot, name, volatileDirectory),
		cloudhypervisorBinaryPath: cloudhypervisorBinaryPath,
		virtiofsd:                 virtiofsdJoiner{virtiofsdBinaryPath: virtiofsdBinaryPath},
	}

	h.cli = newClient(h.VolatilePath(socketFilename))
	manifestFilePath := h.NodeBasePath(manifestFilename)

	if err := h.load(manifestFilePath); err != nil {
		return nil, err
	} else {
		return h, nil
	}
}
