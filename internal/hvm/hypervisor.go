package hvm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"amuz.es/src/spi-ca/chmgr/internal/args"
	"amuz.es/src/spi-ca/chmgr/internal/util"
	"gopkg.in/yaml.v3"
)

type Hypervisor struct {
	name              string `yaml:"-"`
	imageRoot         string `yaml:"-"`
	nodeHome          string `yaml:"-"`
	volatileDirectory string `yaml:"-"`

	args args.Hypervisor

	cli *client
}

func (i *Hypervisor) load(manifestFilename string) error {
	f, err := os.Open(i.NodeBasePath(manifestFilename))
	if err != nil {
		return err
	}

	defer func() { _ = f.Close() }()

	d := yaml.NewDecoder(f)
	err = d.Decode(i)
	if err != nil {
		return err
	}

	return nil
}
func (i *Hypervisor) Close()            { i.cli.Close() }
func (i *Hypervisor) GetClient() Client { return i.cli }
func (i *Hypervisor) ImageBasePath(rest ...string) string {
	args := []string{i.imageRoot, i.args.Image}
	args = append(args, rest...)
	return filepath.Join(args...)
}

func (i *Hypervisor) NodeBasePath(rest ...string) string {
	args := []string{i.nodeHome}
	args = append(args, rest...)
	return filepath.Join(args...)
}

func (i *Hypervisor) KernelCommandline() string {
	args := append([]string(nil), i.args.Cmdline...)
	args = append(args, fmt.Sprintf("base=UUID=%s", i.args.RootfsUuid))
	args = append(args, fmt.Sprintf("systemd.machine_id=%s", i.args.MachineId()))
	return strings.Join(args, " ")
}

func (i *Hypervisor) CpuArgs() string {
	var args []string

	args = append(args, fmt.Sprintf("boot=%d", i.args.Cpus))

	return strings.Join(args, ",")
}

func (i *Hypervisor) MemoryArgs() string {
	var args []string

	args = append(args, fmt.Sprintf("size=%s", i.args.Mem))
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
	args := append([]string(nil), i.args.Cmdline...)
	args = append(args, fmt.Sprintf("oem_strings=amuzes-%s", i.name))
	args = append(args, fmt.Sprintf("serial_number=%s", i.args.MachineId()))
	args = append(args, fmt.Sprintf("uuid=%s", i.args.Uuid.String()))
	return strings.Join(args, ",")
}

func (i *Hypervisor) NetworkInterfaceArgs() string {
	var args []string
	args = append(args, fmt.Sprintf("mac=%s", i.args.NetMacAddr))
	args = append(args, fmt.Sprintf("tap=%s", i.args.NetIfName))
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

func (i *Hypervisor) VolatilePath(rest ...string) string {
	args := []string{i.nodeHome, i.volatileDirectory}
	args = append(args, rest...)
	return filepath.Join(args...)
}

func (i *Hypervisor) VirtiofsArgs(virtiofsFilename string) []string {
	var (
		args                 []string
		virtiofsFilenameTmpl = util.F(virtiofsFilename)
	)

	for _, filename := range i.args.Directory {
		cfg := &VirtiofsConfig{
			Directory:      i.NodeBasePath(filename),
			SocketPath:     i.VolatilePath(virtiofsFilenameTmpl.R(util.FormatArgs{"directoryName": filename})),
			ThreadPoolSize: i.args.Cpus,
		}

		args = append(args, strings.Join(cfg.CommandArgs(), " "))
	}
	return args
}

func (i *Hypervisor) CommandArgs(
	kernelFilename,
	initramfsFilename,
	rootfsFilename,
	monitorFilename,
	virtiofsFilename string,
) []string {

	virtiofsFilenameTmpl := util.F(virtiofsFilename)

	var arguments []string
	arguments = append(
		arguments,
		"--platform", i.PlatformArg(),
		"--kernel", i.ImageBasePath(kernelFilename),
		"--initramfs", i.ImageBasePath(initramfsFilename),
		"--cmdline", i.KernelCommandline(),
		"--cpus", i.CpuArgs(),
		"--memory", i.MemoryArgs(),
		"--balloon", i.BaloonArgs(),
		"--console", "pty",
		"--serial", "off",
		"--api-socket", fmt.Sprintf("path=%s", i.VolatilePath(monitorFilename)),
		"--net", i.NetworkInterfaceArgs(),
		"--watchdog",
		"--pvpanic",
	)

	for _, filename := range i.args.Directory {
		arguments = append(arguments, "--fs", i.DirectoryArgs(filename, i.VolatilePath(virtiofsFilenameTmpl.R(util.FormatArgs{"directoryName": filename}))))
	}

	arguments = append(arguments, "--disk", i.DiskArgs(i.ImageBasePath(rootfsFilename), true))
	for _, filename := range i.args.Disk {
		arguments = append(arguments, "--disk", i.DiskArgs(i.NodeBasePath(filename), false))
	}

	return arguments
}
