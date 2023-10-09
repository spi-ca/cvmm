package main

import (
	"fmt"
	"os"
	"strings"

	"amuz.es/src/spi-ca/chmgr/internal/entry"
	"amuz.es/src/spi-ca/chmgr/internal/hvm"
	"amuz.es/src/spi-ca/chmgr/internal/util"

	flags "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	name = "chmgr"
)

var (
	flagNameReplacer = strings.NewReplacer("-", ".", "_", ".")
	envNameReplacer  = strings.NewReplacer(".", "_", "-", "_")
)

func init() {
	flags.String("image-root", "/srv/vmm/images", "specify image repository path")

	flags.String("image-kernel-filename", "vmlinuz", "specify image kernel filename")
	flags.String("image-initramfs-filename", "initramfs.img", "specify image initramfs filename")
	flags.String("image-rootfs-filename", "root.img", "specify image rootfs filename")

	flags.String("node-root", "/srv/vmm/nodes", "specify node repository path")
	flags.String("manifest-filename", "config.yaml", "specify node manifest file path")
	flags.String("cloudhypervisor-monitor-filename", "monitor.sock", "specify monitor socket filename")
	flags.String("virtiofs-socket-filename", "virtiofs_{{.directoryName}}.sock", "specify virtiofs socket filename")
	flags.String("volatile-directory", "run", "specify volatile directory name")

	flags.String("virtiofsd-path", "/usr/lib/virtiofsd", "specify virtiofsd binary path")
	flags.String("cloudhypervisor-path", "/usr/bin/cloud-hypervisor", "specify cloud-hypervisor binary path")

	flags.Parse()
	viper.SetEnvKeyReplacer(envNameReplacer)
	viper.AutomaticEnv()
	_ = viper.BindFlagValues(util.PFlagViperReplacer{FlagSet: flags.CommandLine, Replacer: flagNameReplacer})
}

// GOGC=100
// GOMEMLIMIT=32Mib
func main() {

	consumedArgs := 0
	if flags.NArg() == 0 {
		usage(fmt.Sprintf("not enough arguments"))
	}

	action := flags.Arg(0)
	consumedArgs++
	//
	switch action {
	//case "start":
	//	var (
	//		nodeName string
	//	)
	//	switch flags.NArg() {
	//	case consumedArgs + 1:
	//		nodeName = flags.Arg(consumedArgs + 0)
	//		consumedArgs += 1
	//	default:
	//		fmt.Println("required arguments missing")
	//		usage()
	//	}
	//	entry.Starter(nodeName)
	//case "console":
	//	var (
	//		nodeName string
	//	)
	//	switch flags.NArg() {
	//	case consumedArgs + 1:
	//		nodeName = flags.Arg(consumedArgs + 0)
	//		consumedArgs += 1
	//	default:
	//		fmt.Println("required arguments missing")
	//		usage()
	//	}
	//	entry.Starter(nodeName)

	//case "stop":
	case "client":
		var (
			rawClientAction string
			nodeName        string
		)
		switch flags.NArg() {
		case consumedArgs + 2:
			rawClientAction = flags.Arg(consumedArgs + 0)
			nodeName = flags.Arg(consumedArgs + 1)
			consumedArgs += 2
		default:
			usage("required argument missing")
		}

		clientAction, err := hvm.ClientActionNameOf(rawClientAction)
		if err != nil {
			usage(fmt.Sprintf("invalid clientAction %s", rawClientAction))
		}
		entry.Client(name, nodeName, clientAction)
	default:
		usage(fmt.Sprintf("invalid action %s", action))
	}
}

func usage(reason string) {
	if len(reason) > 0 {
		util.ErrLog.Println(reason)
	}
	_, _ = os.Stderr.WriteString(util.F(`usage:
	{{.name}} boot NODE_NAME
	{{.name}} power-off NODE_NAME
	{{.name}} console NODE_NAME
{{- range $val := .clientAction}}
	{{$.name}} client {{$val.String}} NODE_NAME
{{- end}}

`).R(util.FormatArgs{
		"name":         name,
		"clientAction": hvm.ClientActions(),
	}))
	flags.PrintDefaults()
	os.Exit(1)
}
