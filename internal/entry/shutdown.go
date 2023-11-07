package entry

import (
	"amuz.es/src/spi-ca/chmgr/internal/hvm"
	"amuz.es/src/spi-ca/chmgr/internal/util"
	"amuz.es/src/spi-ca/chmgr/internal/util/sys"
	"context"
	"fmt"
	"github.com/spf13/viper"
	"os"
	"os/signal"
	"syscall"
)

func Shutdown(name, nodeName string) {
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
		"\n	cloudhypervisor.api.filename=", viper.GetString("cloudhypervisor.api.filename"),
		"\n	volatile.directory=", viper.GetString("volatile.directory"),
		"\n	virtiofs.socket.filename.template=", viper.GetString("virtiofs.socket.filename.template"),
		"\n	image.kernel.filename=", viper.GetString("image.kernel.filename"),
		"\n	image.initramfs.filename=", viper.GetString("image.initramfs.filename"),
		"\n	image.rootfs.filename=", viper.GetString("image.rootfs.filename"),
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

		viper.GetString("cloudhypervisor.api.filename"),
		viper.GetString("virtiofs.socket.filename.template"),

		util.LookupBinary(viper.GetString("cloudhypervisor.path")),
		util.LookupBinary(viper.GetString("virtiofsd.path")),
	)

	if err != nil {
		util.ErrLog.Fatal(err)
	}

	defer h.Close()
	err = h.OpenConsole(ctx)
	if err != nil {
		util.ErrLog.Fatal(err)
	}

	h.GetClient()

	//todo impl
	//status, err := mgr.QueryStatus()
	//if err != nil {
	//	return
	//}
	//
	//if !status.Running {
	//	logger.Infof("cpu halted, so not initiate shutdown")
	//	return
	//}
	//
	//logger.Infof("current vm status %v", status.Status)
	//err = mgr.SystemPowerdown()
	//if err != nil {
	//	return
	//}
	//logger.Infof("initiated shutdown")
	//
	//pid := viper.GetInt("vm_shutdown_pid")
	//shutdownTimeout := viper.GetDuration("vm_shutdown_timeout")
	//if pid < 0 {
	//	return
	//}
	//
	//var (
	//	shutdownDeadline              = time.Now().Add(shutdownTimeout)
	//	shutdownContext, shutdownDone = context.WithDeadline(context.Background(), shutdownDeadline)
	//	cleanFinished                 atomic.Bool
	//)
	//go func() {
	//	defer shutdownDone()
	//
	//	process, err := os.FindProcess(pid)
	//	if err != nil {
	//		cleanFinished.Store(true)
	//		logger.Errorf("Failed to find process: %s\n", err)
	//		return
	//	}
	//
	//	ticker := time.NewTicker(300 * time.Millisecond)
	//	defer ticker.Stop()
	//
	//	for {
	//		select {
	//		case <-ticker.C:
	//			err := process.Signal(syscall.Signal(0))
	//			if err == nil {
	//				continue
	//			} else if err == os.ErrProcessDone {
	//				cleanFinished.Store(true)
	//				logger.Infof("process finished")
	//				return
	//			} else {
	//				fmt.Printf("process.Signal on pid %d returned: %v\n", pid, err)
	//				return
	//			}
	//		case <-shutdownContext.Done():
	//			return
	//		}
	//	}
	//}()
	//
	//logger.Infof("wait until pid(%d) finished", pid)
	//<-shutdownContext.Done()
	//if cleanFinished.Load() {
	//	return
	//}
	//err = mgr.Quit()
	//if err != nil {
	//	return
	//}
	//logger.Infof("initiated quit")
}
