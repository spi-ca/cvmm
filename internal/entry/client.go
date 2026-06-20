package entry

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"amuz.es/src/spi-ca/cvmm/internal/util/sys"

	"amuz.es/src/spi-ca/cvmm/internal/hvm"
	"amuz.es/src/spi-ca/cvmm/internal/model"
	"amuz.es/src/spi-ca/cvmm/internal/util"
	"gopkg.in/yaml.v3"

	"github.com/spf13/viper"
)

func decodeYAMLRequest(action hvm.ClientAction, req any) error {
	if err := yaml.NewDecoder(os.Stdin).Decode(req); err != nil {
		return fmt.Errorf("failed to decode request for %s: %w", action, err)
	}
	return nil
}

func encodeYAMLResponse(action hvm.ClientAction, resp any) error {
	if err := yaml.NewEncoder(os.Stdout).Encode(resp); err != nil {
		return fmt.Errorf("failed to encode response for %s: %w", action, err)
	}
	return nil
}

// Client handles the cvmm client command entrypoint.
func Client(name, nodeName string, action hvm.ClientAction) {
	ctx, cancel := context.WithCancel(context.Background())

	// Client cancels the socket API request when the command receives a termination signal.
	exitSignal := make(chan os.Signal, 1)
	signal.Notify(exitSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	defer signal.Stop(exitSignal)
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
		"\n	pid.filename=", viper.GetString("pid.filename"),
		"\n	console=", viper.GetBool("console"),
		"\n	cloudhypervisor.pid.filename=", viper.GetString("cloudhypervisor.pid.filename"),
		"\n	cloudhypervisor.api.filename=", viper.GetString("cloudhypervisor.api.filename"),
		"\n	volatile.directory=", viper.GetString("volatile.directory"),
		"\n	virtiofs.socket.filename.template=", viper.GetString("virtiofs.socket.filename.template"),
		"\n	virtiofs.pid.filename.template=", viper.GetString("virtiofs.pid.filename.template"),
		"\n	image.kernel.filename=", viper.GetString("image.kernel.filename"),
		"\n	image.initramfs.filename=", viper.GetString("image.initramfs.filename"),
		"\n	image.rootfs.filename=", viper.GetString("image.rootfs.filename"),
		"\n	runas=", viper.GetString("runas"),
		"\n---",
	)
	_ = sys.SetProcessName(fmt.Sprintf("node: %s", nodeName))

	h, err := hvm.Load(
		nodeName,
		viper.GetString("image.root"), viper.GetString("node.root"),
		viper.GetString("volatile.directory"), viper.GetString("manifest.filename"),

		viper.GetString("image.kernel.filename"),
		viper.GetString("image.initramfs.filename"),
		viper.GetString("image.rootfs.filename"),

		viper.GetString("pid.filename"),
		viper.GetString("cloudhypervisor.pid.filename"),
		viper.GetString("cloudhypervisor.api.filename"),
		viper.GetString("virtiofs.socket.filename.template"),
		viper.GetString("virtiofs.pid.filename.template"),

		util.LookupBinary(viper.GetString("cloudhypervisor.path")),
		util.LookupBinary(viper.GetString("virtiofsd.path")),
		viper.GetBool("console"),

		viper.GetString("runas"),
	)

	if err != nil {
		util.ErrLog.Fatal(err)
	}

	defer h.Close()

	var (
		client = h.GetClient()
		resp   any
	)

	switch action {
	case hvm.ClientActionVmmPing:
		resp, err = client.VmmPing(ctx)
	case hvm.ClientActionVmmShutdown:
		err = client.VmmShutdown(ctx)
	case hvm.ClientActionVmmNmi:
		err = client.VmmNmi(ctx)
	case hvm.ClientActionVmInfo:
		resp, err = client.VmInfo(ctx)
	case hvm.ClientActionVmCounters:
		resp, err = client.VmCounters(ctx)
	case hvm.ClientActionVmCreate:
		req := model.VmConfig{}
		if err = decodeYAMLRequest(action, &req); err == nil {
			err = client.VmCreate(ctx, req)
		}
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
		if err = decodeYAMLRequest(action, &req); err == nil {
			err = client.VmResize(ctx, req)
		}
	case hvm.ClientActionVmResizeZone:
		req := model.VmResizeZone{}
		if err = decodeYAMLRequest(action, &req); err == nil {
			err = client.VmResizeZone(ctx, req)
		}
	case hvm.ClientActionVmAddDevice:
		req := model.DeviceConfig{}
		if err = decodeYAMLRequest(action, &req); err == nil {
			resp, err = client.VmAddDevice(ctx, req)
		}
	case hvm.ClientActionVmAddUserDevice:
		req := model.VmAddUserDevice{}
		if err = decodeYAMLRequest(action, &req); err == nil {
			resp, err = client.VmAddUserDevice(ctx, req)
		}
	case hvm.ClientActionVmRemoveDevice:
		req := model.VmRemoveDevice{}
		if err = decodeYAMLRequest(action, &req); err == nil {
			err = client.VmRemoveDevice(ctx, req)
		}
	case hvm.ClientActionVmAddDisk:
		req := model.DiskConfig{}
		if err = decodeYAMLRequest(action, &req); err == nil {
			resp, err = client.VmAddDisk(ctx, req)
		}
	case hvm.ClientActionVmAddFs:
		req := model.FsConfig{}
		if err = decodeYAMLRequest(action, &req); err == nil {
			resp, err = client.VmAddFs(ctx, req)
		}
	case hvm.ClientActionVmAddPmem:
		req := model.PmemConfig{}
		if err = decodeYAMLRequest(action, &req); err == nil {
			resp, err = client.VmAddPmem(ctx, req)
		}
	case hvm.ClientActionVmAddNet:
		req := model.NetConfig{}
		if err = decodeYAMLRequest(action, &req); err == nil {
			resp, err = client.VmAddNet(ctx, req)
		}
	case hvm.ClientActionVmAddVsock:
		req := model.VsockConfig{}
		if err = decodeYAMLRequest(action, &req); err == nil {
			resp, err = client.VmAddVsock(ctx, req)
		}
	case hvm.ClientActionVmAddVdpa:
		req := model.VdpaConfig{}
		if err = decodeYAMLRequest(action, &req); err == nil {
			resp, err = client.VmAddVdpa(ctx, req)
		}
	case hvm.ClientActionVmSnapshot:
		req := model.VmSnapshotConfig{}
		if err = decodeYAMLRequest(action, &req); err == nil {
			err = client.VmSnapshot(ctx, req)
		}
	case hvm.ClientActionVmCoredump:
		req := model.VmCoredumpData{}
		if err = decodeYAMLRequest(action, &req); err == nil {
			err = client.VmCoredump(ctx, req)
		}
	case hvm.ClientActionVmRestore:
		req := model.RestoreConfig{}
		if err = decodeYAMLRequest(action, &req); err == nil {
			err = client.VmRestore(ctx, req)
		}
	case hvm.ClientActionVmReceiveMigration:
		req := model.ReceiveMigrationData{}
		if err = decodeYAMLRequest(action, &req); err == nil {
			err = client.VmReceiveMigration(ctx, req)
		}
	case hvm.ClientActionVmSendMigration:
		req := model.SendMigrationData{}
		if err = decodeYAMLRequest(action, &req); err == nil {
			err = client.VmSendMigration(ctx, req)
		}
	default:
		util.ErrLog.Fatalf("unknown action %s", action)
	}

	if err != nil {
		util.ErrLog.Fatal(err)
	}

	if resp != nil {
		defer os.Stdout.Sync()
		if err = encodeYAMLResponse(action, resp); err != nil {
			util.ErrLog.Fatal(err)
		}
	}

}
