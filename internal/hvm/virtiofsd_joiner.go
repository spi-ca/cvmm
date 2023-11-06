package hvm

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	"amuz.es/src/spi-ca/chmgr/internal/model"
	"amuz.es/src/spi-ca/chmgr/internal/util"
	"amuz.es/src/spi-ca/chmgr/internal/util/sys"
)

type virtiofsdJoiner struct {
	virtiofsdBinaryPath string
}

func (i *virtiofsdJoiner) Execute(ctx context.Context, configs []model.VirtiofsConfig) <-chan error {
	errorChan := make(chan error, len(configs))
	go i.dispatch(ctx, configs, errorChan)
	return errorChan
}

func (i *virtiofsdJoiner) dispatch(ctx context.Context, configs []model.VirtiofsConfig, errorChan chan<- error) {
	defer func() {
		if err := recover(); err != nil {
			util.ErrLog.Printf("panic on workerJoiner: %v", err)
		}
		close(errorChan)
	}()

	wg := &sync.WaitGroup{}
	wg.Add(len(configs))
	for _, cfg := range configs {
		go i.submit(ctx, cfg, errorChan, wg.Done)
	}
	wg.Wait()
}

func (i *virtiofsdJoiner) submit(ctx context.Context, config model.VirtiofsConfig, errorChan chan<- error, closer func()) {
	defer closer()
	defer func() {
		if err := recover(); err != nil {
			util.ErrLog.Printf("virtiofsd panic on worker: %v", err)
		}
	}()

	started := time.Now()
	err := i.Run(ctx, config)
	ended := time.Now()
	if err != nil {
		errorChan <- fmt.Errorf("virtiofsd failed in %s: %w", ended.Sub(started), err)
	}
}

func (i *virtiofsdJoiner) Run(parentContext context.Context, entry model.VirtiofsConfig) error {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	invoke := exec.CommandContext(ctx, i.virtiofsdBinaryPath, entry.CommandArgs()...)
	invoke.SysProcAttr = &syscall.SysProcAttr{}

	if err := sys.ApplySysProAttrPGid(invoke.SysProcAttr); err != nil {
		return fmt.Errorf("failed to set process group id: %w", err)
	}

	if err := sys.ApplySysProAttrPdeathsig(invoke.SysProcAttr, syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to set pdeathsig(%s): %w", syscall.SIGTERM, err)
	}

	stdout, _ := invoke.StdoutPipe()
	stderr, _ := invoke.StderrPipe()

	// On Linux, pdeathsig will kill the child process when the thread dies,
	// not when the process dies. runtime.LockOSThread ensures that as long
	// as this function is executing that OS thread will still be around
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := invoke.Start(); err != nil {
		return fmt.Errorf("failed to start process(%s): %w", i.virtiofsdBinaryPath, err)
	}

	res := &util.ExecutionResult{PID: invoke.Process.Pid}

	go func() {
		select {
		case <-parentContext.Done():
			_ = invoke.Process.Signal(syscall.SIGTERM)
		case <-ctx.Done():
		}
	}()

	wg := &sync.WaitGroup{}
	wg.Add(2)
	go i.handleStdout(res, stdout, wg.Done)
	go i.handleStderr(res, stderr, wg.Done)
	res.Err = invoke.Wait()
	wg.Wait()

	return res.HandleError()
}

func (i *virtiofsdJoiner) handleStdout(res *util.ExecutionResult, reader io.Reader, closer func()) {
	defer closer()
	prefix := fmt.Sprintf("[%d] ", res.PID)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimRightFunc(scanner.Text(), unicode.IsSpace)
		util.InfoLog.Print(prefix, line)
	}
}

func (i *virtiofsdJoiner) handleStderr(res *util.ExecutionResult, reader io.Reader, closer func()) {
	defer closer()
	prefix := fmt.Sprintf("[%d] ", res.PID)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimRightFunc(scanner.Text(), unicode.IsSpace)
		res.AppendLogLine(line)
		util.ErrLog.Print(prefix, line)
	}
}
