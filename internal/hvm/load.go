package hvm

func Load(imageRoot, nodeRoot, volatileDirectory, manifestFilename, socketFilename string) (*Hypervisor, error) {

	h := &Hypervisor{
		imageRoot:         imageRoot,
		nodeRoot:          nodeRoot,
		volatileDirectory: volatileDirectory,
	}

	h.client = NewClient(h.NodeBasePath(socketFilename))

	if err := h.load(manifestFilename); err != nil {
		return nil, err
	} else {
		return h, nil
	}
}
