package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/viper"

	"amuz.es/src/spi-ca/chmgr/internal/util"
)

func main() {

	util.InfoLog.SetPrefix(fmt.Sprintf("%s[%d]&1>", viper.GetString("log.prefix"), os.Getpid()))
	util.ErrLog.SetPrefix(fmt.Sprintf("%s[%d]&2>", viper.GetString("log.prefix"), os.Getpid()))

	err := util.SetProcessName("hello")
	if err != nil {
		panic(err)
	}

	cmd := exec.Command("sh", "-c", "ps | grep prctl")
	err = cmd.Run()
	if err != nil {
		panic(err)
	}

	<-time.After(time.Second)
}
