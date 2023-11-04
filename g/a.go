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

	psname, err := currentProcessname()
	if err != nil {
		panic(err)
	}

	util.InfoLog.Printf("ps: %s", psname)

	err = util.SetProcessName("hello")
	if err != nil {
		panic(err)
	}

	psname, err = currentProcessname()
	if err != nil {
		panic(err)
	}

	util.InfoLog.Printf("ps: %s", psname)
	<-time.After(60 * time.Second)
}

func currentProcessname() (string, error) {
	pid := os.Getpid()

	cmd := exec.Command("sh", "-c", fmt.Sprintf("ps | grep -E \"^%d\"", pid))

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	return string(out), nil
}
