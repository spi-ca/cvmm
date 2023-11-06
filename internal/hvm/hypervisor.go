package hvm

import (
	"amuz.es/src/spi-ca/chmgr/internal/model"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"amuz.es/src/spi-ca/chmgr/internal/util"
	"gopkg.in/yaml.v3"
)

type Hypervisor struct {
	name              string `yaml:"-"`
	imageRoot         string `yaml:"-"`
	nodeHome          string `yaml:"-"`
	volatileDirectory string `yaml:"-"`

	virtiofsSocketTemplate *util.Format
	args                   model.Config

	cli *clientImpl
}

func (i *Hypervisor) load(manifestPath string) error {
	f, err := os.Open(manifestPath)
	if err != nil {
		return err
	}

	defer func() { _ = f.Close() }()

	d := yaml.NewDecoder(f)
	err = d.Decode(i)
	if err != nil {
		return err
	}

	return nil
}

// TODO impl
func (i *Hypervisor) Ping(ctx context.Context) error {
	return nil
}

func (i *Hypervisor) Info(ctx context.Context) (*model.VmInfo, error) { return i.cli.VmInfo(ctx) }
func (i *Hypervisor) Counters(ctx context.Context) (*model.VmCounters, error) {
	return i.cli.VmCounters(ctx)
}

// TODO impl
func (i *Hypervisor) Boot(ctx context.Context) error { return i.cli.VmBoot(ctx) }

// TODO impl
func (i *Hypervisor) Pause(ctx context.Context) error { return i.cli.VmPause(ctx) }

// TODO impl
func (i *Hypervisor) Resume(ctx context.Context) error { return i.cli.VmResume(ctx) }

// TODO impl
func (i *Hypervisor) Shutdown(ctx context.Context) error { return i.cli.VmShutdown(ctx) }

// TODO impl
func (i *Hypervisor) Reboot(ctx context.Context) error { return i.cli.VmReboot(ctx) }

// TODO impl
func (i *Hypervisor) PowerButton(ctx context.Context) error { return i.cli.VmPowerButton(ctx) }

func (i *Hypervisor) Close()            { i.cli.Close() }
func (i *Hypervisor) GetClient() Client { return i.cli }
func (i *Hypervisor) ImageBasePath(rest string) string {
	return filepath.Join(i.imageRoot, i.args.Image, rest)
}

func (i *Hypervisor) NodeBasePath(rest string) string {
	return filepath.Join(i.nodeHome, rest)
}

func (i *Hypervisor) VolatilePath(rest string) string {
	return filepath.Join(i.volatileDirectory, rest)
}

func (i *Hypervisor) VirtiofsSocketPath(name string) string {
	return i.VolatilePath(i.virtiofsSocketTemplate.R(util.FormatArgs{"directoryName": name}))
}

// todo move to VirtiofsConfig
func (i *Hypervisor) VirtiofsArgs() []string {
	var (
		arguments []string
	)

	for _, filename := range i.args.Directory {
		name := filepath.Base(filename)
		cfg := &model.VirtiofsConfig{
			Directory:      i.NodeBasePath(filename),
			SocketPath:     i.VirtiofsSocketPath(name),
			ThreadPoolSize: i.args.Cpus,
		}

		arguments = append(arguments, strings.Join(cfg.CommandArgs(), " "))
	}
	return arguments
}

func (i *Hypervisor) CommandArgs(
	kernelFilename,
	initramfsFilename,
	rootfsFilename,
	monitorFilename,
	virtiofsFilename string,
) []string {
	// TODO hypervisor
	// vmcfg
	_ = i.args.VMConfig(
		i.name,
		i.ImageBasePath(kernelFilename), i.ImageBasePath(initramfsFilename),
		i.ImageBasePath(rootfsFilename), i.NodeBasePath,
		i.VirtiofsSocketPath,
	)
	// virtiofscfg
	_ = i.args.VirtiofsConfig(i.NodeBasePath, i.VirtiofsSocketPath)

	return nil
}

func (i *Hypervisor) OpenConsole(parentCtx context.Context) error {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	info, err := i.cli.VmInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to open console: %w", err)
	}

	ptyPath := info.Config.Console.File
	return util.OpenPty(ctx, os.Stdin, os.Stdout, ptyPath)
}
