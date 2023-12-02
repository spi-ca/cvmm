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
	"time"
	"unicode"

	"amuz.es/src/spi-ca/cvmm/internal/model"
	"amuz.es/src/spi-ca/cvmm/internal/util"
	"amuz.es/src/spi-ca/cvmm/internal/util/sys"
	"github.com/sony/gobreaker"
	"golang.org/x/sys/unix"
)

const (
	availableAwaitingLimit = 30
	shutdownDeadline       = 30 * time.Second
)

var (
	errRunAsNotSpecified = errors.New("runas not specified")
)

type Hypervisor struct {
	name string

	vmcfg        model.VmConfig
	virtiofsdcfg []model.VirtiofsConfig

	pidPath                   string
	cloudhypervisorBinaryPath string
	cloudhypervisorPidPath    string
	virtiofsdBinaryPath       string
	runAs                     *syscall.Credential

	cli *clientImpl
}

// TODO impl
func (i *Hypervisor) Ping(ctx context.Context) error {
	_, err := i.cli.VmmPing(ctx)
	return err
}
func (i *Hypervisor) Info(ctx context.Context) (*model.VmInfo, error) { return i.cli.VmInfo(ctx) }
func (i *Hypervisor) Counters(ctx context.Context) (*model.VmCounters, error) {
	return i.cli.VmCounters(ctx)
}

func (i *Hypervisor) Shutdown(ctx context.Context) {
	pid, err := sys.ReadPidFile(i.pidPath)
	if err != nil {
		util.ErrLog.Printf("Failed to read a pid file: %s\n", i.pidPath)
		return
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		util.ErrLog.Printf("Failed to find process: %s\n", err)
		return
	}

	err = process.Signal(unix.SIGTERM)
	if err == nil {
		shutdownCtx, shutdownDone := context.WithTimeout(ctx, shutdownDeadline)
		defer shutdownDone()

		err = sys.WaitUntilProcessFinished(shutdownCtx, pid)
		switch err {
		case nil:
			return
		case context.DeadlineExceeded:
			util.ErrLog.Printf("termination timeout(%s) exceed: %s\n", shutdownDeadline, err)
		case context.Canceled:
			// do nothing
		default:
			util.ErrLog.Printf("termination awaiting failed: %s\n", err)
		}
	}

	process.Kill()
}

func (i *Hypervisor) Reboot(ctx context.Context) error { return i.cli.VmReboot(ctx) }
func (i *Hypervisor) Close()                           { i.cli.Close() }
func (i *Hypervisor) GetClient() Client                { return i.cli }
func (i *Hypervisor) Start(parentCtx context.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pidClean, err := sys.AcquirePidFile(i.pidPath, os.Getpid())
	if err != nil {
		return fmt.Errorf("failed to acquire a pid: %w", err)
	}
	defer pidClean()

	vmErrorChan := make(chan error, 1)

	cmd := exec.CommandContext(ctx, i.cloudhypervisorBinaryPath, "--api-socket", fmt.Sprintf("path=%s", i.cli.socketPath))
	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{}

	if err = sys.ApplySysProAttrPGid(cmd.SysProcAttr); err != nil {
		return fmt.Errorf("failed to set process group id: %w", err)
	}

	if err = sys.ApplySysProAttrPdeathsig(cmd.SysProcAttr, syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to set pdeathsig(%s): %w", syscall.SIGTERM, err)
	}

	cmd.SysProcAttr.AmbientCaps = []uintptr{unix.CAP_NET_ADMIN}
	cmd.SysProcAttr.Credential = i.runAs

	if sys.IsPidFileActive(i.cloudhypervisorPidPath) {
		return fmt.Errorf("hypervisor already running")
	}

	_ = os.Remove(i.cli.socketPath)

	go func() {
		defer close(vmErrorChan)
		defer util.InfoLog.Printf("hypervisor stopped")

		err := i.invoke(cmd, i.cloudhypervisorPidPath)
		if err != nil {
			vmErrorChan <- fmt.Errorf("hypervisor failed: %w", err)
		}
	}()

	if err := i.waitForHypervisorAvailable(ctx); err != nil {
		return err
	}

	util.InfoLog.Printf("hypervisor started(pid: %d, i socket: %s)", cmd.Process.Pid, i.cli.socketPath)

	if err := i.cli.VmCreate(ctx, i.vmcfg); err != nil {
		return err
	}
	recoilerClosedChan := make(chan struct{})
	go i.virtiofsdRecoiler(ctx, recoilerClosedChan)
	go i.hypervisorStatusMonitor(parentCtx)

	if err := i.cli.VmBoot(ctx); err != nil {
		return err
	}
	util.InfoLog.Printf("hypervisor boot")

	var errs []error

	select {
	case <-parentCtx.Done():
		if err := i.cli.VmPowerButton(ctx); err != nil {
			errs = append(errs, err)
		} else {
			util.InfoLog.Printf("hypervisor shutdown requested")
		}
		// wait hypervisor
		if err, ok := <-vmErrorChan; ok {
			errs = append(errs, err)
		}
		// parent wants stop the hypervisor
	case err, ok := <-vmErrorChan:
		if ok {
			errs = append(errs, err)
		}
	}

	cancel()

	<-recoilerClosedChan

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

func (i *Hypervisor) invoke(cmd *exec.Cmd, pidPath string) error {
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

func (i *Hypervisor) virtiofsdRecoiler(ctx context.Context, closer chan<- struct{}) {
	close(closer)

	cfgs := i.virtiofsdcfg

	if len(cfgs) <= 0 {
		return
	}

	wg := &sync.WaitGroup{}
	wg.Add(len(cfgs))

	for idx := range cfgs {
		go func(cfg model.VirtiofsConfig) {
			name := fmt.Sprintf("virtiofsd-%s", cfg.Directory)
			defer func() {
				if err := recover(); err != nil {
					util.ErrLog.Printf("%s panic: %v", name, err)
				}
				wg.Done()
			}()

			b := gobreaker.NewCircuitBreaker(gobreaker.Settings{
				Name:        name,
				MaxRequests: 2,
				Interval:    100 * time.Millisecond,
				Timeout:     3 * time.Second,
				ReadyToTrip: func(counts gobreaker.Counts) bool {
					failureRatio := counts.TotalFailures
					failureRatio *= 100
					failureRatio /= counts.Requests
					return counts.Requests >= 3 && failureRatio >= 60
				},
				OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
					if from == gobreaker.StateClosed && to == gobreaker.StateOpen {
						util.ErrLog.Print("endpoint unavailable")
					} else if from == gobreaker.StateHalfOpen && to == gobreaker.StateClosed {
						util.ErrLog.Print("endpoint is returning available")
					}
				},
			})

			recoil := true

			for recoil {
				b.Execute(func() (_ interface{}, err error) {
					_ = os.Remove(cfg.SocketPath)

					cmd := exec.CommandContext(ctx, i.virtiofsdBinaryPath, cfg.CommandArgs()...)
					cmd.Cancel = func() error {
						return cmd.Process.Signal(syscall.SIGTERM)
					}

					cmd.SysProcAttr = &syscall.SysProcAttr{}

					if err = sys.ApplySysProAttrPGid(cmd.SysProcAttr); err != nil {
						util.ErrLog.Printf("failed to set process group id: %s", err)
						return
					}

					if err = sys.ApplySysProAttrPdeathsig(cmd.SysProcAttr, syscall.SIGTERM); err != nil {
						util.ErrLog.Printf("failed to set pdeathsig(%s): %s", syscall.SIGTERM, err)
						return
					}
					cmd.SysProcAttr.AmbientCaps = []uintptr{
						unix.CAP_CHOWN,
						unix.CAP_DAC_OVERRIDE,
						unix.CAP_FOWNER,
						unix.CAP_FSETID,
						unix.CAP_SETGID,
						unix.CAP_SETUID,
						unix.CAP_MKNOD,
						unix.CAP_SETFCAP,
						unix.CAP_DAC_READ_SEARCH,
					}

					util.InfoLog.Printf("virtiofsd[%s] started", name)

					if err = i.invoke(cmd, ""); err != nil {
						util.ErrLog.Printf("virtiofsd[%s] failed: %s", name, err)
					} else {
						util.InfoLog.Printf("virtiofsd[%s] stopped", name)
					}

					select {
					case <-ctx.Done():
						recoil = false
					default:
					}
					return
				})
			}
		}(cfgs[idx])
	}

	wg.Wait()
}

func (i *Hypervisor) waitForHypervisorAvailable(ctx context.Context) error {
	var err error
	for iter := 0; iter < availableAwaitingLimit; iter++ {
		if _, err = i.cli.VmmPing(ctx); err == nil {
			break
		}
		<-time.After(time.Second)
	}

	if err != nil {
		return fmt.Errorf("hypervisor unavailable: %w", err)
	}
	return nil
}

func (i *Hypervisor) hypervisorStatusMonitor(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	previousStatus := model.NodeStatusCreated

	util.InfoLog.Printf("hypervisor status : %s", previousStatus)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := i.cli.VmmPing(ctx); err != nil {
				util.InfoLog.Printf("hypervisor unavilable: %s", err)
				return
			} else if info, err := i.cli.VmInfo(ctx); err != nil {
				util.InfoLog.Printf("failed to get hypervisor info: %s", err)
				return
			} else if previousStatus != info.State {
				util.InfoLog.Printf("hypervisor status : %s", info.State)
				previousStatus = info.State
			} else {
				previousStatus = info.State
			}

		}
	}
}
