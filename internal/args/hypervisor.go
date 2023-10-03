package args

import (
	"amuz.es/src/spi-ca/chmgr/internal/util"
	"fmt"
	"github.com/google/uuid"
	"strings"
)

type Hypervisor struct {
	Name       string          `yaml:"name"`
	Cpus       int             `yaml:"cpus"`
	Mem        util.IECSize    `yaml:"mem"`
	Uuid       uuid.UUID       `yaml:"uuid"`
	RootfsUuid uuid.UUID       `yaml:"rootfs_uuid"`
	Image      string          `yaml:"image"`
	NetMacAddr util.MACAddress `yaml:"net_mac_addr"`
	NetIfName  string          `yaml:"net_if_name"`
	Cmdline    []string        `yaml:"cmdline"`
	Disk       []string        `yaml:"disk"`
	Directory  []string        `yaml:"directory"`
}

func (i *Hypervisor) MachineId() string { return strings.ReplaceAll(i.Uuid.String(), "-", "") }

func (i *Hypervisor) KernelCommandline() string {
	args := append([]string(nil), i.Cmdline...)
	args = append(args, fmt.Sprintf("base=UUID=%s", i.RootfsUuid.String()))
	args = append(args, fmt.Sprintf("systemd.machine_id=%s", i.MachineId()))
	return strings.Join(args, " ")
}

func (i *Hypervisor) CpuArgs() string {
	var args []string

	args = append(args, fmt.Sprintf("boot=%d", i.Cpus))

	return strings.Join(args, ",")
}

func (i *Hypervisor) MemoryArgs() string {
	var args []string

	args = append(args, fmt.Sprintf("size=%s", i.Mem))
	args = append(args, "shared=on")
	args = append(args, "mergeable=on")
	args = append(args, "thp=on")

	return strings.Join(args, ",")
}

func (i *Hypervisor) BaloonArgs() string {
	var args []string

	args = append(args, "size=0")
	args = append(args, "free_page_reporting=on")

	return strings.Join(args, ",")
}

func (i *Hypervisor) PlatformArg() string {
	args := append([]string(nil), i.Cmdline...)
	args = append(args, fmt.Sprintf("oem_strings=amuzes-%s", i.Name))
	args = append(args, fmt.Sprintf("serial_number=%s", i.MachineId()))
	args = append(args, fmt.Sprintf("uuid=%s", i.Uuid.String()))
	return strings.Join(args, ",")
}

func (i *Hypervisor) NetworkInterfaceArgs() string {
	var args []string
	args = append(args, fmt.Sprintf("mac=%s", i.NetMacAddr))
	args = append(args, fmt.Sprintf("tap=%s", i.NetIfName))
	args = append(args, "host_mac=")
	args = append(args, "ip=")
	args = append(args, "mask=")
	args = append(args, "num_queues=2")
	args = append(args, "queue_size=128")

	return strings.Join(args, ",")
}

func (i *Hypervisor) DirectoryArgs(name string, socketPath string) string {
	var args []string
	args = append(args, fmt.Sprintf("tag=%s", name))
	args = append(args, fmt.Sprintf("socket=%s", socketPath))
	args = append(args, "num_queues=1")
	args = append(args, "queue_size=1024")

	return strings.Join(args, ",")
}

func (i *Hypervisor) DiskArgs(filePath string, readonly bool) string {
	var args []string

	args = append(args, fmt.Sprintf("path=%s", filePath))
	if readonly {
		args = append(args, "readonly=on")
	} else {
		args = append(args, "readonly=off")
	}
	args = append(args, "direct=on")
	args = append(args, "num_queues=2")
	args = append(args, "queue_size=128")

	return strings.Join(args, ",")
}
