package hvm

func Load(imageRoot, nodeRoot, volatileDirectory, manifestFilename, socketFilename string) (*Hypervisor, error) {

	h := &Hypervisor{
		imageRoot:         imageRoot,
		nodeRoot:          nodeRoot,
		volatileDirectory: volatileDirectory,
	}

	h.client = NewClient(h.NodeBasePath(socketFilename))

	err := h.load(manifestFilename)
	if err != nil {
		return nil, err
	}

	return h, nil
}
