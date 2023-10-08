package entry

import (
	"amuz.es/src/spi-ca/chmgr/internal"
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

func Starter(nodeName string) {
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
		"\n---",
	)

	//util.InfoLog.Printf("fast-volume-sync/copier(%s -> %s) had been initiated", srcPath, dstPath)

	runner := hvm.Hypervisor{
		//	FileMode: sys.UnFilemodeStr(viper.GetString("file.mode")),
		//	Args: args.RsyncArgs{
		//		Verbose:            viper.GetBool("rsync.verbose"),
		//		Delete:             viper.GetBool("rsync.delete"),
		//		PreservePermission: viper.GetBool("rsync.perms"),
		//		PreserveOwnership:  viper.GetBool("rsync.owner"),
		//		CopySpecial:        viper.GetBool("rsync.special"),
		//		Compress:           viper.GetBool("rsync.compress"),
		//		WholeFile:          viper.GetBool("rsync.whole.file"),
		//		Inplace:            viper.GetBool("rsync.inplace"),
		//		Recursive:          viper.GetBool("rsync.recursive"),
		//		Port:               viper.GetInt("rsync.port"),
		//		BandwidthLimit:     viper.GetString("rsync.bandwidth.limit"),
		//	},
		//	UseRsync:         viper.GetBool("rsync.enabled"),
		//	ScanDuration:     viper.GetDuration("scan.deadline"),
		//	FinderBinaryPath: util.LookupBinary(viper.GetString("scan.find.path")),
		//	TaskSize:         viper.GetInt("task.size"),
		//	ChunkSize:        viper.GetInt("chunk.size"),
		//	Retry: args.RetryArgs{
		//		Attempts:  viper.GetInt("retry.attempts"),
		//		Delay:     viper.GetDuration("retry.delay"),
		//		MaxDelay:  viper.GetDuration("retry.max.delay"),
		//		MaxJitter: viper.GetDuration("retry.max.jitter"),
		//	},
	}
	started := time.Now()
	err := runner.Execute(ctx, srcPath, dstPath)

	c := internal.NewNodeClient(os.Args[1])

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	errorChan := make(chan error, 1)
	go internal.NodeStatusChecker(ctx, c, internal.NodeStatusRunning, errorChan)
	for err := range errorChan {
		util.InfoLog.Printf("err %v", err)
	}

	util.InfoLog.Printf("initiated shutdown")

	ended := time.Now()
	if err == nil {
		util.InfoLog.Printf("chmgr/starter(%s) had been ended in %s", nodeName, ended.Sub(started))
	} else {
		util.ErrLog.Fatal(err)
	}
}
