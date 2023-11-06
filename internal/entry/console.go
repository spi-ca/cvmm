package entry

import (
	"amuz.es/src/spi-ca/chmgr/internal/util/sys"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"amuz.es/src/spi-ca/chmgr/internal/hvm"
	"amuz.es/src/spi-ca/chmgr/internal/util"
	"github.com/spf13/viper"
)

func Console(name, nodeName string) {
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
	_ = sys.SetProcessName(fmt.Sprintf("node: %s", nodeName))

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
	err = h.OpenConsole(ctx)
	if err != nil {
		util.ErrLog.Fatal(err)
	}
}
