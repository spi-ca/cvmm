package entry

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"amuz.es/src/spi-ca/chmgr/internal/hvm"
	"amuz.es/src/spi-ca/chmgr/internal/model"
	"amuz.es/src/spi-ca/chmgr/internal/util"
	"gopkg.in/yaml.v3"

	"github.com/spf13/viper"
)

func Client(name, nodeName string, action hvm.ClientAction) {
	ctx, cancel := context.WithCancel(context.Background())

	// 시그널 처리
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

	util.InfoLog.SetPrefix(fmt.Sprintf("%s[%d]&1>", name, os.Getpid()))
	util.ErrLog.SetPrefix(fmt.Sprintf("%s[%d]&2>", name, os.Getpid()))
	util.InfoLog.Print(
		"args:",
		"\n	argNodeName=", nodeName,
		"\n	virtiofsd.path=", viper.GetString("virtiofsd.path"),
		"\n	cloudhypervisor.path=", viper.GetString("cloudhypervisor.path"),
		"\n	image.root=", viper.GetString("image.root"),
		"\n	node.root=", viper.GetString("node.root"),
		"\n	manifest.filename=", viper.GetString("manifest.filename"),
		"\n	cloudhypervisor.monitor.filename=", viper.GetString("cloudhypervisor.monitor.filename"),
		"\n	volatile.directory=", viper.GetString("volatile.directory"),
		"\n---",
	)

	h, err := hvm.Load(
		nodeName,
		viper.GetString("image.root"), viper.GetString("node.root"),
		viper.GetString("volatile.directory"), viper.GetString("manifest.filename"),
		viper.GetString("cloudhypervisor.monitor.filename"),
		util.LookupBinary(viper.GetString("cloudhypervisor.path")),
		util.LookupBinary(viper.GetString("virtiofsd.path")),
	)

	if err != nil {
		util.ErrLog.Fatal(err)
	}

	defer h.Close()

	// todo impl
	//ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	//defer cancel()
	//
	//errorChan := make(chan error, 1)
	//go internal.NodeStatusChecker(ctx, c, internal.NodeStatusRunning, errorChan)
	//for err := range errorChan {
	//	util.ErrLog.Printf("err %v", err)
	//}

	var (
		client = h.GetClient()
		resp   any
	)

	switch action {
	case hvm.ClientActionVmmPing:
		resp, err = client.VmmPing(ctx)
	case hvm.ClientActionVmmShutdown:
		err = client.VmmShutdown(ctx)
	case hvm.ClientActionVmInfo:
		resp, err = client.VmInfo(ctx)
	case hvm.ClientActionVmCounters:
		resp, err = client.VmCounters(ctx)
	case hvm.ClientActionVmCreate:
		req := model.VmConfig{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		err = client.VmCreate(ctx, req)
	case hvm.ClientActionVmDelete:
		err = client.VmDelete(ctx)
	case hvm.ClientActionVmBoot:
		err = client.VmBoot(ctx)
	case hvm.ClientActionVmPause:
		err = client.VmPause(ctx)
	case hvm.ClientActionVmResume:
		err = client.VmResume(ctx)
	case hvm.ClientActionVmShutdown:
		err = client.VmShutdown(ctx)
	case hvm.ClientActionVmReboot:
		err = client.VmReboot(ctx)
	case hvm.ClientActionVmPowerButton:
		err = client.VmPowerButton(ctx)
	case hvm.ClientActionVmResize:
		req := model.VmResize{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		err = client.VmResize(ctx, req)
	case hvm.ClientActionVmResizeZone:
		req := model.VmResizeZone{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		err = client.VmResizeZone(ctx, req)
	case hvm.ClientActionVmAddDevice:
		req := model.DeviceConfig{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		resp, err = client.VmAddDevice(ctx, req)
	case hvm.ClientActionVmRemoveDevice:
		req := model.VmRemoveDevice{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		err = client.VmRemoveDevice(ctx, req)
	case hvm.ClientActionVmAddDisk:
		req := model.DiskConfig{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		resp, err = client.VmAddDisk(ctx, req)
	case hvm.ClientActionVmAddFs:
		req := model.FsConfig{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		resp, err = client.VmAddFs(ctx, req)
	case hvm.ClientActionVmAddPmem:
		req := model.PmemConfig{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		resp, err = client.VmAddPmem(ctx, req)
	case hvm.ClientActionVmAddNet:
		req := model.NetConfig{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		resp, err = client.VmAddNet(ctx, req)
	case hvm.ClientActionVmAddVsock:
		req := model.VsockConfig{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		resp, err = client.VmAddVsock(ctx, req)
	case hvm.ClientActionVmAddVdpa:
		req := model.VdpaConfig{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		resp, err = client.VmAddVdpa(ctx, req)
	case hvm.ClientActionVmSnapshot:
		req := model.VmSnapshotConfig{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		err = client.VmSnapshot(ctx, req)
	case hvm.ClientActionVmCoredump:
		req := model.VmCoredumpData{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		err = client.VmCoredump(ctx, req)
	case hvm.ClientActionVmRestore:
		req := model.RestoreConfig{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		err = client.VmRestore(ctx, req)
	case hvm.ClientActionVmReceiveMigration:
		req := model.ReceiveMigrationData{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		err = client.VmReceiveMigration(ctx, req)
	case hvm.ClientActionVmSendMigration:
		req := model.SendMigrationData{}
		err = yaml.NewDecoder(os.Stdin).Decode(&req)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal request for %s: %w", action, err))
		}
		err = client.VmSendMigration(ctx, req)
	default:
		util.ErrLog.Fatalf("unknown action %s", action)
	}

	if err != nil {
		util.ErrLog.Fatal(err)
	}

	if resp != nil {
		defer os.Stdout.Sync()
		err = yaml.NewEncoder(os.Stdout).Encode(resp)
		if err != nil {
			util.ErrLog.Printf("failed to marshal response: %w", err)
		}
	}

}
