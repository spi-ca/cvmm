package hvm

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unicode"

	"amuz.es/src/spi-ca/chmgr/internal/model"
	"amuz.es/src/spi-ca/chmgr/internal/util"
	"amuz.es/src/spi-ca/chmgr/internal/util/sys"
)

type Hypervisor struct {
	name string

	vmcfg        model.VmConfig
	virtiofsdcfg []model.VirtiofsConfig

	cloudhypervisorBinaryPath string
	cloudhypervisorPidPath    string
	virtiofsdBinaryPath       string

	cli *clientImpl
}

// TODO impl
func (i *Hypervisor) Ping(ctx context.Context) error {
	return nil
}
func (i *Hypervisor) Info(ctx context.Context) (*model.VmInfo, error) { return i.cli.VmInfo(ctx) }
func (i *Hypervisor) Counters(ctx context.Context) (*model.VmCounters, error) {
	return i.cli.VmCounters(ctx)
}

// TODO impl
func (i *Hypervisor) Boot(ctx context.Context) error { return i.cli.VmBoot(ctx) }

// TODO impl
func (i *Hypervisor) Pause(ctx context.Context) error { return i.cli.VmPause(ctx) }

// TODO impl
func (i *Hypervisor) Resume(ctx context.Context) error { return i.cli.VmResume(ctx) }

// TODO impl
func (i *Hypervisor) Shutdown(ctx context.Context) error { return i.cli.VmShutdown(ctx) }

// TODO impl
func (i *Hypervisor) Reboot(ctx context.Context) error { return i.cli.VmReboot(ctx) }

// TODO impl
func (i *Hypervisor) PowerButton(ctx context.Context) error { return i.cli.VmPowerButton(ctx) }
func (i *Hypervisor) Close()                                { i.cli.Close() }
func (i *Hypervisor) GetClient() Client                     { return i.cli }
func (i *Hypervisor) Start(parentCtx context.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	vmErrorChan := make(chan error, 1)

	cmd := exec.CommandContext(ctx, i.cloudhypervisorBinaryPath, "--api-socket", fmt.Sprintf("path=%s", i.cli.socketPath))
	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}

	if sys.IsPidFileActive(i.cloudhypervisorPidPath) {
		return fmt.Errorf("hypervisor already running")
	}

	_ = os.Remove(i.cli.socketPath)
	defer os.Remove(i.cli.socketPath)

	go func() {
		defer close(vmErrorChan)
		defer util.InfoLog.Printf("hypervisor stopped")

		err := i.invoke(cmd, i.cloudhypervisorPidPath)
		if err != nil {
			vmErrorChan <- fmt.Errorf("hypervisor failed: %w", err)
		}
	}()
	util.InfoLog.Printf("hypervisor started(api socket: %s)", i.cli.socketPath)

	virtiofsdErrorChan := chan error(nil)

	if cfgs := i.virtiofsdcfg; len(cfgs) > 0 {
		virtiofsdErrorChan = make(chan error, len(cfgs))
		go i.dispatchVirtiofsConfigs(ctx, virtiofsdErrorChan)
	}

	var errs []error

	select {
	case <-parentCtx.Done():
		// parent wants stop the hypervisor
		cancel()
		// wait hypervisor
		if err, ok := <-vmErrorChan; ok {
			errs = append(errs, err)
		}
	case err, ok := <-vmErrorChan:
		if ok {
			errs = append(errs, err)
		}
		cancel()
	case err, ok := <-virtiofsdErrorChan:
		if ok {
			errs = append(errs, err)
		}
		cancel()
		// wait hypervisor
		if err, ok = <-vmErrorChan; ok {
			errs = append(errs, err)
		}
	}

	//remain virtiofsd errors
	if virtiofsdErrorChan != nil {
		for range virtiofsdErrorChan {
			// wait sibling virtiofsd finished
		}
	}

	return errors.Join(errs...)
}

func (i *Hypervisor) OpenConsole(parentCtx context.Context) error {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	info, err := i.cli.VmInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to open console: %w", err)
	}

	ptyPath := info.Config.Console.File
	return util.OpenPty(ctx, os.Stdin, os.Stdout, ptyPath)
}

func (i *Hypervisor) dispatchVirtiofsConfigs(ctx context.Context, errorChan chan<- error) {
	defer func() {
		if err := recover(); err != nil {
			util.ErrLog.Printf("panic on virtiofsdWaiter: %v", err)
		}
		close(errorChan)
		util.InfoLog.Printf("all virtiofsd stopped")
	}()

	cfgs := i.virtiofsdcfg

	wg := &sync.WaitGroup{}
	wg.Add(len(cfgs))
	for _, cfg := range cfgs {
		name := fmt.Sprintf("virtiofsd-%s", cfg.Directory)
		cmd := exec.CommandContext(ctx, i.virtiofsdBinaryPath, cfg.CommandArgs()...)
		cmd.Cancel = func() error {
			return cmd.Process.Signal(syscall.SIGTERM)
		}

		go func() {
			defer func() {
				if err := recover(); err != nil {
					util.ErrLog.Printf("%s panic: %v", name, err)
				}
			}()

			defer wg.Done()
			util.InfoLog.Printf("virtiofsd[%s] started", name)
			defer util.InfoLog.Printf("virtiofsd[%s] stopped", name)
			err := i.invoke(cmd, "")
			if err != nil {
				errorChan <- fmt.Errorf("%s failed: %w", name, err)
			}
		}()
	}
	wg.Wait()
}

func (i *Hypervisor) invoke(cmd *exec.Cmd, pidPath string) error {

	cmd.SysProcAttr = &syscall.SysProcAttr{}

	if err := sys.ApplySysProAttrPGid(cmd.SysProcAttr); err != nil {
		return fmt.Errorf("failed to set process group id: %w", err)
	}

	if err := sys.ApplySysProAttrPdeathsig(cmd.SysProcAttr, syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to set pdeathsig(%s): %w", syscall.SIGTERM, err)
	}

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	// On Linux, pdeathsig will kill the child process when the thread dies,
	// not when the process dies. runtime.LockOSThread ensures that as long
	// as this function is executing that OS thread will still be around
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start process(%s): %w", cmd.Path, err)
	}

	res := &util.ExecutionResult{PID: cmd.Process.Pid}

	wg := &sync.WaitGroup{}
	wg.Add(2)
	go i.handleStdout(res, stdout, wg.Done)
	go i.handleStderr(res, stderr, wg.Done)

	if len(pidPath) > 0 {
		pidClean, err := sys.AcquirePidFile(pidPath, cmd.Process.Pid)
		if err != nil {
			return fmt.Errorf("failed to start process(%s): %w", cmd.Path, err)
		}
		defer pidClean()
	}

	res.Err = cmd.Wait()
	wg.Wait()

	return res.HandleError()
}

func (i *Hypervisor) handleStdout(res *util.ExecutionResult, reader io.Reader, closer func()) {
	defer closer()
	prefix := fmt.Sprintf("[%d] ", res.PID)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimRightFunc(scanner.Text(), unicode.IsSpace)
		util.InfoLog.Print(prefix, line)
	}
}

func (i *Hypervisor) handleStderr(res *util.ExecutionResult, reader io.Reader, closer func()) {
	defer closer()
	prefix := fmt.Sprintf("[%d] ", res.PID)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimRightFunc(scanner.Text(), unicode.IsSpace)
		res.AppendLogLine(line)
		util.ErrLog.Print(prefix, line)
	}
}
