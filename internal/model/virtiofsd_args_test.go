package model

import (
	"fmt"
	"testing"
)

func TestVirtiofsConfigUnit(t *testing.T) {
	cfg := &VirtiofsConfig{
		Directory:      "/srv/vmm/nodes/a/configuration",
		SocketPath:     "/srv/vmm/nodes/a/run/virtiofs_configuration.sock",
		ThreadPoolSize: 2,
	}
	fmt.Printf("virtiofs config = %s\n", cfg)
}
