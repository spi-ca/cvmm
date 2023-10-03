package entry

import (
	"amuz.es/src/spi-ca/chmgr/internal/cloudhypervisor"
	"amuz.es/src/spi-ca/chmgr/internal/hvm"
	"amuz.es/src/spi-ca/chmgr/internal/util"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/viper"
)

func Console(nodeName string) {
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

	util.InfoLog.SetPrefix(fmt.Sprintf("%s[%d]&1>", viper.GetString("log.prefix"), os.Getpid()))
	util.ErrLog.SetPrefix(fmt.Sprintf("%s[%d]&2>", viper.GetString("log.prefix"), os.Getpid()))
	util.InfoLog.Print(
		"args:",
		"\n	argNodeName=", nodeName,
		"\n	log.prefix=", viper.GetString("log.prefix"),
		"\n	virtiofsd.path=", viper.GetString("virtiofsd.path"),
		"\n	cloudhypervisor.path=", viper.GetString("cloudhypervisor.path"),
		"\n	image.root=", viper.GetString("image.root"),
		"\n	node.root=", viper.GetString("node.root"),
		"\n	manifest.filename=", viper.GetString("manifest.filename"),
		"\n	volatile.directory=", viper.GetString("volatile.directory"),
		"\n---",
	)

	util.InfoLog.Printf("chmgr/console(%s) had been initiated", nodeName)

	h := &hvm.Hypervisor{
		imageRoot:         viper.GetString("image.root"),
		nodeRoot:          viper.GetString("node.root"),
		volatileDirectory: viper.GetString("volatile.directory"),
	}

	err := h.LoadFromYaml(viper.GetString("manifest.filename"))
	if err != nil {
		util.ErrLog.Fatal(err)
	}

	started := time.Now()
	//err := runner.Execute(ctx, srcPath, dstPath)
	//
	c := cloudhypervisor.NewNodeClient(h.VolatilePath())
	//
	//ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	//defer cancel()
	//
	//errorChan := make(chan error, 1)
	//go internal.NodeStatusChecker(ctx, c, internal.NodeStatusRunning, errorChan)
	//for err := range errorChan {
	//	log.Printf("err %v", err)
	//}

	util.InfoLog.Printf("initiated shutdown")

	ended := time.Now()
	if err == nil {
		util.InfoLog.Printf("chmgr/console(%s) had been ended in %s", nodeName, ended.Sub(started))
	} else {
		util.ErrLog.Fatal(err)
	}
}
