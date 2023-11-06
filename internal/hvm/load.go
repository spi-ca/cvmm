package hvm

import (
	"amuz.es/src/spi-ca/chmgr/internal/util"
	"path/filepath"
)

func Load(name, imageRoot, nodeRoot, volatileDirectory, manifestFilename, socketFilename, virtiofsFilenameTmpl string) (*Hypervisor, error) {
	h := &Hypervisor{
		name:                   name,
		imageRoot:              imageRoot,
		nodeHome:               filepath.Join(nodeRoot, name),
		volatileDirectory:      filepath.Join(nodeRoot, name, volatileDirectory),
		virtiofsSocketTemplate: util.F(virtiofsFilenameTmpl),
	}

	h.cli = newClient(h.VolatilePath(socketFilename))
	manifestFilePath := h.NodeBasePath(manifestFilename)

	if err := h.load(manifestFilePath); err != nil {
		return nil, err
	} else {
		return h, nil
	}
}
