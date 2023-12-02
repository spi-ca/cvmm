package model

import (
	"fmt"
	"net"
	"strings"

	"amuz.es/src/spi-ca/cvmm/internal/util"
	"github.com/google/uuid"
)

type (
	// VmmPingResponse is presentation structure for the Virtual Machine Monitor information
	VmmPingResponse struct {
		BuildVersion string   `json:"build_version,omitempty" yaml:"build_version,omitempty"`
		Version      string   `json:"version" yaml:"version"`
		PID          int64    `json:"pid" yaml:"pid"`
		Features     []string `json:"features" yaml:"features"`
	}

	// VmInfo is presentation structure for the Virtual Machine information
	VmInfo struct {
		Config           VmConfig              `json:"config" yaml:"config"`
		State            NodeStatus            `json:"state" yaml:"state"` // Enum: [Created, Running, Shutdown, Paused]
		MemoryActualSize int64                 `json:"memory_actual_size" yaml:"memory_actual_size"`
		DeviceTree       map[string]DeviceNode `json:"device_tree" yaml:"device_tree"`
	}

	DeviceResource struct {
		PciBar *PciDeviceInfo `json:"pciBar,omitempty" yaml:"pciBar,omitempty"`
	}

	PciResourceInfo struct {
		Index        int    `json:"index" yaml:"index"`
		Base         int64  `json:"base" yaml:"base"`
		Size         int    `json:"size" yaml:"size"`
		Type         string `json:"type" yaml:"type"`
		Prefetchable bool   `json:"prefetchable" yaml:"prefetchable"`
	}

	DeviceNode struct {
		ID        string           `json:"id" yaml:"id"`
		Resources []DeviceResource `json:"resources" yaml:"resources"` // It's represented as a generic object since there's a comment indicating Rust enum type.
		Children  []string         `json:"children" yaml:"children"`
		PciBdf    string           `json:"pci_bdf" yaml:"pci_bdf"`
	}

	VMCounter struct {
		// BlockCounters
		WriteLatencyMin uint64 `json:"write_latency_min,omitempty"`
		ReadBytes       uint64 `json:"read_bytes,omitempty"`
		ReadLatencyMin  uint64 `json:"read_latency_min,omitempty"`
		WriteLatencyMax uint64 `json:"write_latency_max,omitempty"`
		ReadLatencyAvg  uint64 `json:"read_latency_avg,omitempty"`
		ReadOps         uint64 `json:"read_ops,omitempty"`
		WriteLatencyAvg uint64 `json:"write_latency_avg,omitempty"`
		WriteBytes      uint64 `json:"write_bytes,omitempty"`
		ReadLatencyMax  uint64 `json:"read_latency_max,omitempty"`
		WriteOps        uint64 `json:"write_ops,omitempty"`
		// NetCounters
		RxFrames uint64 `json:"rx_frames,omitempty"`
		TxBytes  uint64 `json:"tx_bytes,omitempty"`
		RxBytes  uint64 `json:"rx_bytes,omitempty"`
		TxFrames uint64 `json:"tx_frames,omitempty"`
	}

	VmCounters map[string]VMCounter

	// PciDeviceInfo is presentation structure for the Information about a PCI device
	PciDeviceInfo struct {
		ID  string `json:"id" yaml:"id"`
		Bdf string `json:"bdf" yaml:"bdf"`
	}

	// PayloadConfig is presentation structure for the Payloads to boot in guest
	PayloadConfig struct {
		Firmware  string `json:"firmware,omitempty" yaml:"firmware,omitempty"`
		Kernel    string `json:"kernel,omitempty" yaml:"kernel,omitempty"`
		Cmdline   string `json:"cmdline,omitempty" yaml:"cmdline,omitempty"`
		Initramfs string `json:"initramfs,omitempty" yaml:"initramfs,omitempty"`
	}

	// VmConfig is presentation structure for the Virtual machine configuration
	VmConfig struct {
		Cpus     *CpusConfig     `json:"cpus,omitempty" yaml:"cpus,omitempty"`
		Memory   *MemoryConfig   `json:"memory,omitempty" yaml:"memory,omitempty"`
		Payload  PayloadConfig   `json:"payload" yaml:"payload"`
		Disks    []DiskConfig    `json:"disks,omitempty" yaml:"disks,omitempty"`
		Net      []NetConfig     `json:"net,omitempty" yaml:"net,omitempty"`
		Rng      *RngConfig      `json:"rng,omitempty" yaml:"rng,omitempty"`
		Balloon  *BalloonConfig  `json:"balloon,omitempty" yaml:"balloon,omitempty"`
		Fs       []FsConfig      `json:"fs,omitempty" yaml:"fs,omitempty"`
		Pmem     []PmemConfig    `json:"pmem,omitempty" yaml:"pmem,omitempty"`
		Serial   *SerialConfig   `json:"serial,omitempty" yaml:"serial,omitempty"`
		Console  *ConsoleConfig  `json:"console,omitempty" yaml:"console,omitempty"`
		Devices  []DeviceConfig  `json:"devices,omitempty" yaml:"devices,omitempty"`
		Vdpa     []VdpaConfig    `json:"vdpa,omitempty" yaml:"vdpa,omitempty"`
		Vsock    *VsockConfig    `json:"vsock,omitempty" yaml:"vsock,omitempty"`
		SgxEpc   []SgxEpcConfig  `json:"sgx_epc,omitempty" yaml:"sgx_epc,omitempty"`
		Numa     []NumaConfig    `json:"numa,omitempty" yaml:"numa,omitempty"`
		Iommu    bool            `json:"iommu,omitempty" yaml:"iommu,omitempty"`
		Watchdog bool            `json:"watchdog,omitempty" yaml:"watchdog,omitempty"`
		Pvpanic  bool            `json:"pvpanic,omitempty" yaml:"pvpanic,omitempty"`
		Platform *PlatformConfig `json:"platform,omitempty" yaml:"platform,omitempty"`
		Tpm      *TpmConfig      `json:"tpm,omitempty" yaml:"tpm,omitempty"`
	}

	CpuAffinity struct {
		Vcpu     int   `json:"vcpu" yaml:"vcpu"`
		HostCpus []int `json:"host_cpus" yaml:"host_cpus"`
	}

	CpuFeatures struct {
		Amx bool `json:"amx,omitempty" yaml:"amx,omitempty"`
	}

	CpuTopology struct {
		ThreadsPerCore int `json:"threads_per_core,omitempty" yaml:"threads_per_core,omitempty"`
		CoresPerDie    int `json:"cores_per_die,omitempty" yaml:"cores_per_die,omitempty"`
		DiesPerPackage int `json:"dies_per_package,omitempty" yaml:"dies_per_package,omitempty"`
		Packages       int `json:"packages,omitempty" yaml:"packages,omitempty"`
	}

	CpusConfig struct {
		BootVcpus   int           `json:"boot_vcpus" yaml:"boot_vcpus"`
		MaxVcpus    int           `json:"max_vcpus" yaml:"max_vcpus"`
		Topology    *CpuTopology  `json:"topology,omitempty" yaml:"topology,omitempty"`
		KvmHyperv   bool          `json:"kvm_hyperv,omitempty" yaml:"kvm_hyperv,omitempty"`
		MaxPhysBits int           `json:"max_phys_bits,omitempty" yaml:"max_phys_bits,omitempty"`
		Affinity    []CpuAffinity `json:"affinity,omitempty" yaml:"affinity,omitempty"`
		Features    *CpuFeatures  `json:"features,omitempty" yaml:"features,omitempty"`
	}

	PlatformConfig struct {
		NumPciSegments int       `json:"num_pci_segments,omitempty" yaml:"num_pci_segments,omitempty"`
		IommuSegments  []int16   `json:"iommu_segments,omitempty" yaml:"iommu_segments,omitempty"`
		SerialNumber   string    `json:"serial_number,omitempty" yaml:"serial_number,omitempty"`
		UUID           uuid.UUID `json:"uuid,omitempty" yaml:"uuid,omitempty"`
		OemStrings     []string  `json:"oem_strings,omitempty" yaml:"oem_strings,omitempty"`
		Tdx            bool      `json:"tdx,omitempty" yaml:"tdx,omitempty"`
	}

	MemoryZoneConfig struct {
		ID             string `json:"id" yaml:"id"`
		Size           int64  `json:"size" yaml:"size"` //default: 512 MB
		File           string `json:"file,omitempty" yaml:"file,omitempty"`
		Mergeable      bool   `json:"mergeable,omitempty" yaml:"mergeable,omitempty"`
		Shared         bool   `json:"shared,omitempty" yaml:"shared,omitempty"`
		Hugepages      bool   `json:"hugepages,omitempty" yaml:"hugepages,omitempty"`
		HugepageSize   int64  `json:"hugepage_size,omitempty" yaml:"hugepage_size,omitempty"`
		HostNumaNode   int32  `json:"host_numa_node,omitempty" yaml:"host_numa_node,omitempty"`
		HotplugSize    int64  `json:"hotplug_size,omitempty" yaml:"hotplug_size,omitempty"`
		HotpluggedSize int64  `json:"hotplugged_size,omitempty" yaml:"hotplugged_size,omitempty"`
		Prefault       bool   `json:"prefault,omitempty" yaml:"prefault,omitempty"`
	}

	MemoryConfig struct {
		Size           int64              `json:"size" yaml:"size"` //default: 512 MB
		HotplugSize    int64              `json:"hotplug_size,omitempty" yaml:"hotplug_size,omitempty"`
		HotpluggedSize int64              `json:"hotplugged_size,omitempty" yaml:"hotplugged_size,omitempty"`
		Mergeable      bool               `json:"mergeable,omitempty" yaml:"mergeable,omitempty"`
		HotplugMethod  string             `json:"hotplug_method,omitempty" yaml:"hotplug_method,omitempty"` //default: "Acpi"
		Shared         bool               `json:"shared,omitempty" yaml:"shared,omitempty"`
		Hugepages      bool               `json:"hugepages,omitempty" yaml:"hugepages,omitempty"`
		HugepageSize   int64              `json:"hugepage_size,omitempty" yaml:"hugepage_size,omitempty"`
		Prefault       bool               `json:"prefault,omitempty" yaml:"prefault,omitempty"`
		Thp            bool               `json:"thp,omitempty" yaml:"thp,omitempty"`
		Zones          []MemoryZoneConfig `json:"zones,omitempty" yaml:"zones,omitempty"`
	}

	// TokenBucket defines a token bucket with a maximum capacity (_size_), an initial burst size
	// (_one_time_burst_) and an interval for refilling purposes (_refill_time_).
	// The refill-rate is derived from _size_ and _refill_time_, and it is the constant
	// rate at which the tokens replenish. The refill process only starts happening after
	// the initial burst budget is consumed.
	// Consumption from the token bucket is unbounded in speed which allows for bursts
	// bound in size by the amount of tokens available.
	// Once the token bucket is empty, consumption speed is bound by the refill-rate.
	TokenBucket struct {
		// the total number of tokens this bucket can hold.
		Size int64 `json:"size" yaml:"size"`
		// The initial size of a token bucket.
		OneTimeBurst int64 `json:"one_time_burst,omitempty" yaml:"one_time_burst,omitempty"`
		// The amount of milliseconds it takes for the bucket to refill.
		RefillTime int64 `json:"refill_time" yaml:"refill_time"`
	}

	// RateLimiterConfig Defines an IO rate limiter with independent bytes/s and ops/s limits.
	// Limits are defined by configuring each of the _bandwidth_ and _ops_ token buckets.
	RateLimiterConfig struct {
		Bandwidth *TokenBucket `json:"bandwidth,omitempty" yaml:"bandwidth,omitempty"`
		Ops       *TokenBucket `json:"ops,omitempty" yaml:"ops,omitempty"`
	}

	DiskConfig struct {
		Path              string             `json:"path" yaml:"path"`
		Readonly          bool               `json:"readonly,omitempty" yaml:"readonly,omitempty"`
		Direct            bool               `json:"direct,omitempty" yaml:"direct,omitempty"`
		Iommu             bool               `json:"iommu,omitempty" yaml:"iommu,omitempty"`
		NumQueues         int                `json:"num_queues,omitempty" yaml:"num_queues,omitempty"` // default: 1
		QueueSize         int                `json:"queue_size,omitempty" yaml:"queue_size,omitempty"` // default: 128
		VhostUser         bool               `json:"vhost_user,omitempty" yaml:"vhost_user,omitempty"`
		VhostSocket       string             `json:"vhost_socket,omitempty" yaml:"vhost_socket,omitempty"`
		RateLimiterConfig *RateLimiterConfig `json:"rate_limiter_config,omitempty" yaml:"rate_limiter_config,omitempty"`
		PciSegment        int16              `json:"pci_segment,omitempty" yaml:"pci_segment,omitempty"`
		ID                string             `json:"id,omitempty" yaml:"id,omitempty"`
		Serial            string             `json:"serial,omitempty" yaml:"serial,omitempty"`
	}

	NetConfig struct {
		Tap               string             `json:"tap,omitempty" yaml:"tap,omitempty"`
		IP                net.IP             `json:"ip,omitempty" yaml:"ip,omitempty"`     // default: "192.168.249.1"
		Mask              net.IP             `json:"mask,omitempty" yaml:"mask,omitempty"` // default: "255.255.255.0"
		Mac               util.MACAddress    `json:"mac,omitempty" yaml:"mac,omitempty"`
		HostMac           util.MACAddress    `json:"host_mac,omitempty" yaml:"host_mac,omitempty"`
		MTU               int                `json:"mtu,omitempty" yaml:"mtu,omitempty"`
		Iommu             bool               `json:"iommu,omitempty" yaml:"iommu,omitempty"`
		NumQueues         int                `json:"num_queues,omitempty" yaml:"num_queues,omitempty"` // default: 2
		QueueSize         int                `json:"queue_size,omitempty" yaml:"queue_size,omitempty"` // default: 256
		VhostUser         bool               `json:"vhost_user,omitempty" yaml:"vhost_user,omitempty"`
		VhostSocket       string             `json:"vhost_socket,omitempty" yaml:"vhost_socket,omitempty"`
		VhostMode         string             `json:"vhost_mode,omitempty" yaml:"vhost_mode,omitempty"` // default:  "Client"
		ID                string             `json:"id,omitempty" yaml:"id,omitempty"`
		PciSegment        int16              `json:"pci_segment,omitempty" yaml:"pci_segment,omitempty"`
		RateLimiterConfig *RateLimiterConfig `json:"rate_limiter_config,omitempty" yaml:"rate_limiter_config,omitempty"`
	}

	RngConfig struct {
		Src   string `json:"src" yaml:"src"`
		Iommu bool   `json:"iommu,omitempty" yaml:"iommu,omitempty"`
	}

	BalloonConfig struct {
		Size int64 `json:"size" yaml:"size"`
		//Deflate balloon when the guest is under memory pressure.
		DeflateOnOom bool `json:"deflate_on_oom" yaml:"deflate_on_oom"`
		//Enable guest to report free pages
		FreePageReporting bool `json:"free_page_reporting" yaml:"free_page_reporting"`
	}

	FsConfig struct {
		Tag        string `json:"tag" yaml:"tag"`
		Socket     string `json:"socket" yaml:"socket"`
		NumQueues  int    `json:"num_queues,omitempty" yaml:"num_queues,omitempty"` // default: 1
		QueueSize  int    `json:"queue_size,omitempty" yaml:"queue_size,omitempty"` // default: 1024
		PciSegment int16  `json:"pci_segment,omitempty" yaml:"pci_segment,omitempty"`
		ID         string `json:"id,omitempty" yaml:"id,omitempty"`
	}

	PmemConfig struct {
		File          string `json:"file" yaml:"file"`
		Size          int64  `json:"size,omitempty" yaml:"size,omitempty"`
		Iommu         bool   `json:"iommu,omitempty" yaml:"iommu,omitempty"`
		DiscardWrites bool   `json:"discard_writes,omitempty" yaml:"discard_writes,omitempty"`
		PciSegment    int16  `json:"pci_segment,omitempty" yaml:"pci_segment,omitempty"`
		ID            string `json:"id,omitempty" yaml:"id,omitempty"`
	}

	SerialConfig struct {
		File string      `json:"file,omitempty" yaml:"file,omitempty"`
		Mode ConsoleMode `json:"mode" yaml:"mode"`
	}

	ConsoleConfig struct {
		File  string      `json:"file,omitempty" yaml:"file,omitempty"`
		Mode  ConsoleMode `json:"mode" yaml:"mode"`
		Iommu bool        `json:"iommu,omitempty" yaml:"iommu,omitempty"`
	}

	DeviceConfig struct {
		Path       string `json:"path" yaml:"path"`
		Iommu      bool   `json:"iommu,omitempty" yaml:"iommu,omitempty"`
		PciSegment int16  `json:"pci_segment,omitempty" yaml:"pci_segment,omitempty"`
		ID         string `json:"id,omitempty" yaml:"id,omitempty"`
	}

	TpmConfig struct {
		Socket string `json:"socket" yaml:"socket"`
	}

	VdpaConfig struct {
		Path       string `json:"path" yaml:"path"`
		NumQueues  int    `json:"num_queues,omitempty" yaml:"num_queues,omitempty"` // default: 1
		Iommu      bool   `json:"iommu,omitempty" yaml:"iommu,omitempty"`
		PciSegment int16  `json:"pci_segment,omitempty" yaml:"pci_segment,omitempty"`
		ID         string `json:"id,omitempty" yaml:"id,omitempty"`
	}

	VsockConfig struct {
		//Guest Vsock CID
		CID int64 `json:"cid" yaml:"cid"`
		//Path to UNIX domain socket, used to proxy vsock connections.
		Socket     string `json:"socket" yaml:"socket"`
		Iommu      bool   `json:"iommu,omitempty" yaml:"iommu,omitempty"`
		PciSegment int16  `json:"pci_segment,omitempty" yaml:"pci_segment,omitempty"`
		ID         string `json:"id,omitempty" yaml:"id,omitempty"`
	}

	SgxEpcConfig struct {
		ID       string `json:"id" yaml:"id"`
		Size     int64  `json:"size,omitempty" yaml:"size,omitempty"`
		Prefault bool   `json:"prefault,omitempty" yaml:"prefault,omitempty"`
	}

	NumaDistance struct {
		Destination int32 `json:"destination" yaml:"destination"`
		Distance    int32 `json:"distance" yaml:"distance"`
	}

	NumaConfig struct {
		GuestNumaID    int32          `json:"guest_numa_id" yaml:"guest_numa_id"`
		Cpus           []int32        `json:"cpus,omitempty" yaml:"cpus,omitempty"`
		Distances      []NumaDistance `json:"distances,omitempty" yaml:"distances,omitempty"`
		MemoryZones    []string       `json:"memory_zones,omitempty" yaml:"memory_zones,omitempty"`
		SgxEpcSections []string       `json:"sgx_epc_sections,omitempty" yaml:"sgx_epc_sections,omitempty"`
	}

	VmResize struct {
		DesiredVcpus int `json:"desired_vcpus,omitempty" yaml:"desired_vcpus,omitempty"`
		//desired memory ram in bytes
		DesiredRam int64 `json:"desired_ram,omitempty" yaml:"desired_ram,omitempty"`
		//desired balloon size in bytes
		DesiredBalloon int64 `json:"desired_balloon,omitempty" yaml:"desired_balloon,omitempty"`
	}

	VmResizeZone struct {
		ID string `json:"id" yaml:"id"`
		//desired memory zone size in bytes
		DesiredRam int64 `json:"desired_ram" yaml:"desired_ram"`
	}

	VmRemoveDevice struct {
		ID string `json:"id" yaml:"id"`
	}

	VmSnapshotConfig struct {
		DestinationURL string `json:"destination_url" yaml:"destination_url"`
	}

	VmCoredumpData struct {
		DestinationURL string `json:"destination_url" yaml:"destination_url"`
	}

	RestoreConfig struct {
		SourceURL string `json:"source_url" yaml:"source_url"`
		Prefault  bool   `json:"prefault,omitempty" yaml:"prefault,omitempty"`
	}

	ReceiveMigrationData struct {
		ReceiverURL string `json:"receiver_url" yaml:"receiver_url"`
	}

	SendMigrationData struct {
		DestinationURL string `json:"destination_url" yaml:"destination_url"`
		Local          bool   `json:"local,omitempty" yaml:"local,omitempty"`
	}
)

func (c CpusConfig) String() string       { return joinArgs(c.CommandArgs()) }
func (p PlatformConfig) String() string   { return joinArgs(p.CommandArgs()) }
func (m MemoryZoneConfig) String() string { return joinArgs(m.CommandArgs()) }
func (m MemoryConfig) String() string     { return joinArgs(m.CommandArgs()) }
func (p PayloadConfig) String() string    { return joinArgs(p.CommandArgs()) }
func (d DiskConfig) String() string       { return joinArgs(d.CommandArgs()) }
func (n NetConfig) String() string        { return joinArgs(n.CommandArgs()) }
func (r RngConfig) String() string        { return joinArgs(r.CommandArgs()) }
func (b BalloonConfig) String() string    { return joinArgs(b.CommandArgs()) }
func (f FsConfig) String() string         { return joinArgs(f.CommandArgs()) }
func (p PmemConfig) String() string       { return joinArgs(p.CommandArgs()) }
func (c SerialConfig) String() string     { return joinArgs(c.CommandArgs()) }
func (c ConsoleConfig) String() string    { return joinArgs(c.CommandArgs()) }
func (d DeviceConfig) String() string     { return joinArgs(d.CommandArgs()) }
func (v VdpaConfig) String() string       { return joinArgs(v.CommandArgs()) }
func (v VsockConfig) String() string      { return joinArgs(v.CommandArgs()) }
func (n NumaConfig) String() string       { return joinArgs(n.CommandArgs()) }
func (t TpmConfig) String() string        { return joinArgs(t.CommandArgs()) }
func (t SgxEpcConfig) String() string     { return joinArgs(t.CommandArgs()) }
func (c VmConfig) String() string         { return joinArgs(c.CommandArgs()) }

func (c CpuTopology) String() string {
	return fmt.Sprintf("%d:%d:%d:%d", c.ThreadsPerCore, c.CoresPerDie, c.DiesPerPackage, c.Packages)
}

func (c CpuAffinity) String() string {
	return fmt.Sprintf("%d@[%s]", c.Vcpu, util.ConsecutiveRanges(c.HostCpus).String())
}

func (c CpuFeatures) String() string {
	var args = []string{}
	if c.Amx {
		args = append(args, "amx")
	}
	return strings.Join(args, ",")
}

func (v NumaDistance) String() string { return fmt.Sprintf("%d@%d", v.Destination, v.Distance) }

func (r RateLimiterConfig) String() string {
	var args []string

	if t := r.Bandwidth; t != nil {
		if t.Size > 0 {
			args = append(args, fmt.Sprintf("bw_size=%d", t.Size))
		}

		if t.OneTimeBurst > 0 {
			args = append(args, fmt.Sprintf("bw_one_time_burst=%d", t.OneTimeBurst))
		}

		if t.RefillTime > 0 {
			args = append(args, fmt.Sprintf("bw_refill_time=%d", t.RefillTime))
		}
	}

	if t := r.Ops; t != nil {
		if t.Size > 0 {
			args = append(args, fmt.Sprintf("ops_size=%d", t.Size))
		}

		if t.OneTimeBurst > 0 {
			args = append(args, fmt.Sprintf("ops_one_time_burst=%d", t.OneTimeBurst))
		}

		if t.RefillTime > 0 {
			args = append(args, fmt.Sprintf("ops_refill_time=%d", t.RefillTime))
		}
	}

	return strings.Join(args, ",")
}
func (c CpusConfig) CommandArgs() []string {

	var args []string

	if c.BootVcpus > 0 {
		args = append(args, fmt.Sprintf("boot=%d", c.BootVcpus))
	}

	if c.MaxVcpus > 0 {
		args = append(args, fmt.Sprintf("max=%d", c.BootVcpus))
	}

	if c.Topology != nil {
		cfg := c.Topology.String()
		if len(cfg) > 0 {
			args = append(args, fmt.Sprintf("topology=%s", c.Topology))
		}
	}

	if c.KvmHyperv {
		args = append(args, "kvm_hyperv=on")
	}

	if c.MaxPhysBits > 0 {
		args = append(args, fmt.Sprintf("max_phys_bits=%d", c.MaxPhysBits))
	}

	if len(c.Affinity) > 0 {
		var affinities []string
		for _, a := range c.Affinity {
			affinities = append(affinities, fmt.Sprintf("%s", a))
		}
		args = append(args, fmt.Sprintf("affinity=[%s]", strings.Join(affinities, ",")))
	}

	if c.Features != nil {
		cfg := c.Features.String()
		if len(cfg) > 0 {
			args = append(args, fmt.Sprintf("features=[%s]", c.Features))
		}
	}

	if len(args) > 0 {
		return append([]string(nil), "--cpus", strings.Join(args, ","))
	} else {
		return nil
	}
}

func (p PlatformConfig) CommandArgs() []string {
	var args []string

	if p.NumPciSegments > 0 {
		args = append(args, fmt.Sprintf("num_pci_segments=%d", p.NumPciSegments))
	}

	if len(p.IommuSegments) > 0 {
		segs := make([]int, 0, len(p.IommuSegments))
		for _, seg := range p.IommuSegments {
			segs = append(segs, int(seg))
		}
		args = append(args, fmt.Sprintf("iommu_segments=[%s]", util.ConsecutiveRanges(segs).String()))
	}

	if len(p.SerialNumber) > 0 {
		args = append(args, fmt.Sprintf("serial_number=%s", p.SerialNumber))
	}

	if p.UUID != uuid.Nil {
		args = append(args, fmt.Sprintf("uuid=%s", p.UUID))
	}

	if len(p.OemStrings) > 0 {
		args = append(args, fmt.Sprintf("oem_strings=%s", strings.Join(p.OemStrings, " ")))
	}

	if len(args) > 0 {
		return append([]string(nil), "--platform", strings.Join(args, ","))
	} else {
		return nil
	}
}

func (m MemoryZoneConfig) CommandArgs() []string {
	var args []string

	if m.Size > 0 {
		args = append(args, fmt.Sprintf("size=%d", m.Size))
	}

	if len(m.File) > 0 {
		args = append(args, "file=%s", m.File)
	}

	if m.Shared {
		args = append(args, "shared=on")
	}

	if m.Hugepages {
		args = append(args, "hugepages=on")
	}

	if m.HugepageSize > 0 {
		args = append(args, fmt.Sprintf("hugepage_size=%d", m.HugepageSize))
	}

	if m.HostNumaNode > 0 {
		args = append(args, fmt.Sprintf("host_numa_node=%d", m.HostNumaNode))
	}

	if len(m.ID) > 0 {
		args = append(args, fmt.Sprintf("id=%s", m.ID))
	}

	if m.HotplugSize > 0 {
		args = append(args, fmt.Sprintf("hotplug_size=%d", m.HotplugSize))
	}

	if m.HotpluggedSize > 0 {
		args = append(args, fmt.Sprintf("hotplugged_size=%d", m.HotpluggedSize))
	}

	if m.Prefault {
		args = append(args, "prefault=on")
	}

	if len(args) > 0 {
		return append([]string(nil), "--memory-zone", strings.Join(args, ","))
	} else {
		return nil
	}
}

func (m MemoryConfig) CommandArgs() []string {
	var args []string

	if m.Size > 0 {
		args = append(args, fmt.Sprintf("size=%d", m.Size))
	}

	if m.Mergeable {
		args = append(args, "mergeable=on")
	}

	if m.Shared {
		args = append(args, "shared=on")
	}

	if m.Hugepages {
		args = append(args, "hugepages=on")
	}

	if m.HugepageSize > 0 {
		args = append(args, fmt.Sprintf("hugepage_size=%d", m.HugepageSize))
	}

	if len(m.HotplugMethod) > 0 {
		args = append(args, fmt.Sprintf("hotplug_method=%s", m.HotplugMethod))
	}

	if m.HotplugSize > 0 {
		args = append(args, fmt.Sprintf("hotplug_size=%d", m.HotplugSize))
	}

	if m.HotpluggedSize > 0 {
		args = append(args, fmt.Sprintf("hotplugged_size=%d", m.HotpluggedSize))
	}

	if m.Prefault {
		args = append(args, "prefault=on")
	}

	if m.Thp {
		args = append(args, "thp=on")
	}

	flags := []string(nil)
	if len(args) > 0 {
		flags = append(flags, "--memory", strings.Join(args, ","))
	}

	for _, cfg := range m.Zones {
		flags = append(flags, cfg.CommandArgs()...)
	}

	return flags
}

func (p PayloadConfig) CommandArgs() []string {
	var flags []string

	if len(p.Firmware) > 0 {
		flags = append(flags, "--firmware", p.Firmware)
	}

	if len(p.Kernel) > 0 {
		flags = append(flags, "--kernel", p.Kernel)
	}

	if len(p.Cmdline) > 0 {
		flags = append(flags, "--cmdline", p.Cmdline)
	}

	if len(p.Initramfs) > 0 {
		flags = append(flags, "--initramfs", p.Initramfs)
	}

	return flags
}

func (d DiskConfig) CommandArgs() []string {
	var args []string

	if len(d.Path) > 0 {
		args = append(args, fmt.Sprintf("path=%s", d.Path))
	}

	if d.Readonly {
		args = append(args, "readonly=on")
	}

	if d.Direct {
		args = append(args, "direct=on")
	}

	if d.Iommu {
		args = append(args, "iommu=on")
	}

	if d.NumQueues > 0 {
		args = append(args, fmt.Sprintf("num_queues=%d", d.NumQueues))
	}

	if d.QueueSize > 0 {
		args = append(args, fmt.Sprintf("queue_size=%d", d.QueueSize))
	}

	if d.VhostUser {
		args = append(args, "vhost_user=on")
	}

	if len(d.VhostSocket) > 0 {
		args = append(args, fmt.Sprintf("socket=%s", d.VhostSocket))
	}

	if d.RateLimiterConfig != nil {
		config := d.RateLimiterConfig.String()
		if len(config) > 0 {
			args = append(args, config)
		}
	}

	if len(d.ID) > 0 {
		args = append(args, fmt.Sprintf("id=%s", d.ID))
	}

	if d.PciSegment > 0 {
		args = append(args, fmt.Sprintf("pci_segment=%d", d.PciSegment))
	}

	if len(args) > 0 {
		return append([]string(nil), "--disk", strings.Join(args, ","))
	} else {
		return nil
	}
}

func (n NetConfig) CommandArgs() []string {
	var args []string

	if len(n.Tap) > 0 {
		args = append(args, fmt.Sprintf("tap=%s", n.Tap))
	}

	if len(n.IP) > 0 && !n.IP.IsUnspecified() {
		args = append(args, fmt.Sprintf("ip=%s", n.IP))
	}

	if len(n.Mask) > 0 && !n.Mask.IsUnspecified() {
		args = append(args, fmt.Sprintf("mask=%s", n.Mask))
	}

	if len(n.Mac) > 0 {
		args = append(args, fmt.Sprintf("mac=%s", n.Mac))
	}

	// TODO fd? offload_tso, offload_ufo, offload_csum

	if n.Iommu {
		args = append(args, "iommu=on")
	}

	if n.NumQueues > 0 {
		args = append(args, fmt.Sprintf("num_queues=%d", n.NumQueues))
	}

	if n.QueueSize > 0 {
		args = append(args, fmt.Sprintf("queue_size=%d", n.QueueSize))
	}

	if len(n.ID) > 0 {
		args = append(args, fmt.Sprintf("id=%s", n.ID))
	}

	if n.VhostUser {
		args = append(args, "vhost_user=on")
	}

	if len(n.VhostSocket) > 0 {
		args = append(args, fmt.Sprintf("socket=%s", n.VhostSocket))
	}

	if len(n.VhostMode) > 0 {
		args = append(args, fmt.Sprintf("vhost_mode=%s", n.VhostMode))
	}

	if n.RateLimiterConfig != nil {
		config := n.RateLimiterConfig.String()
		if len(config) > 0 {
			args = append(args, config)
		}
	}

	if n.PciSegment > 0 {
		args = append(args, fmt.Sprintf("pci_segment=%d", n.PciSegment))
	}

	if len(args) > 0 {
		return append([]string(nil), "--net", strings.Join(args, ","))
	} else {
		return nil
	}
}

func (r RngConfig) CommandArgs() []string {

	var args []string

	if len(r.Src) > 0 {
		args = append(args, fmt.Sprintf("src=%s", r.Src))
	}

	if r.Iommu {
		args = append(args, "iommu=on")
	}

	if len(args) > 0 {
		return append([]string(nil), "--rng", strings.Join(args, ","))
	} else {
		return nil
	}
}

func (b BalloonConfig) CommandArgs() []string {

	var args []string

	if b.Size > 0 {
		args = append(args, fmt.Sprintf("size=%d", b.Size))
	}

	if b.DeflateOnOom {
		args = append(args, "deflate_on_oom=on")
	}

	if b.FreePageReporting {
		args = append(args, "free_page_reporting=on")
	}

	if len(args) > 0 {
		return append([]string(nil), "--balloon", strings.Join(args, ","))
	} else {
		return nil
	}
}

func (f FsConfig) CommandArgs() []string {
	var args []string

	if len(f.Tag) > 0 {
		args = append(args, fmt.Sprintf("tag=%s", f.Tag))
	}

	if len(f.Socket) > 0 {
		args = append(args, fmt.Sprintf("socket=%s", f.Socket))
	}

	if f.NumQueues > 0 {
		args = append(args, fmt.Sprintf("num_queues=%d", f.NumQueues))
	}

	if f.QueueSize > 0 {
		args = append(args, fmt.Sprintf("queue_size=%d", f.QueueSize))
	}

	if len(f.ID) > 0 {
		args = append(args, fmt.Sprintf("id=%s", f.ID))
	}

	if f.PciSegment > 0 {
		args = append(args, fmt.Sprintf("pci_segment=%d", f.PciSegment))
	}

	if len(args) > 0 {
		return append([]string(nil), "--fs", strings.Join(args, ","))
	} else {
		return nil
	}
}

func (p PmemConfig) CommandArgs() []string {
	var args []string

	if len(p.File) > 0 {
		args = append(args, fmt.Sprintf("file=%s", p.File))
	}

	if p.Size > 0 {
		args = append(args, fmt.Sprintf("size=%d", p.Size))
	}

	if p.Iommu {
		args = append(args, "iommu=on")
	}

	if p.DiscardWrites {
		args = append(args, "discard_writes=on")
	}

	if len(p.ID) > 0 {
		args = append(args, fmt.Sprintf("id=%s", p.ID))
	}

	if p.PciSegment > 0 {
		args = append(args, fmt.Sprintf("pci_segment=%d", p.PciSegment))
	}

	if len(args) > 0 {
		return append([]string(nil), "--pmem", strings.Join(args, ","))
	} else {
		return nil
	}
}

func (c SerialConfig) CommandArgs() []string {
	var args []string

	switch c.Mode {
	case ConsoleModeOff:
		args = append(args, "off")
	case ConsoleModePty:
		args = append(args, "pty")
	case ConsoleModeTty:
		args = append(args, "tty")
	case ConsoleModeNull:
		args = append(args, "null")
	case ConsoleModeFile:
		if len(c.File) > 0 {
			args = append(args, fmt.Sprintf("file=%s", c.File))
		}
	}

	if len(args) > 0 {
		return append([]string(nil), "--serial", strings.Join(args, ","))
	} else {
		return nil
	}
}

func (c ConsoleConfig) CommandArgs() []string {
	var args []string

	switch c.Mode {
	case ConsoleModeOff:
		args = append(args, "off")
	case ConsoleModePty:
		args = append(args, "pty")
	case ConsoleModeTty:
		args = append(args, "tty")
	case ConsoleModeNull:
		args = append(args, "null")
	case ConsoleModeFile:
		if len(c.File) > 0 {
			args = append(args, fmt.Sprintf("file=%s", c.File))
		}
	}

	if c.Iommu {
		args = append(args, "iommu=on")
	}

	if len(args) > 0 {
		return append([]string(nil), "--console", strings.Join(args, ","))
	} else {
		return nil
	}
}

func (d DeviceConfig) CommandArgs() []string {
	var args []string

	if len(d.Path) > 0 {
		args = append(args, fmt.Sprintf("path=%s", d.Path))
	}

	if d.Iommu {
		args = append(args, "iommu=on")
	}

	if len(d.ID) > 0 {
		args = append(args, fmt.Sprintf("id=%s", d.ID))
	}

	if d.PciSegment > 0 {
		args = append(args, fmt.Sprintf("pci_segment=%d", d.PciSegment))
	}

	if len(args) > 0 {
		return append([]string(nil), "--device", strings.Join(args, ","))
	} else {
		return nil
	}
}

func (v VdpaConfig) CommandArgs() []string {
	var args []string

	if len(v.Path) > 0 {
		args = append(args, fmt.Sprintf("path=%s", v.Path))
	}

	if v.NumQueues > 0 {
		args = append(args, fmt.Sprintf("num_queues=%d", v.NumQueues))
	}

	if v.Iommu {
		args = append(args, "iommu=on")
	}

	if len(v.ID) > 0 {
		args = append(args, fmt.Sprintf("id=%s", v.ID))
	}

	if v.PciSegment > 0 {
		args = append(args, fmt.Sprintf("pci_segment=%d", v.PciSegment))
	}

	if len(args) > 0 {
		return append([]string(nil), "--vdpa", strings.Join(args, ","))
	} else {
		return nil
	}
}

func (v VsockConfig) CommandArgs() []string {
	var args []string

	if v.CID > 0 {
		args = append(args, fmt.Sprintf("cid=%d", v.CID))
	}

	if len(v.Socket) > 0 {
		args = append(args, fmt.Sprintf("socket=%s", v.Socket))
	}

	if v.Iommu {
		args = append(args, "iommu=on")
	}

	if len(v.ID) > 0 {
		args = append(args, fmt.Sprintf("id=%s", v.ID))
	}

	if v.PciSegment > 0 {
		args = append(args, fmt.Sprintf("pci_segment=%d", v.PciSegment))
	}

	if len(args) > 0 {
		return append([]string(nil), "--vsock", strings.Join(args, ","))
	} else {
		return nil
	}
}

func (n NumaConfig) CommandArgs() []string {
	var args []string

	if n.GuestNumaID > 0 {
		args = append(args, fmt.Sprintf("guest_numa_id=%d", n.GuestNumaID))
	}

	if len(n.Cpus) > 0 {
		cpuIds := make([]int, 0, len(n.Cpus))
		for _, id := range n.Cpus {
			cpuIds = append(cpuIds, int(id))
		}
		args = append(args, fmt.Sprintf("cpus=[%s]", util.ConsecutiveRanges(cpuIds).String()))
	}
	if len(n.Distances) > 0 {
		dist := make([]string, 0, len(n.Distances))
		for _, cfg := range n.Distances {
			dist = append(dist, cfg.String())
		}
		args = append(args, fmt.Sprintf("distances=[%s]", strings.Join(dist, ",")))
	}

	if len(n.MemoryZones) > 0 {
		args = append(args, fmt.Sprintf("memory_zones=[%s]", strings.Join(n.MemoryZones, ",")))
	}

	if len(n.SgxEpcSections) > 0 {
		args = append(args, fmt.Sprintf("sgx_epc_sections=[%s]", strings.Join(n.SgxEpcSections, ",")))
	}

	if len(args) > 0 {
		return append([]string(nil), "--numa", strings.Join(args, ","))
	} else {
		return nil
	}
}

func (t TpmConfig) CommandArgs() []string {
	var args []string
	if len(t.Socket) > 0 {
		args = append(args, fmt.Sprintf("socket=%s", t.Socket))
	}

	if len(args) > 0 {
		return append([]string(nil), "--tpm", strings.Join(args, ","))
	} else {
		return nil
	}
}

func (t SgxEpcConfig) CommandArgs() []string {
	var args []string

	if len(t.ID) > 0 {
		args = append(args, fmt.Sprintf("id=%s", t.ID))
	}

	if t.Size > 0 {
		args = append(args, fmt.Sprintf("size=%d", t.Size))
	}

	if t.Prefault {
		args = append(args, "prefault=on")
	}

	if len(args) > 0 {
		return append([]string(nil), "--sgx-epc", strings.Join(args, ","))
	} else {
		return nil
	}
}

func (c VmConfig) CommandArgs() []string {
	var args []string

	args = append(args, c.Payload.CommandArgs()...)

	if c.Cpus != nil {
		e := c.Cpus
		args = append(args, e.CommandArgs()...)
	}

	if c.Platform != nil {
		e := c.Platform
		args = append(args, e.CommandArgs()...)
	}

	if c.Memory != nil {
		e := c.Memory
		args = append(args, e.CommandArgs()...)

	}

	for _, e := range c.Disks {
		args = append(args, e.CommandArgs()...)

	}

	for _, e := range c.Net {
		args = append(args, e.CommandArgs()...)

	}

	if c.Rng != nil {
		e := c.Rng
		args = append(args, e.CommandArgs()...)

	}

	if c.Balloon != nil {
		e := c.Balloon
		args = append(args, e.CommandArgs()...)

	}

	for _, e := range c.Fs {
		args = append(args, e.CommandArgs()...)

	}

	for _, e := range c.Pmem {
		args = append(args, e.CommandArgs()...)

	}

	if c.Serial != nil {
		e := c.Serial
		args = append(args, e.CommandArgs()...)

	}

	if c.Console != nil {
		e := c.Console
		args = append(args, e.CommandArgs()...)

	}

	for _, e := range c.Devices {
		args = append(args, e.CommandArgs()...)

	}

	for _, e := range c.Vdpa {
		args = append(args, e.CommandArgs()...)

	}

	if c.Vsock != nil {
		e := c.Vsock
		args = append(args, e.CommandArgs()...)

	}

	for _, e := range c.Numa {
		args = append(args, e.CommandArgs()...)

	}

	if c.Tpm != nil {
		e := c.Tpm
		args = append(args, e.CommandArgs()...)

	}

	for _, e := range c.SgxEpc {
		args = append(args, e.CommandArgs()...)

	}

	if c.Watchdog {
		args = append(args, "--watchdog")
	}

	if c.Pvpanic {
		args = append(args, "--pvpanic")
	}
	return args
}

func joinArgs(args []string) string {
	builder := strings.Builder{}
	for i, arg := range args {
		switch i {
		case 0:
			//do nothing
		default:
			if strings.HasPrefix(arg, "--") {
				builder.WriteRune('\n')
			} else {
				builder.WriteRune(' ')
			}
		}
		builder.WriteString(arg)
	}

	return builder.String()
}

/*
cloud-hypervisor
--api-socket path=/srv/vmm/nodes/test/run/monitor.sock
*/
func DefaultVmConfig() VmConfig {
	return VmConfig{
		Payload: PayloadConfig{
			Firmware: "",
			Kernel:   "/srv/vmm/images/test/vmlinuz",
			Cmdline: strings.Join([]string{
				"console=hvc0",
				"cpuidle.governor=haltpoll",
				"clocksource=kvm-clock",
				"base=UUID=3a42a0c0-dfd2-40b2-b9eb-861f2610a5c1",
				"systemd.machine_id=a6a7a918188645d5adb5aaabdca22f16",
				"net.ifnames=0",
				"verbose",
				"loglevel=7",
				"reboot=t",
			}, " "),
			Initramfs: "/srv/vmm/images/test/initramfs.img",
		},

		Platform: &PlatformConfig{
			SerialNumber: "a6a7a918188645d5adb5aaabdca22f16",
			UUID:         uuid.MustParse("a6a7a918-1886-45d5-adb5-aaabdca22f16"),
			OemStrings:   []string{"amuzes-test"},
		},
		Disks: []DiskConfig{
			{
				Path:      "/srv/vmm/images/test/root.img",
				Readonly:  true,
				Direct:    true,
				NumQueues: 2,
				QueueSize: 128,
			},
			{
				Path:      "/srv/vmm/nodes/test/data.img",
				Direct:    true,
				NumQueues: 2,
				QueueSize: 128,
			},
		},
		Cpus: &CpusConfig{BootVcpus: 2, MaxVcpus: 2},
		Memory: &MemoryConfig{
			Size:      2 * 1024 * 1024 * 1024,
			Mergeable: true,
			Shared:    true,
			Thp:       true,
		},
		Console: &ConsoleConfig{Mode: ConsoleModePty},
		Serial:  &SerialConfig{Mode: ConsoleModeOff},
		Net: []NetConfig{
			{
				Tap:       "vmtap-tst",
				NumQueues: 2,
				QueueSize: 128,
			},
		},
		Rng:      &RngConfig{Src: "/dev/urandom"},
		Watchdog: true,
		Pvpanic:  true,
		Balloon: &BalloonConfig{
			Size:              0,
			FreePageReporting: true,
		},
	}
}
