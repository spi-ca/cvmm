package hvm

import (
	"net"

	"amuz.es/src/spi-ca/chmgr/internal/util"
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
		Serial   *ConsoleConfig  `json:"serial,omitempty" yaml:"serial,omitempty"`
		Console  *ConsoleConfig  `json:"console,omitempty" yaml:"console,omitempty"`
		Devices  []DeviceConfig  `json:"devices,omitempty" yaml:"devices,omitempty"`
		Vdpa     []VdpaConfig    `json:"vdpa,omitempty" yaml:"vdpa,omitempty"`
		Vsock    *VsockConfig    `json:"vsock,omitempty" yaml:"vsock,omitempty"`
		SgxEpc   []SgxEpcConfig  `json:"sgx_epc,omitempty" yaml:"sgx_epc,omitempty"`
		Numa     []NumaConfig    `json:"numa,omitempty" yaml:"numa,omitempty"`
		Iommu    bool            `json:"iommu,omitempty" yaml:"iommu,omitempty"`
		Watchdog bool            `json:"watchdog,omitempty" yaml:"watchdog,omitempty"`
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
		NumPciSegments int        `json:"num_pci_segments,omitempty" yaml:"num_pci_segments,omitempty"`
		IommuSegments  []int16    `json:"iommu_segments,omitempty" yaml:"iommu_segments,omitempty"`
		SerialNumber   string     `json:"serial_number,omitempty" yaml:"serial_number,omitempty"`
		UUID           *uuid.UUID `json:"uuid,omitempty" yaml:"uuid,omitempty"`
		OemStrings     []string   `json:"oem_strings,omitempty" yaml:"oem_strings,omitempty"`
		Tdx            bool       `json:"tdx,omitempty" yaml:"tdx,omitempty"`
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
