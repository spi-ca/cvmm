package hvm

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"strings"
	"testing"
)

func TestCfg(t *testing.T) {
	r := strings.NewReader(`cpus: 2
mem: 2G
uuid: a6a7a918-1886-45d5-adb5-aaabdca22f16
rootfs_uuid: 3a42a0c0-dfd2-40b2-b9eb-861f2610a5c1
image: test
net_if_name: vmtap-tst
disk:
  - data.img
directory:
  - cfg
`)
	cfg := &Hypervisor{
		name:              "mock",
		imageRoot:         "/srv/vmm/images",
		nodeHome:          "/srv/vmm/nodes/mock",
		volatileDirectory: "/srv/vmm/nodes/mock/run",
	}

	d := yaml.NewDecoder(r)
	err := d.Decode(cfg)
	if err != nil {
		panic(err)
	}

	//for _, args := range cfg.VirtiofsArgs() {
	//	fmt.Printf("v = %s\n", args)
	//}
	fmt.Printf("v = %v\n", cfg)
	cfg.args.VMConfig(
		i.name,
		i.ImageBasePath(kernelFilename), i.ImageBasePath(initramfsFilename),
		i.ImageBasePath(rootfsFilename), i.NodeBasePath,
		virtiofsSocketPathResolver,
	)
	fmt.Printf("v = %s\n",
		strings.Join(
			cfg.args.(
				"vmlinuz",
				"initramfs.img",
				"root.img",
				"api.sock",
				"virtiofs_{{.directoryName}}.sock",
			),
			" \\\n\t",
		),
	)
	marshalled, err := yaml.Marshal(cfg)
	if err != nil {
		panic(err)
	}
	fmt.Printf("v = %s\n", string(marshalled))

}
