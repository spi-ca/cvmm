package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"amuz.es/src/spi-ca/cvmm/internal/util"
	"github.com/google/uuid"
)

var benchmarkLoadConfigSink *Config

func BenchmarkLoadConfig(b *testing.B) {
	for _, size := range []struct {
		name   string
		disks  int
		shares int
	}{
		{name: "small", disks: 1, shares: 1},
		{name: "medium", disks: 4, shares: 4},
		{name: "large", disks: 8, shares: 8},
	} {
		b.Run(size.name, func(b *testing.B) {
			manifestPath := filepath.Join(b.TempDir(), "config.yaml")
			if err := os.WriteFile(manifestPath, []byte(benchmarkManifest(size.disks, size.shares)), 0o644); err != nil {
				b.Fatal(err)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				cfg, err := LoadConfig(manifestPath)
				if err != nil {
					b.Fatal(err)
				}
				benchmarkLoadConfigSink = cfg
			}
		})
	}
}

func BenchmarkConfigVMConfigAssembly(b *testing.B) {
	for _, size := range []struct {
		name   string
		disks  int
		shares int
	}{
		{name: "small", disks: 1, shares: 1},
		{name: "medium", disks: 4, shares: 4},
		{name: "large", disks: 8, shares: 8},
	} {
		b.Run(size.name, func(b *testing.B) {
			cfg := benchmarkConfig(size.disks, size.shares)
			for i := 0; i < b.N; i++ {
				_ = cfg.VMConfig(
					"bench-node",
					"/srv/vmm/images/bench/vmlinuz",
					"/srv/vmm/images/bench/initramfs.img",
					"/srv/vmm/images/bench/root.img",
					"/srv/vmm/nodes/bench-node",
					"/srv/vmm/nodes/bench-node/run/virtiofs.sock",
					"/srv/vmm/nodes/bench-node/run/passt.sock",
					false,
				)
			}
		})
	}
}

func BenchmarkVirtiofsCommandArgs(b *testing.B) {
	for _, shares := range []int{1, 4, 8, 16} {
		b.Run(fmt.Sprintf("shares_%d", shares), func(b *testing.B) {
			cfg := benchmarkConfig(0, shares)
			virtiofs := cfg.VirtiofsConfig(
				"/srv/vmm/nodes/bench-node",
				"/srv/vmm/nodes/bench-node/run/virtiofs.sock",
				"/srv/vmm/nodes/bench-node/run/virtiofs.pid",
				"hvm",
			)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for idx := range virtiofs {
					_ = virtiofs[idx].CommandArgs()
				}
			}
		})
	}
}

func benchmarkManifest(disks, shares int) string {
	var b strings.Builder
	b.WriteString(`cpus: 4
mem: 4G
uuid: 87773d86-0030-4db4-9e90-e5a4314ff11b
image: bench-image
cmdline:
  - quiet
  - panic=-1
`)
	if disks > 0 {
		b.WriteString("disk:\n")
		for idx := 0; idx < disks; idx++ {
			fmt.Fprintf(&b, "  - data/disk-%02d.img\n", idx)
		}
	}
	if shares > 0 {
		b.WriteString("directory:\n")
		for idx := 0; idx < shares; idx++ {
			fmt.Fprintf(&b, "  - share/dir-%02d\n", idx)
		}
	}
	return b.String()
}

func benchmarkConfig(disks, shares int) *Config {
	cfg := &Config{
		Cpus:    4,
		Mem:     util.IECSize(4 << 30),
		Uuid:    uuid.MustParse("87773d86-0030-4db4-9e90-e5a4314ff11b"),
		Image:   "bench-image",
		Cmdline: []string{"quiet", "panic=-1"},
	}
	for idx := 0; idx < disks; idx++ {
		cfg.Disk = append(cfg.Disk, filepath.Join("data", fmt.Sprintf("disk-%02d.img", idx)))
	}
	for idx := 0; idx < shares; idx++ {
		cfg.Directory = append(cfg.Directory, filepath.Join("share", fmt.Sprintf("dir-%02d", idx)))
	}
	return cfg
}
