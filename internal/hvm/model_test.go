package hvm

import (
	"encoding/json"
	"gopkg.in/yaml.v3"
	"testing"
)

func TestVMInfo_Serialize(t *testing.T) {
	var response = []byte(`{
  "config": {
    "cpus": {
      "boot_vcpus": 2,
      "max_vcpus": 2,
      "topology": null,
      "kvm_hyperv": false,
      "max_phys_bits": 46,
      "affinity": null,
      "features": {
        "amx": false
      }
    },
    "memory": {
      "size": 4294967296,
      "mergeable": true,
      "hotplug_method": "Acpi",
      "hotplug_size": null,
      "hotplugged_size": null,
      "shared": true,
      "hugepages": false,
      "hugepage_size": null,
      "prefault": false,
      "zones": null,
      "thp": true
    },
    "payload": {
      "firmware": null,
      "kernel": "/srv/vmm/images/kube-master/vmlinuz",
      "cmdline": "console=hvc0 cpuidle.governor=haltpoll clocksource=kvm-clock base=UUID=3a42a0c0-dfd2-40b2-b9eb-861f2610a5c1 systemd.machine_id=87773d8614794db49e90e5a4314ff11b net.ifnames=0 quiet loglevel=3",
      "initramfs": "/srv/vmm/images/kube-master/initramfs.img"
    },
    "disks": [
      {
        "path": "/srv/vmm/images/kube-master/root.img",
        "readonly": true,
        "direct": true,
        "iommu": false,
        "num_queues": 2,
        "queue_size": 128,
        "vhost_user": false,
        "vhost_socket": null,
        "rate_limiter_config": null,
        "id": "_disk0",
        "disable_io_uring": false,
        "pci_segment": 0,
        "serial": null
      },
      {
        "path": "/srv/vmm/nodes/kube-master-01/data.img",
        "readonly": false,
        "direct": true,
        "iommu": false,
        "num_queues": 2,
        "queue_size": 128,
        "vhost_user": false,
        "vhost_socket": null,
        "rate_limiter_config": null,
        "id": "_disk1",
        "disable_io_uring": false,
        "pci_segment": 0,
        "serial": null
      }
    ],
    "net": [
      {
        "tap": "vmtap-km01",
        "ip": "192.168.249.1",
        "mask": "255.255.255.0",
        "mac": "2e:f4:5f:11:1b:56",
        "host_mac": "2e:a7:7e:1b:cc:03",
        "mtu": null,
        "iommu": false,
        "num_queues": 2,
        "queue_size": 128,
        "vhost_user": false,
        "vhost_socket": null,
        "vhost_mode": "Client",
        "id": "_net2",
        "fds": null,
        "rate_limiter_config": null,
        "pci_segment": 0,
        "offload_tso": true,
        "offload_ufo": true,
        "offload_csum": true
      }
    ],
    "rng": {
      "src": "/dev/urandom",
      "iommu": false
    },
    "balloon": null,
    "fs": [
      {
        "tag": "configuration",
        "socket": "/srv/vmm/nodes/kube-master-01/run/virtiofsd.sock",
        "num_queues": 1,
        "queue_size": 1024,
        "id": "_fs3",
        "pci_segment": 0
      }
    ],
    "pmem": null,
    "serial": {
      "file": null,
      "mode": "Off",
      "iommu": false
    },
    "console": {
      "file": "/dev/pts/23",
      "mode": "Pty",
      "iommu": false
    },
    "devices": null,
    "user_devices": null,
    "vdpa": null,
    "vsock": null,
    "pvpanic": true,
    "iommu": false,
    "sgx_epc": null,
    "numa": null,
    "watchdog": true,
    "platform": {
      "num_pci_segments": 1,
      "iommu_segments": null,
      "serial_number": "87773d8614794db49e90e5a4314ff11b",
      "uuid": "87773d86-1479-4db4-9e90-e5a4314ff11b",
      "oem_strings": [
        "amuzes-kube-master-01"
      ]
    },
    "tpm": null
  },
  "state": "Running",
  "memory_actual_size": 4294967296,
  "device_tree": {
    "_disk1": {
      "id": "_disk1",
      "resources": [],
      "parent": "_virtio-pci-_disk1",
      "children": [],
      "pci_bdf": null
    },
    "__ioapic": {
      "id": "__ioapic",
      "resources": [],
      "parent": null,
      "children": [],
      "pci_bdf": null
    },
    "_fs3": {
      "id": "_fs3",
      "resources": [],
      "parent": "_virtio-pci-_fs3",
      "children": [],
      "pci_bdf": null
    },
    "_virtio-pci-__watchdog": {
      "id": "_virtio-pci-__watchdog",
      "resources": [
        {
          "PciBar": {
            "index": 0,
            "base": 70365520330752,
            "size": 524288,
            "type_": "Mmio64",
            "prefetchable": false
          }
        }
      ],
      "parent": null,
      "children": [
        "__watchdog"
      ],
      "pci_bdf": "0000:00:07.0"
    },
    "_disk0": {
      "id": "_disk0",
      "resources": [],
      "parent": "_virtio-pci-_disk0",
      "children": [],
      "pci_bdf": null
    },
    "_net2": {
      "id": "_net2",
      "resources": [],
      "parent": "_virtio-pci-_net2",
      "children": [],
      "pci_bdf": null
    },
    "__watchdog": {
      "id": "__watchdog",
      "resources": [],
      "parent": "_virtio-pci-__watchdog",
      "children": [],
      "pci_bdf": null
    },
    "_virtio-pci-__console": {
      "id": "_virtio-pci-__console",
      "resources": [
        {
          "PciBar": {
            "index": 0,
            "base": 70365522427904,
            "size": 524288,
            "type_": "Mmio64",
            "prefetchable": false
          }
        }
      ],
      "parent": null,
      "children": [
        "__console"
      ],
      "pci_bdf": "0000:00:01.0"
    },
    "_virtio-pci-_fs3": {
      "id": "_virtio-pci-_fs3",
      "resources": [
        {
          "PciBar": {
            "index": 0,
            "base": 70365520855040,
            "size": 524288,
            "type_": "Mmio64",
            "prefetchable": false
          }
        }
      ],
      "parent": null,
      "children": [
        "_fs3"
      ],
      "pci_bdf": "0000:00:06.0"
    },
    "_virtio-pci-_disk1": {
      "id": "_virtio-pci-_disk1",
      "resources": [
        {
          "PciBar": {
            "index": 0,
            "base": 3891265536,
            "size": 524288,
            "type_": "Mmio32",
            "prefetchable": false
          }
        }
      ],
      "parent": null,
      "children": [
        "_disk1"
      ],
      "pci_bdf": "0000:00:03.0"
    },
    "__pvpanic": {
      "id": "__pvpanic",
      "resources": [
        {
          "PciBar": {
            "index": 0,
            "base": 3891261440,
            "size": 2,
            "type_": "Mmio32",
            "prefetchable": false
          }
        }
      ],
      "parent": null,
      "children": [],
      "pci_bdf": "0000:00:08.0"
    },
    "__console": {
      "id": "__console",
      "resources": [],
      "parent": "_virtio-pci-__console",
      "children": [],
      "pci_bdf": null
    },
    "__rng": {
      "id": "__rng",
      "resources": [],
      "parent": "_virtio-pci-__rng",
      "children": [],
      "pci_bdf": null
    },
    "_virtio-pci-_disk0": {
      "id": "_virtio-pci-_disk0",
      "resources": [
        {
          "PciBar": {
            "index": 0,
            "base": 3891789824,
            "size": 524288,
            "type_": "Mmio32",
            "prefetchable": false
          }
        }
      ],
      "parent": null,
      "children": [
        "_disk0"
      ],
      "pci_bdf": "0000:00:02.0"
    },
    "_virtio-pci-__rng": {
      "id": "_virtio-pci-__rng",
      "resources": [
        {
          "PciBar": {
            "index": 0,
            "base": 70365521379328,
            "size": 524288,
            "type_": "Mmio64",
            "prefetchable": false
          }
        }
      ],
      "parent": null,
      "children": [
        "__rng"
      ],
      "pci_bdf": "0000:00:05.0"
    },
    "_virtio-pci-_net2": {
      "id": "_virtio-pci-_net2",
      "resources": [
        {
          "PciBar": {
            "index": 0,
            "base": 70365521903616,
            "size": 524288,
            "type_": "Mmio64",
            "prefetchable": false
          }
        }
      ],
      "parent": null,
      "children": [
        "_net2"
      ],
      "pci_bdf": "0000:00:04.0"
    }
  }
}`)
	i := VmInfo{}
	err := json.Unmarshal(response, &i)
	if err != nil {
		panic(err)
	}

	e, err := json.Marshal(&i)
	if err != nil {
		panic(err)
	}
	t.Logf("json() = %v", i)
	t.Logf("json() = %s", string(e))

	e2, err := yaml.Marshal(&i)
	if err != nil {
		panic(err)
	}
	t.Logf("yaml() =\n %s", string(e2))

	//t.Logf("json() = %v, want %v", string(got), string(tt.want))

	//	type fields struct {
	//		Name       string
	//		Cpus       int
	//		Mem        util.IECSize
	//		Uuid       uuid.UUID
	//		RootfsUuid uuid.UUID
	//		Image      string
	//		NetMacAddr util.MACAddress
	//		NetIfName  string
	//		Cmdline    []string
	//		Disk       []string
	//		Directory  []string
	//	}
	//	tests := []struct {
	//		name   string
	//		fields fields
	//		want   []byte
	//	}{
	//		{
	//			name: "yaml test",
	//			fields: fields{
	//				Name:       "test-mock",
	//				Cpus:       2,
	//				Mem:        util.MustLoadIECSize("4G"),
	//				Uuid:       uuid.MustParse("87773d86-0030-4db4-9e90-e5a4314ff11b"),
	//				RootfsUuid: uuid.MustParse("3a42a0c0-dfd2-40b2-b9eb-86842610a5c1"),
	//				Image:      "test-image",
	//				NetMacAddr: util.MustLoadMACAddress("2e:33:5f:11:1b:42"),
	//				NetIfName:  "vmtap-01",
	//				Cmdline: []string{
	//					"console=hvc0",
	//					"cpuidle.governor=haltpoll",
	//					"clocksource=kvm-clock",
	//					"net.ifnames=0",
	//					"quiet",
	//					"loglevel=3",
	//				},
	//				Disk:      []string{"data.img"},
	//				Directory: []string{"configuration"},
	//			},
	//			want: []byte(`name: test-mock
	//cpus: 2
	//mem: 4G
	//uuid: 87773d86-0030-4db4-9e90-e5a4314ff11b
	//rootfs_uuid: 3a42a0c0-dfd2-40b2-b9eb-86842610a5c1
	//image: test-image
	//net_mac_addr: 2e:33:5f:11:1b:42
	//net_if_name: vmtap-01
	//cmdline:
	//    - console=hvc0
	//    - cpuidle.governor=haltpoll
	//    - clocksource=kvm-clock
	//    - net.ifnames=0
	//    - quiet
	//    - loglevel=3
	//disk:
	//    - data.img
	//directory:
	//    - configuration
	//`),
	//		},
	//	}
	//	for _, tt := range tests {
	//		t.Run(tt.name, func(t *testing.T) {
	//			i := &Hypervisor{
	//				Name:       tt.fields.Name,
	//				Cpus:       tt.fields.Cpus,
	//				Mem:        tt.fields.Mem,
	//				Uuid:       tt.fields.Uuid,
	//				RootfsUuid: tt.fields.RootfsUuid,
	//				Image:      tt.fields.Image,
	//				NetMacAddr: tt.fields.NetMacAddr,
	//				NetIfName:  tt.fields.NetIfName,
	//				Cmdline:    tt.fields.Cmdline,
	//				Disk:       tt.fields.Disk,
	//				Directory:  tt.fields.Directory,
	//			}
	//			got, err := yaml.Marshal(i)
	//			if err != nil {
	//				panic(err)
	//			}
	//
	//			if bytes.Compare(got, tt.want) != 0 {
	//				t.Errorf("yaml() = %v, want %v", string(got), string(tt.want))
	//			}
	//		})
	//	}
}
