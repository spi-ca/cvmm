package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"amuz.es/src/spi-ca/chmgr/internal/util"
	"github.com/google/uuid"
)

type Config struct {
	Cpus       int             `json:"cpus" yaml:"cpus"`
	Mem        util.IECSize    `json:"mem" yaml:"mem"`
	Uuid       uuid.UUID       `json:"uuid" yaml:"uuid"`
	RootfsUuid uuid.UUID       `json:"rootfs_uuid" yaml:"rootfs_uuid"`
	Image      string          `json:"image" yaml:"image"`
	NetMacAddr util.MACAddress `json:"net_mac_addr" yaml:"net_mac_addr"`
	NetIfName  string          `json:"net_if_name" yaml:"net_if_name"`
	Cmdline    []string        `json:"cmdline" yaml:"cmdline"`
	Disk       []string        `json:"disk" yaml:"disk"`
	Directory  []string        `json:"directory" yaml:"directory"`
}

func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var cfg Config
	d := yaml.NewDecoder(f)
	err = d.Decode(&cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}
func (i *Config) VirtiofsConfig(
	diskImageDirectoryPath,
	virtiofsdSocketPathTemplate string,
) []VirtiofsConfig {
	var cfgs []VirtiofsConfig

	for _, dir := range i.Directory {
		diskPath := dir
		if !filepath.IsAbs(dir) {
			diskPath = filepath.Join(diskImageDirectoryPath, dir)
		}
		name := filepath.Base(dir)

		cfg := VirtiofsConfig{
			Directory:      diskPath,
			SocketPath:     util.AppendFileSuffix(virtiofsdSocketPathTemplate, name),
			ThreadPoolSize: i.Cpus,
		}

		cfgs = append(cfgs, cfg)
	}
	return cfgs
}

func (i *Config) VMConfig(
	name,
	kernelPath,
	initramfsPath,
	rootfsPath,
	diskImageDirectoryPath string,
	virtiofsSocketPathTemplate string,
	consoleHasStd bool,
) VmConfig {
	return VmConfig{
		Payload:  i.PayloadConfig(kernelPath, initramfsPath),
		Platform: i.PlatformConfig(name),
		Cpus:     i.CpusConfig(),
		Memory:   i.MemoryConfig(),
		Disks:    i.DiskConfig(rootfsPath, diskImageDirectoryPath),
		Net:      i.NetConfig(),
		Rng:      i.RngConfig(),
		Balloon:  i.BalloonConfig(),
		Fs:       i.FsConfig(virtiofsSocketPathTemplate),
		Pmem:     i.PmemConfig(),
		Serial:   i.SerialConfig(),
		Console:  i.ConsoleConfig(consoleHasStd),
		Devices:  i.DeviceConfig(),
		Vdpa:     i.VdpaConfig(),
		Vsock:    i.VsockConfig(),
		Numa:     i.NumaConfig(),
		Watchdog: i.WatchdogConfig(),
		Pvpanic:  i.PvpanicConfig(),
		SgxEpc:   i.SgxEpcConfig(),
		Tpm:      i.TpmConfig(),
	}
}

func (i *Config) PayloadConfig(kernelPath, initramfsPath string) PayloadConfig {
	return PayloadConfig{
		Kernel:    kernelPath,
		Cmdline:   i.KernelCommandline(),
		Initramfs: initramfsPath,
	}
}

func (i *Config) PlatformConfig(name string) *PlatformConfig {
	return &PlatformConfig{
		SerialNumber: i.MachineId(),
		UUID:         i.Uuid,
		OemStrings:   []string{fmt.Sprintf("amuzes-%s", name)},
	}
}

func (i *Config) CpusConfig() *CpusConfig {
	return &CpusConfig{
		BootVcpus: i.Cpus,
		MaxVcpus:  i.Cpus,
	}
}

func (i *Config) MemoryConfig() *MemoryConfig {
	return &MemoryConfig{
		Size:      int64(i.Mem),
		Mergeable: true,
		Shared:    true,
		Thp:       true,
	}
}

func (i *Config) imageConfig(imageFilePath string) DiskConfig {
	return DiskConfig{
		Path:      imageFilePath,
		Readonly:  true,
		Direct:    true,
		NumQueues: i.Cpus,
		QueueSize: 128,
	}
}

func (i *Config) DiskConfig(imageFilePath string, diskImageDirectoryPath string) []DiskConfig {
	var (
		cfgs []DiskConfig
	)

	cfgs = append(cfgs, i.imageConfig(imageFilePath))
	for _, dir := range i.Disk {
		diskPath := dir
		if !filepath.IsAbs(dir) {
			diskPath = filepath.Join(diskImageDirectoryPath, dir)
		}
		cfg := DiskConfig{
			Path:      diskPath,
			Readonly:  false,
			Direct:    true,
			NumQueues: i.Cpus,
			QueueSize: 128,
		}
		cfgs = append(cfgs, cfg)
	}

	return cfgs
}

func (i *Config) NetConfig() []NetConfig {
	return []NetConfig{
		{
			Tap:       i.NetIfName,
			Mac:       i.NetMacAddr,
			NumQueues: i.Cpus,
			QueueSize: 128,
		},
	}
}

func (i *Config) RngConfig() *RngConfig {
	return &RngConfig{
		Src: "/dev/urandom",
	}
}

func (i *Config) BalloonConfig() *BalloonConfig {
	return &BalloonConfig{
		FreePageReporting: true,
	}
}

func (i *Config) FsConfig(virtiofsSocketPathTemplate string) []FsConfig {
	var (
		cfgs []FsConfig
	)

	for _, dir := range i.Directory {
		name := filepath.Base(dir)
		cfg := FsConfig{
			Tag:       name,
			Socket:    util.AppendFileSuffix(virtiofsSocketPathTemplate, name),
			NumQueues: 1, // virtiofs only supported single threaded
			QueueSize: 1024,
		}
		cfgs = append(cfgs, cfg)
	}

	return cfgs
}

func (i *Config) SerialConfig() *SerialConfig {
	return &SerialConfig{
		Mode: ConsoleModeOff,
	}
}
func (i *Config) ConsoleConfig(std bool) *ConsoleConfig {
	mode := ConsoleModePty
	if std {
		mode = ConsoleModeTty
	}
	return &ConsoleConfig{
		Mode: mode,
	}
}
func (i *Config) PmemConfig() []PmemConfig     { return nil }
func (i *Config) DeviceConfig() []DeviceConfig { return nil }
func (i *Config) VdpaConfig() []VdpaConfig     { return nil }
func (i *Config) VsockConfig() *VsockConfig    { return nil }
func (i *Config) NumaConfig() []NumaConfig     { return nil }
func (i *Config) WatchdogConfig() bool         { return true }
func (i *Config) PvpanicConfig() bool          { return true }
func (i *Config) SgxEpcConfig() []SgxEpcConfig { return nil }
func (i *Config) TpmConfig() *TpmConfig        { return nil }

func (i *Config) MachineId() string { return strings.ReplaceAll(i.Uuid.String(), "-", "") }

func (i *Config) KernelCommandline() string {
	args := append([]string(nil),
		fmt.Sprintf("base=UUID=%s", i.RootfsUuid.String()),
		fmt.Sprintf("systemd.machine_id=%s", i.MachineId()),
		"console=hvc0",
	)
	args = append(args, i.Cmdline...)
	return strings.Join(args, " ")
}
