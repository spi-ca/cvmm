package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"amuz.es/src/spi-ca/cvmm/internal/entry"
	"amuz.es/src/spi-ca/cvmm/internal/hvm"
	"amuz.es/src/spi-ca/cvmm/internal/util"

	flags "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	name = "cvmm"
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
	flags.String("pid-filename", "cvmm.pid", "specify pid filename")
	flags.String("cloudhypervisor-api-filename", "cloudhypervisor.sock", "specify api socket filename")
	flags.String("cloudhypervisor-pid-filename", "cloudhypervisor.pid", "specify cloudhypervisor pid filename")
	flags.String("virtiofs-socket-filename-template", "virtiofs.sock", "specify virtiofs socket filename")
	flags.String("virtiofs-pid-filename-template", "virtiofs.pid", "specify virtiofs pid filename")
	flags.String("volatile-directory", "run", "specify volatile directory name")

	flags.String("runas", "", "run as user while executing hypervisor. user:group")

	flags.Bool("console", false, "redirect console to stdin/stdout")

	flags.String("virtiofsd-path", "/usr/lib/virtiofsd", "specify virtiofsd binary path")
	flags.String("cloudhypervisor-path", "/usr/bin/cloud-hypervisor", "specify cloud-hypervisor binary path")

	flags.Parse()
	viper.SetEnvKeyReplacer(envNameReplacer)
	viper.AutomaticEnv()
	_ = viper.BindFlagValues(util.PFlagViperReplacer{FlagSet: flags.CommandLine, Replacer: flagNameReplacer})
}

func main() {

	consumedArgs := 0
	if flags.NArg() == 0 {
		usage(fmt.Sprintf("not enough arguments"))
	}

	action := flags.Arg(0)
	consumedArgs++
	//
	switch action {
	case "start":
		var (
			nodeName string
		)
		switch flags.NArg() {
		case consumedArgs + 1:
			nodeName = flags.Arg(consumedArgs + 0)
			consumedArgs += 1
		default:
			usage("required arguments missing")
		}
		entry.Start(name, nodeName)
	case "shutdown":
		var (
			nodeName string
		)
		switch flags.NArg() {
		case consumedArgs + 1:
			nodeName = flags.Arg(consumedArgs + 0)
			consumedArgs += 1
		default:
			usage("required arguments missing")
		}
		entry.Shutdown(name, nodeName)
	case "console":
		var (
			nodeName string
		)
		switch flags.NArg() {
		case consumedArgs + 1:
			nodeName = flags.Arg(consumedArgs + 0)
			consumedArgs += 1
		default:
			usage("required argument missing")
		}

		entry.Console(name, nodeName)
	case "console-file":
		var (
			rawPtyId string
		)
		switch flags.NArg() {
		case consumedArgs + 1:
			rawPtyId = flags.Arg(consumedArgs + 0)
			consumedArgs += 1
		default:
			usage("required argument missing")
		}

		ptyId, err := strconv.Atoi(rawPtyId)
		if err != nil {
			usage(fmt.Sprintf("invalid pty_id %s, %s", rawPtyId, err))
		}

		entry.ConsoleFile(name, ptyId)
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
	{{.name}} start NODE_NAME
	{{.name}} shutdown NODE_NAME
	{{.name}} console NODE_NAME
	{{.name}} console-file PTY_ID
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
