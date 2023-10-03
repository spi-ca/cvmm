package hvm

import (
	"fmt"
	"strings"
	"testing"
)

func TestVirtiofsConfigUnit(t *testing.T) {
	cfg := &VirtiofsConfig{
		Directory:      "/srv/vmm/nodes/a/configuration",
		SocketPath:     "/srv/vmm/nodes/a/run/virtiofs_configuration.sock",
		ThreadPoolSize: 2,
	}
	args := cfg.CommandArgs()
	fmt.Printf("memorySize = %v\n", strings.Join(args, " \t\n"))
}
