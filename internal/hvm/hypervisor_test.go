package hvm

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"strings"
	"testing"
)

func TestCfg(t *testing.T) {
	r := strings.NewReader(`name: kube-master-01
cpus: 2
mem: 4G
uuid: 87773d86-1479-4db4-9e90-e5a4314ff11b
rootfs_uuid: 3a42a0c0-dfd2-40b2-b9eb-861f2610a5c1
image: kube-master
net_mac_addr: 2e:f4:5f:11:1b:56
net_if_name: vmtap-km01
cmdline:
- console=hvc0
- cpuidle.governor=haltpoll
- clocksource=kvm-clock
- net.ifnames=0
- quiet
- loglevel=3
disk:
- data.img
directory:
- configuration`)
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

	for _, args := range cfg.VirtiofsArgs("/srv/vmm/nodes") {
		fmt.Printf("v = %s\n", args)
	}
	fmt.Printf("v = %v\n", cfg)
	fmt.Printf("v = %s\n",
		strings.Join(
			cfg.CommandArgs(
				"vmlinuz",
				"initramfs.img",
				"root.img",
				"monitor.sock",
				"virtiofs_{{.directoryName}}.sock",
			),
			" \\\n\t",
		),
	)
	//
	//newFd, err := syscall.Open(".", syscall.O_TMPFILE|os.O_RDWR|os.O_CREATE|os.O_APPEND|syscall.O_CLOEXEC, 0o644)
	//if err != nil {
	//	_ = os.Rename(rotateFilename, filename)
	//	return "", fmt.Errorf("failed to rename a file(%s): %w", filename, err)
	//}

	marshalled, err := yaml.Marshal(cfg)
	if err != nil {
		panic(err)
	}
	fmt.Printf("v = %s\n", string(marshalled))

}
