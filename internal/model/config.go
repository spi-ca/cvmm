package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"amuz.es/src/spi-ca/cvmm/internal/util"
	"github.com/google/uuid"
)

// Config describes configuration data that cvmm translates into runtime VM state.
type Config struct {
	Cpus       int             `json:"cpus" yaml:"cpus"`
	Mem        util.IECSize    `json:"mem" yaml:"mem"`
	Uuid       uuid.UUID       `json:"uuid" yaml:"uuid"`
	Image      string          `json:"image" yaml:"image"`
	NetMacAddr util.MACAddress `json:"net_mac_addr" yaml:"net_mac_addr"`
	NetIfName  string          `json:"net_if_name" yaml:"net_if_name"`
	Cmdline    []string        `json:"cmdline" yaml:"cmdline"`
	Disk       []string        `json:"disk" yaml:"disk"`
	Directory  []string        `json:"directory" yaml:"directory"`
}

// LoadConfig reads and decodes a YAML node manifest from path.
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

// VirtiofsConfig derives one virtiofsd configuration for each manifest directory entry.
func (i *Config) VirtiofsConfig(
	diskImageDirectoryPath,
	virtiofsdSocketPathTemplate,
	virtiofsdPidPathTemplate,
	hypervisorRunAsGroup string,
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
			PidPath:        util.AppendFileSuffix(virtiofsdPidPathTemplate, name),
			SocketGroup:    hypervisorRunAsGroup,
			ThreadPoolSize: i.Cpus,
		}

		cfgs = append(cfgs, cfg)
	}
	return cfgs
}

// VMConfig converts the node manifest into the cloud-hypervisor VM create payload.
func (i *Config) VMConfig(
	name,
	kernelPath,
	initramfsPath,
	rootfsPath,
	diskImageDirectoryPath,
	virtiofsSocketPathTemplate string,
	consoleHasStd bool,
) VmConfig {
	return VmConfig{
		Payload:         i.PayloadConfig(kernelPath, initramfsPath),
		RateLimitGroups: i.RateLimitGroupsConfig(),
		Platform:        i.PlatformConfig(name),
		Cpus:            i.CpusConfig(),
		Memory:          i.MemoryConfig(),
		Disks:           i.DiskConfig(rootfsPath, diskImageDirectoryPath),
		Net:             i.NetConfig(),
		Rng:             i.RngConfig(),
		Balloon:         i.BalloonConfig(),
		Fs:              i.FsConfig(virtiofsSocketPathTemplate),
		Pmem:            i.PmemConfig(),
		Serial:          i.SerialConfig(),
		Console:         i.ConsoleConfig(consoleHasStd),
		DebugConsole:    i.DebugConsoleConfig(),
		Devices:         i.DeviceConfig(),
		Vdpa:            i.VdpaConfig(),
		Vsock:           i.VsockConfig(),
		Numa:            i.NumaConfig(),
		Watchdog:        i.WatchdogConfig(),
		Pvpanic:         i.PvpanicConfig(),
		SgxEpc:          i.SgxEpcConfig(),
		PciSegment:      i.PciSegmentsConfig(),
		Tpm:             i.TpmConfig(),
		Landlock:        i.LandlockEnableConfig(),
		LandlockRules:   i.LandlockRulesConfig(),
	}
}

// PayloadConfig builds the kernel/initramfs payload section for the guest.
func (i *Config) PayloadConfig(kernelPath, initramfsPath string) PayloadConfig {
	return PayloadConfig{
		Kernel:    kernelPath,
		Cmdline:   i.KernelCommandline(),
		Initramfs: initramfsPath,
	}
}

// PlatformConfig builds the platform identity from the node UUID and name.
func (i *Config) PlatformConfig(name string) *PlatformConfig {
	return &PlatformConfig{
		SerialNumber: i.MachineId(),
		UUID:         i.Uuid,
		OemStrings:   []string{fmt.Sprintf("amuzes-%s", name)},
	}
}

// CpusConfig maps the manifest CPU count to boot and maximum vCPU settings.
func (i *Config) CpusConfig() *CpusConfig {
	return &CpusConfig{
		BootVcpus: i.Cpus,
		MaxVcpus:  i.Cpus,
	}
}

// MemoryConfig maps the manifest memory size to shared THP-enabled guest memory.
func (i *Config) MemoryConfig() *MemoryConfig {
	return &MemoryConfig{
		Size:      int64(i.Mem),
		Mergeable: true,
		Shared:    true,
		Thp:       true,
	}
}

// imageConfig builds the readonly root disk entry for the image rootfs.
func (i *Config) imageConfig(imageFilePath string) DiskConfig {
	return DiskConfig{
		Path:     imageFilePath,
		Readonly: true,
	}
}

// DiskConfig combines the readonly rootfs with manifest writable disks.
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
			Path:     diskPath,
			Readonly: false,
		}
		cfgs = append(cfgs, cfg)
	}

	return cfgs
}

// NetConfig builds the TAP network payload from the manifest or generated defaults.
func (i *Config) NetConfig() []NetConfig {
	return []NetConfig{
		{
			Tap:       i.NetIfName,
			Mac:       i.NetMacAddr,
			NumQueues: i.Cpus,
			QueueSize: 1024,
		},
	}
}

// RngConfig configures the guest RNG source.
func (i *Config) RngConfig() *RngConfig {
	return &RngConfig{
		Src: "/dev/urandom",
	}
}

// BalloonConfig enables guest balloon free page reporting.
func (i *Config) BalloonConfig() *BalloonConfig {
	return &BalloonConfig{
		FreePageReporting: true,
	}
}

// FsConfig maps manifest directories to virtio-fs tags and sockets.
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

// SerialConfig disables the serial console path by default.
func (i *Config) SerialConfig() *SerialConfig {
	return &SerialConfig{
		Mode: ConsoleModeOff,
	}
}

// ConsoleConfig selects PTY or stdio console mode based on the CLI console flag.
func (i *Config) ConsoleConfig(std bool) *ConsoleConfig {
	mode := ConsoleModePty
	if std {
		mode = ConsoleModeTty
	}
	return &ConsoleConfig{
		Mode: mode,
	}
}

// DebugConsoleConfig returns nil so generated VM payloads omit debug-console configuration.
func (i *Config) DebugConsoleConfig() *DebugConsoleConfig { return nil }

// PmemConfig returns nil so generated VM payloads omit persistent memory devices.
func (i *Config) PmemConfig() []PmemConfig { return nil }

// DeviceConfig returns nil so generated VM payloads omit direct device passthrough entries.
func (i *Config) DeviceConfig() []DeviceConfig { return nil }

// RateLimitGroupsConfig returns nil so generated VM payloads omit rate limiter groups.
func (i *Config) RateLimitGroupsConfig() []RateLimitGroupConfig { return nil }

// VdpaConfig returns nil so generated VM payloads omit VDPA devices.
func (i *Config) VdpaConfig() []VdpaConfig { return nil }

// VsockConfig returns nil so generated VM payloads omit vsock devices.
func (i *Config) VsockConfig() *VsockConfig { return nil }

// NumaConfig returns nil so generated VM payloads omit NUMA topology.
func (i *Config) NumaConfig() []NumaConfig { return nil }

// WatchdogConfig enables the cloud-hypervisor watchdog device.
func (i *Config) WatchdogConfig() bool { return true }

// PvpanicConfig enables the guest pvpanic device.
func (i *Config) PvpanicConfig() bool { return true }

// SgxEpcConfig returns nil so generated VM payloads omit SGX EPC sections.
func (i *Config) SgxEpcConfig() []SgxEpcConfig { return nil }

// PciSegmentsConfig returns nil so generated VM payloads omit PCI segment configuration.
func (i *Config) PciSegmentsConfig() []PciSegmentConfig { return nil }

// TpmConfig returns nil so generated VM payloads omit TPM configuration.
func (i *Config) TpmConfig() *TpmConfig { return nil }

// LandlockEnableConfig returns false so generated VM payloads disable cloud-hypervisor Landlock.
func (i *Config) LandlockEnableConfig() bool { return false }

// LandlockRulesConfig returns nil so generated VM payloads omit explicit Landlock rules.
func (i *Config) LandlockRulesConfig() []LandlockConfig { return nil }

// MachineId returns the UUID without dashes for platform serial and kernel command line use.
func (i *Config) MachineId() string { return strings.ReplaceAll(i.Uuid.String(), "-", "") }

// KernelCommandline prepends cvmm defaults to manifest-provided kernel arguments.
func (i *Config) KernelCommandline() string {
	args := append([]string(nil),
		fmt.Sprintf("systemd.machine_id=%s", i.MachineId()),
		"console=hvc0",
	)
	args = append(args, i.Cmdline...)
	return strings.Join(args, " ")
}
