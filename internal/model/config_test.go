package model

import (
	"bytes"
	"testing"

	"gopkg.in/yaml.v3"

	"amuz.es/src/spi-ca/cvmm/internal/util"
	"github.com/google/uuid"
)

func TestConfig_MarshalYAMLMatchesManifestSchema(t *testing.T) {
	cfg := &Config{
		Cpus:  2,
		Mem:   util.MustLoadIECSize("4G"),
		Uuid:  uuid.MustParse("87773d86-0030-4db4-9e90-e5a4314ff11b"),
		Image: "test-image",
		Net: ManifestNetConfig{
			Backend: NetBackendTap,
			MacAddr: util.MustLoadMACAddress("2e:33:5f:11:1b:42"),
			IfName:  "vmtap-01",
		},
		Cmdline: []string{
			"console=hvc0",
			"quiet",
		},
		Disk:      []string{"data.img"},
		Directory: []string{"configuration"},
	}

	got, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}

	want := []byte(`cpus: 2
mem: 4G
uuid: 87773d86-0030-4db4-9e90-e5a4314ff11b
image: test-image
net:
    backend: tap
    mac_addr: 2e:33:5f:11:1b:42
    if_name: vmtap-01
cmdline:
    - console=hvc0
    - quiet
disk:
    - data.img
directory:
    - configuration
`)

	if !bytes.Equal(got, want) {
		t.Fatalf("yaml.Marshal() = %s, want %s", string(got), string(want))
	}
}
