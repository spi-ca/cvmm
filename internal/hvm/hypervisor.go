package hvm

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
)

var (
	shutdownDeadline = 30 * time.Second
)

var (
	errRunAsNotSpecified = errors.New("runas not specified")
)

// Hypervisor owns one node's cloud-hypervisor client, child process paths, and virtiofsd runtime configuration.
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

// Ping verifies that the cloud-hypervisor VMM API is reachable.
func (i *Hypervisor) Ping(ctx context.Context) error {
	_, err := i.cli.VmmPing(ctx)
	return err
}

// Info returns the current VM information from the cloud-hypervisor API.
func (i *Hypervisor) Info(ctx context.Context) (*model.VmInfo, error) { return i.cli.VmInfo(ctx) }

// Counters returns the current VM counters from the cloud-hypervisor API.
func (i *Hypervisor) Counters(ctx context.Context) (*model.VmCounters, error) {
	return i.cli.VmCounters(ctx)
}

// Shutdown terminates the cvmm manager process recorded in the node pid file, escalating to kill after a timeout.
func (i *Hypervisor) Shutdown(ctx context.Context) {
	pid, err := sys.ReadPidFile(i.pidPath)
	if err != nil {
		util.ErrLog.Printf("Failed to read a pid file: %s\n", i.pidPath)
		return
	}

	status, statusErr := sys.ProcessIdentityWithExpectation(pid, i.managerProcessIdentity())
	switch status {
	case sys.ProcessIdentityInactive:
		util.ErrLog.Printf("pidfile(%s) target is not running: %d\n", i.pidPath, pid)
		return
	case sys.ProcessIdentityMismatch:
		util.ErrLog.Printf("refusing to signal pidfile(%s) because process(%d) is not the active manager for node %q\n", i.pidPath, pid, i.name)
		return
	case sys.ProcessIdentityUnknown:
		if statusErr != nil {
			util.ErrLog.Printf("failed to inspect pidfile(%s): %s\n", i.pidPath, statusErr)
		} else {
			util.ErrLog.Printf("refusing to signal pidfile(%s) because process(%d) identity is unavailable\n", i.pidPath, pid)
		}
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
			// Parent cancellation is an expected shutdown path.
		default:
			util.ErrLog.Printf("termination awaiting failed: %s\n", err)
		}
	}

	process.Kill()
}

// Reboot requests a guest reboot through the cloud-hypervisor API.
func (i *Hypervisor) Reboot(ctx context.Context) error { return i.cli.VmReboot(ctx) }

// Close releases resources held by the receiver.
func (i *Hypervisor) Close() { i.cli.Close() }

// GetClient exposes the node-local cloud-hypervisor API client used by entry commands.
func (i *Hypervisor) GetClient() Client { return i.cli }

func (i *Hypervisor) managerProcessName() string {
	return fmt.Sprintf("node: %s", i.name)
}

func (i *Hypervisor) managerProcessIdentity() sys.ProcessIdentityExpectation {
	return sys.ProcessIdentityExpectation{
		Name:               i.managerProcessName(),
		ExecutableBasename: filepath.Base(os.Args[0]),
		CommandArgs:        []string{"start", i.name},
	}
}

func (i *Hypervisor) cloudHypervisorProcessIdentity() sys.ProcessIdentityExpectation {
	return sys.ProcessIdentityExpectation{
		Name:               filepath.Base(i.cloudhypervisorBinaryPath),
		ExecutableBasename: filepath.Base(i.cloudhypervisorBinaryPath),
		CommandArgs:        []string{"--api-socket", fmt.Sprintf("path=%s", i.cli.socketPath)},
	}
}

func (i *Hypervisor) preparePidFile(path string, expected sys.ProcessIdentityExpectation, alreadyRunningMessage string) error {
	pid, _, err := sys.ReadPidFileInfo(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("failed to inspect pidfile(%s): %w", path, err)
	}

	status, statusErr := sys.ProcessIdentityWithExpectation(pid, expected)
	switch status {
	case sys.ProcessIdentityInactive, sys.ProcessIdentityMismatch:
		util.ErrLog.Printf("reusing stale pidfile(%s) for unrelated process(%d)", path, pid)
		return nil
	case sys.ProcessIdentityMatch:
		return errors.New(alreadyRunningMessage)
	case sys.ProcessIdentityUnknown:
		if statusErr != nil {
			return fmt.Errorf("failed to inspect pidfile(%s): %w", path, statusErr)
		}
		return fmt.Errorf("active pidfile(%s) belongs to unverifiable process(%d)", path, pid)
	default:
		return nil
	}
}

func (i *Hypervisor) acquireManagerPidFile() (func(), error) {
	if err := i.preparePidFile(i.pidPath, i.managerProcessIdentity(), fmt.Sprintf("already running(%s)", i.pidPath)); err != nil {
		return nil, err
	}

	pidClean, err := sys.AcquirePidFileReplacing(i.pidPath, os.Getpid())
	if err != nil {
		return nil, fmt.Errorf("failed to acquire a pid: %w", err)
	}
	return pidClean, nil
}

// Start acquires the cvmm pid file, launches cloud-hypervisor, creates and boots the VM, and reconciles virtiofsd helpers until shutdown.
func (i *Hypervisor) Start(parentCtx context.Context) error {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	pidClean, err := i.acquireManagerPidFile()
	if err != nil {
		return err
	}
	defer pidClean()

	if err := i.preparePidFile(i.cloudhypervisorPidPath, i.cloudHypervisorProcessIdentity(), "hypervisor already running"); err != nil {
		return err
	}

	ancillaryCtx, ancillaryCancel := context.WithCancel(context.Background())
	defer ancillaryCancel()

	processCtx, processCancel := context.WithCancel(context.Background())
	defer processCancel()
	vmErrorChan := make(chan error, 1)
	startedChan := make(chan *os.Process, 1)
	cmd := exec.CommandContext(processCtx, i.cloudhypervisorBinaryPath, "--api-socket", fmt.Sprintf("path=%s", i.cli.socketPath))
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return cmd.Process.Signal(syscall.SIGTERM)
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{}

	if err = sys.ApplySysProAttrPGid(cmd.SysProcAttr); err != nil {
		processCancel()
		return fmt.Errorf("failed to set process group id: %w", err)
	}

	if err = sys.ApplySysProAttrPdeathsig(cmd.SysProcAttr, syscall.SIGTERM); err != nil {
		processCancel()
		return fmt.Errorf("failed to set pdeathsig(%s): %w", syscall.SIGTERM, err)
	}

	cmd.SysProcAttr.AmbientCaps = []uintptr{unix.CAP_NET_ADMIN}
	cmd.SysProcAttr.Credential = i.runAs

	_ = os.Remove(i.cli.socketPath)

	invokeResultChan := vmErrorChan
	go func() {
		startedClosed := false
		closeStarted := func() {
			if !startedClosed {
				close(startedChan)
				startedClosed = true
			}
		}
		defer close(invokeResultChan)
		defer closeStarted()
		defer util.InfoLog.Printf("hypervisor stopped")

		invokeErr := i.invoke(cmd, i.cloudhypervisorPidPath, func(process *os.Process) {
			startedChan <- process
			closeStarted()
		})
		if invokeErr != nil {
			invokeResultChan <- fmt.Errorf("hypervisor failed: %w", invokeErr)
		}
	}()

	var hypervisorProcess *os.Process
	hypervisorStarted := func() *os.Process {
		if hypervisorProcess != nil {
			return hypervisorProcess
		}
		select {
		case process, ok := <-startedChan:
			if ok {
				hypervisorProcess = process
			}
		default:
		}
		return hypervisorProcess
	}

	waitHypervisorResult := func() error {
		if vmErrorChan == nil {
			return nil
		}
		ch := vmErrorChan
		vmErrorChan = nil
		if invokeErr, ok := <-ch; ok {
			return invokeErr
		}
		return nil
	}
	tryHypervisorResult := func() (error, bool) {
		if vmErrorChan == nil {
			return nil, true
		}
		select {
		case invokeErr, ok := <-vmErrorChan:
			vmErrorChan = nil
			if ok {
				return invokeErr, true
			}
			return nil, true
		default:
			return nil, false
		}
	}
	appendErr := func(errs []error, candidate error) []error {
		if candidate != nil {
			return append(errs, candidate)
		}
		return errs
	}
	var (
		recoilerClosedChan    chan struct{}
		statusMonitorDoneChan chan struct{}
	)
	stopAncillaries := func() {
		cancel()
		ancillaryCancel()
		if statusMonitorDoneChan != nil {
			<-statusMonitorDoneChan
			statusMonitorDoneChan = nil
		}
		if recoilerClosedChan != nil {
			<-recoilerClosedChan
			recoilerClosedChan = nil
		}
	}
	stopHypervisorImmediately := func() error {
		stopAncillaries()
		process := hypervisorStarted()
		if process == nil {
			processCancel()
			return waitHypervisorResult()
		}
		if invokeErr, done := tryHypervisorResult(); done {
			return invokeErr
		}

		processCancel()

		stopCtx, stopDone := context.WithTimeout(context.Background(), time.Second)
		defer stopDone()

		var errs []error
		waitErr := sys.WaitUntilProcessFinished(stopCtx, process.Pid)
		switch waitErr {
		case nil:
		case context.DeadlineExceeded, context.Canceled:
			if killErr := process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
				errs = append(errs, fmt.Errorf("failed to kill hypervisor process: %w", killErr))
			}
		default:
			errs = append(errs, fmt.Errorf("failed to wait for hypervisor process exit after cancellation: %w", waitErr))
			if killErr := process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
				errs = append(errs, fmt.Errorf("failed to kill hypervisor process: %w", killErr))
			}
		}

		errs = appendErr(errs, waitHypervisorResult())
		return errors.Join(errs...)
	}
	gracefulShutdown := func() error {
		defer stopAncillaries()
		var errs []error
		process := hypervisorStarted()
		if process == nil {
			return stopHypervisorImmediately()
		}
		if invokeErr, done := tryHypervisorResult(); done {
			return invokeErr
		}

		shutdownCtx, shutdownDone := context.WithTimeout(context.WithoutCancel(parentCtx), shutdownDeadline)
		defer shutdownDone()

		if powerErr := i.cli.VmPowerButton(shutdownCtx); powerErr != nil {
			switch {
			case errors.Is(powerErr, ErrVmNotCreated), errors.Is(powerErr, ErrVmNotBooted), errors.Is(powerErr, context.Canceled), errors.Is(powerErr, context.DeadlineExceeded):
			default:
				util.ErrLog.Printf("failed to request hypervisor shutdown: %v", powerErr)
			}
			return stopHypervisorImmediately()
		}
		util.InfoLog.Printf("hypervisor shutdown requested")

		waitErr := sys.WaitUntilProcessFinished(shutdownCtx, process.Pid)
		switch waitErr {
		case nil:
		case context.DeadlineExceeded, context.Canceled:
			util.ErrLog.Printf("hypervisor did not exit before shutdown deadline: %v", waitErr)
			processCancel()
			if killErr := process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
				errs = append(errs, fmt.Errorf("failed to kill hypervisor process: %w", killErr))
			}
		default:
			errs = append(errs, fmt.Errorf("failed to wait for hypervisor process exit: %w", waitErr))
			processCancel()
			if killErr := process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
				errs = append(errs, fmt.Errorf("failed to kill hypervisor process: %w", killErr))
			}
		}

		errs = appendErr(errs, waitHypervisorResult())
		return errors.Join(errs...)
	}

	waitForHypervisorReady := func() error {
		var err error
		for iter := 0; iter < availableAwaitingLimit; iter++ {
			if _, err = i.cli.VmmPing(ctx); err == nil {
				return nil
			}
			if invokeErr, done := tryHypervisorResult(); done {
				if invokeErr != nil {
					return invokeErr
				}
				return fmt.Errorf("hypervisor exited before readiness completed")
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if iter+1 >= availableAwaitingLimit {
				break
			}

			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return ctx.Err()
			case invokeErr, ok := <-vmErrorChan:
				if !timer.Stop() {
					<-timer.C
				}
				vmErrorChan = nil
				if ok {
					return invokeErr
				}
				return fmt.Errorf("hypervisor exited before readiness completed")
			case <-timer.C:
			}
		}
		return fmt.Errorf("hypervisor unavailable: %w", err)
	}

	if err := waitForHypervisorReady(); err != nil {
		if parentCtx.Err() != nil {
			return stopHypervisorImmediately()
		}
		return errors.Join(err, stopHypervisorImmediately())
	}

	process := hypervisorStarted()
	if process == nil {
		return errors.Join(fmt.Errorf("hypervisor exited before readiness completed"), waitHypervisorResult())
	}
	util.InfoLog.Printf("hypervisor started(pid: %d, i socket: %s)", process.Pid, i.cli.socketPath)

	if err := i.cli.VmCreate(ctx, i.vmcfg); err != nil {
		if parentCtx.Err() != nil {
			return gracefulShutdown()
		}
		return errors.Join(err, stopHypervisorImmediately())
	}

	recoilerClosedChan = make(chan struct{})
	go i.virtiofsdRecoiler(ancillaryCtx, recoilerClosedChan)
	statusMonitorDoneChan = make(chan struct{})
	go func() {
		defer close(statusMonitorDoneChan)
		i.hypervisorStatusMonitor(ctx)
	}()

	if err := i.cli.VmBoot(ctx); err != nil {
		if parentCtx.Err() != nil {
			return gracefulShutdown()
		}
		return errors.Join(err, stopHypervisorImmediately())
	}
	util.InfoLog.Printf("hypervisor boot")

	select {
	case <-parentCtx.Done():
		return gracefulShutdown()
	case invokeErr, ok := <-vmErrorChan:
		vmErrorChan = nil
		stopAncillaries()
		if ok {
			return invokeErr
		}
		return nil
	}
}

// OpenConsole discovers the VM console PTY from VmInfo and connects it to the current terminal.
func (i *Hypervisor) OpenConsole(parentCtx context.Context) error {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	info, err := i.cli.VmInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to open console: %w", err)
	}

	if info.Config.Console == nil {
		return fmt.Errorf("failed to open console: console PTY not available")
	}

	ptyPath := info.Config.Console.File
	if err := util.ValidateConsolePTYPath(ptyPath); err != nil {
		return fmt.Errorf("failed to open console: %w", err)
	}
	return util.OpenPty(ctx, os.Stdin, os.Stdout, ptyPath)
}

// invoke starts a child process, records its pid, streams stdout/stderr to logs, and returns contextual exit errors.
func (i *Hypervisor) invoke(cmd *exec.Cmd, pidPath string, started func(*os.Process)) error {
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

	if started != nil {
		started(cmd.Process)
	}

	res := &util.ExecutionResult{PID: cmd.Process.Pid}

	wg := &sync.WaitGroup{}
	wg.Add(2)
	go i.handleStdout(res, stdout, wg.Done)
	go i.handleStderr(res, stderr, wg.Done)

	if len(pidPath) > 0 {
		pidClean, err := sys.AcquirePidFileReplacing(pidPath, cmd.Process.Pid)
		if err != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
			waitCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if waitErr := sys.WaitUntilProcessFinished(waitCtx, cmd.Process.Pid); errors.Is(waitErr, context.DeadlineExceeded) || errors.Is(waitErr, context.Canceled) {
				_ = cmd.Process.Kill()
			}
			res.Err = cmd.Wait()
			wg.Wait()
			return fmt.Errorf("failed to start process(%s): %w", cmd.Path, err)
		}
		defer pidClean()
	}

	res.Err = cmd.Wait()
	wg.Wait()

	return res.HandleError()
}

// handleStdout copies child stdout to the info log and execution result buffer.
func (i *Hypervisor) handleStdout(res *util.ExecutionResult, reader io.Reader, closer func()) {
	defer closer()
	prefix := fmt.Sprintf("[%d] ", res.PID)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimRightFunc(scanner.Text(), unicode.IsSpace)
		util.InfoLog.Print(prefix, line)
	}
}

// handleStderr copies child stderr to the error log and execution result buffer.
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

// virtiofsdRecoiler keeps one virtiofsd process running for each configured shared directory until context cancellation.
func (i *Hypervisor) virtiofsdRecoiler(ctx context.Context, closer chan<- struct{}) {
	defer close(closer)

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

					if err = i.invoke(cmd, cfg.PidPath, nil); err != nil {
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

// waitForHypervisorAvailable polls the VMM ping endpoint until the API socket is ready or the retry limit is exhausted.
func (i *Hypervisor) waitForHypervisorAvailable(ctx context.Context) error {
	var err error
	for iter := 0; iter < availableAwaitingLimit; iter++ {
		if _, err = i.cli.VmmPing(ctx); err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if iter+1 >= availableAwaitingLimit {
			break
		}

		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}

	return fmt.Errorf("hypervisor unavailable: %w", err)
}

// hypervisorStatusMonitor logs VM state transitions while the parent context is active.
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
