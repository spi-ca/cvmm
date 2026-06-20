package entry

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"amuz.es/src/spi-ca/cvmm/internal/hvm"
	"amuz.es/src/spi-ca/cvmm/internal/util"
	"amuz.es/src/spi-ca/cvmm/internal/util/sys"
	"github.com/spf13/viper"
)

// Shutdown handles the cvmm shutdown command entrypoint.
func Shutdown(name, nodeName string) {
	ctx, cancel := context.WithCancel(context.Background())

	// Shutdown cancels its pid-file termination workflow when interrupted.
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
	h.Shutdown(ctx)
}
