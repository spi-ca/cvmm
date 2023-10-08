package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/viper"

	"gopkg.in/yaml.v3"

	"amuz.es/src/spi-ca/chmgr/internal/hvm"
	"amuz.es/src/spi-ca/chmgr/internal/util"
)

func main() {
	var (
		path   = os.Args[1]
		action = os.Args[2]
	)

	util.InfoLog.SetPrefix(fmt.Sprintf("%s[%d]&1>", viper.GetString("log.prefix"), os.Getpid()))
	util.ErrLog.SetPrefix(fmt.Sprintf("%s[%d]&2>", viper.GetString("log.prefix"), os.Getpid()))

	ctx, cancel := context.WithCancel(context.Background())

	exitSignal := make(chan os.Signal, 1)
	signal.Notify(exitSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	defer signal.Ignore(syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	go func() {
		select {
		case <-ctx.Done():
			return
		case sysSignal := <-exitSignal:
			util.ErrLog.Println(sysSignal.String(), " received")
			cancel()
			return
		}
	}()

	var (
		client = hvm.NewClient(path)
		err    error
		resp   any
	)
	switch action {
	case "vmm-ping":
		resp, err = client.VmmPing(ctx)
	case "vmm-shutdown":
		err = client.VmmShutdown(ctx)
	case "vm-info":
		resp, err = client.VmInfo(ctx)
	case "vm-counters":
		resp, err = client.VmCounters(ctx)
	case "vm-create":
		req := hvm.VmConfig{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		err = client.VmCreate(ctx, &req)
	case "vm-delete":
		err = client.VmDelete(ctx)
	case "vm-boot":
		err = client.VmBoot(ctx)
	case "vm-pause":
		err = client.VmPause(ctx)
	case "vm-resume":
		err = client.VmResume(ctx)
	case "vm-shutdown":
		err = client.VmShutdown(ctx)
	case "vm-reboot":
		err = client.VmReboot(ctx)
	case "vm-power-button":
		err = client.VmPowerButton(ctx)
	case "vm-resize":
		req := hvm.VmResize{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		err = client.VmResize(ctx, &req)
	case "vm-resize-zone":
		req := hvm.VmResizeZone{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		err = client.VmResizeZone(ctx, &req)
	case "vm-add-device":
		req := hvm.DeviceConfig{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		resp, err = client.VmAddDevice(ctx, &req)
	case "vm-remove-device":
		req := hvm.VmRemoveDevice{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		err = client.VmRemoveDevice(ctx, &req)
	case "vm-add-disk":
		req := hvm.DiskConfig{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		resp, err = client.VmAddDisk(ctx, &req)
	case "vm-add-fs":
		req := hvm.FsConfig{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		resp, err = client.VmAddFs(ctx, &req)
	case "vm-add-pmem":
		req := hvm.PmemConfig{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		resp, err = client.VmAddPmem(ctx, &req)
	case "vm-add-net":
		req := hvm.NetConfig{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		resp, err = client.VmAddNet(ctx, &req)
	case "vm-add-vsock":
		req := hvm.VsockConfig{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		resp, err = client.VmAddVsock(ctx, &req)
	case "vm-add-vdpa":
		req := hvm.VdpaConfig{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		resp, err = client.VmAddVdpa(ctx, &req)
	case "vm-shanshot":
		req := hvm.VmSnapshotConfig{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		err = client.VmShanshot(ctx, &req)
	case "vm-coredump":
		req := hvm.VmCoredumpData{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		err = client.VmCoredump(ctx, &req)
	case "vm-restore":
		req := hvm.RestoreConfig{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		err = client.VmRestore(ctx, &req)
	case "vm-receive-migration":
		req := hvm.ReceiveMigrationData{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		err = client.VmReceiveMigration(ctx, &req)
	case "vm-send-migration":
		req := hvm.SendMigrationData{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		err = client.VmSendMigration(ctx, &req)
	default:
		panic(fmt.Errorf("invalid action %s", action))
	}

	if err != nil {
		panic(fmt.Errorf("failed to execute %s: %w", action, err))
	}

	if resp != nil {
		defer os.Stdout.Sync()
		err = yaml.NewEncoder(os.Stdout).Encode(resp)
		if err != nil {
			panic(fmt.Errorf("failed to marshal response: %w", err))
		}
	}
}
