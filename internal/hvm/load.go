package hvm

import "path/filepath"

func Load(name, imageRoot, nodeRoot, volatileDirectory, manifestFilename, socketFilename string) (*Hypervisor, error) {
	h := &Hypervisor{
		name:              name,
		imageRoot:         imageRoot,
		nodeHome:          filepath.Join(nodeRoot, name),
		volatileDirectory: volatileDirectory,
	}

	h.cli = newClient(h.NodeBasePath(socketFilename))

	if err := h.load(manifestFilename); err != nil {
		return nil, err
	} else {
		return h, nil
	}
}
