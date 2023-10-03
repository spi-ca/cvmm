package args

import (
	"testing"

	"amuz.es/src/spi-ca/chmgr/internal/util"
	"github.com/google/uuid"
)

//func TestHypervisorArgs(t *testing.T) {
//	r := strings.NewReader(`
//	name: test-mock
//cpus: 2
//mem: 4G
//uuid: 87773d86-0030-4db4-9e90-e5a4314ff11b
//rootfs_uuid: 3a42a0c0-dfd2-40b2-b9eb-86842610a5c1
//image: kube-master
//net_mac_addr: 2e:33:5f:11:1b:42
//net_if_name: vmtap-01
//cmdline:
//- console=hvc0
//- cpuidle.governor=haltpoll
//- clocksource=kvm-clock
//- net.ifnames=0
//- quiet
//- loglevel=3
//disk:
//- data.img
//directory:
//- configuration`)
//
//	cfg := &Hypervisor{}
//
//	d := yaml.NewDecoder(r)
//	err := d.Decode(cfg)
//	if err != nil {
//		panic(err)
//	}
//
//	fmt.Printf("v = %v\n", cfg)
//
//	marshalled, err := yaml.Marshal(cfg)
//	if err != nil {
//		panic(err)
//	}
//	fmt.Printf("v = %s\n", string(marshalled))
//
//}

func TestHypervisor_MachineId(t *testing.T) {
	type fields struct {
		Name       string
		Cpus       int
		Mem        util.IECSize
		Uuid       uuid.UUID
		RootfsUuid uuid.UUID
		Image      string
		NetMacAddr util.MACAddress
		NetIfName  string
		Cmdline    []string
		Disk       []string
		Directory  []string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "yaml test",
			fields: fields{
				Name:       "test-mock",
				Cpus:       2,
				Mem:        util.MustLoadIECSize("4G"),
				Uuid:       uuid.MustParse("87773d86-0030-4db4-9e90-e5a4314ff11b"),
				RootfsUuid: uuid.MustParse("3a42a0c0-dfd2-40b2-b9eb-86842610a5c1"),
				Image:      "test-image",
				NetMacAddr: util.MustLoadMACAddress("2e:33:5f:11:1b:42"),
				NetIfName:  "",
				Cmdline:    []string{},
				Disk:       nil,
				Directory:  nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &Hypervisor{
				Name:       tt.fields.Name,
				Cpus:       tt.fields.Cpus,
				Mem:        tt.fields.Mem,
				Uuid:       tt.fields.Uuid,
				RootfsUuid: tt.fields.RootfsUuid,
				Image:      tt.fields.Image,
				NetMacAddr: tt.fields.NetMacAddr,
				NetIfName:  tt.fields.NetIfName,
				Cmdline:    tt.fields.Cmdline,
				Disk:       tt.fields.Disk,
				Directory:  tt.fields.Directory,
			}
			if got := i.MachineId(); got != tt.want {
				t.Errorf("MachineId() = %v, want %v", got, tt.want)
			}
		})
	}
}
