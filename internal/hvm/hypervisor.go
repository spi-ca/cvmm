package hvm

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
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

// Hypervisor owns one node's cloud-hypervisor client, child process paths, and ancillary helper runtime configuration.
type Hypervisor struct {
	name string

	vmcfg        model.VmConfig
	virtiofsdcfg []model.VirtiofsConfig
	netBackend   model.NetBackend

	volatileBasePath          string
	pidPath                   string
	cloudhypervisorBinaryPath string
	cloudhypervisorPidPath    string
	virtiofsdBinaryPath       string
	passtBinaryPath           string
	passtcfg                  model.PasstConfig
	runAs                     *syscall.Credential
	managerUID                uint32
	managerGID                uint32

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

func (i *Hypervisor) passtProcessIdentity() sys.ProcessIdentityExpectation {
	return sys.ProcessIdentityExpectation{
		Name:               filepath.Base(i.passtBinaryPath),
		ExecutableBasename: filepath.Base(i.passtBinaryPath),
		CommandArgs:        i.passtcfg.CommandArgs(),
	}
}

func (i *Hypervisor) usesPasstNetwork() bool {
	if i.netBackend == model.NetBackendPasst {
		return true
	}
	return len(i.vmcfg.Net) > 0 && i.vmcfg.Net[0].VhostUser
}

func (i *Hypervisor) cloudHypervisorAmbientCaps() []uintptr {
	if i.usesPasstNetwork() {
		return nil
	}
	return []uintptr{unix.CAP_NET_ADMIN}
}

func (i *Hypervisor) validateRuntimeDirectoryPath() error {
	if i.volatileBasePath == "" {
		return nil
	}
	info, err := os.Lstat(i.volatileBasePath)
	if err != nil {
		return fmt.Errorf("failed to inspect runtime directory %q: %w", i.volatileBasePath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("runtime directory %q must not be a symlink", i.volatileBasePath)
	}
	if !info.IsDir() {
		return fmt.Errorf("runtime directory %q is not a directory", i.volatileBasePath)
	}
	return nil
}

func (i *Hypervisor) validatePasstServiceIdentity() error {
	if i.managerUID == 0 {
		return fmt.Errorf("passt backend requires running cvmm as a dedicated non-root service user; root manager with --runas is unsupported")
	}
	if i.runAs != nil && (i.runAs.Uid != i.managerUID || i.runAs.Gid != i.managerGID) {
		return fmt.Errorf("passt backend does not support --runas changing cloud-hypervisor away from the service uid/gid; set net.backend: tap or run without --runas")
	}
	return nil
}

func (i *Hypervisor) validatePasstRuntimeDirectory() error {
	info, err := os.Lstat(i.volatileBasePath)
	if err != nil {
		return fmt.Errorf("failed to inspect passt runtime directory %q: %w", i.volatileBasePath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("passt runtime directory %q must not be a symlink", i.volatileBasePath)
	}
	if !info.IsDir() {
		return fmt.Errorf("passt runtime directory %q is not a directory", i.volatileBasePath)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("failed to inspect passt runtime directory %q ownership", i.volatileBasePath)
	}
	if stat.Uid != i.managerUID {
		return fmt.Errorf("passt runtime directory %q must be owned by service uid %d", i.volatileBasePath, i.managerUID)
	}
	if perm := info.Mode().Perm(); perm > 0o700 {
		return fmt.Errorf("passt runtime directory %q must not be more permissive than 0700 (found %04o)", i.volatileBasePath, perm)
	}
	return nil
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

// Start acquires the cvmm pid file, launches cloud-hypervisor, creates and boots the VM, and reconciles helpers until shutdown.
func (i *Hypervisor) Start(parentCtx context.Context) error {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	if err := i.validateRuntimeDirectoryPath(); err != nil {
		return err
	}
	if i.usesPasstNetwork() {
		if err := i.validatePasstServiceIdentity(); err != nil {
			return err
		}
		if err := i.validatePasstRuntimeDirectory(); err != nil {
			return err
		}
	}

	pidClean, err := i.acquireManagerPidFile()
	if err != nil {
		return err
	}
	defer pidClean()

	if err := i.preparePidFile(i.cloudhypervisorPidPath, i.cloudHypervisorProcessIdentity(), "hypervisor already running"); err != nil {
		return err
	}
	if i.usesPasstNetwork() {
		if err := i.preparePidFile(i.passtcfg.PidPath, i.passtProcessIdentity(), "passt already running"); err != nil {
			return err
		}
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
	cmd.SysProcAttr.AmbientCaps = i.cloudHypervisorAmbientCaps()
	cmd.SysProcAttr.Credential = i.runAs

	_ = os.Remove(i.cli.socketPath)
	go func() {
		startedClosed := false
		closeStarted := func() {
			if !startedClosed {
				close(startedChan)
				startedClosed = true
			}
		}
		defer close(vmErrorChan)
		defer closeStarted()
		defer util.InfoLog.Printf("hypervisor stopped")

		invokeErr := i.invoke(cmd, i.cloudhypervisorPidPath, func(process *os.Process) {
			startedChan <- process
			closeStarted()
		})
		if invokeErr != nil {
			vmErrorChan <- fmt.Errorf("hypervisor failed: %w", invokeErr)
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
		passtErrorChan        chan error
		passtStartedChan      chan *os.Process
		passtProcess          *os.Process
		passtProcessCancel    context.CancelFunc
		recoilerClosedChan    chan struct{}
		statusMonitorDoneChan chan struct{}
	)
	if i.usesPasstNetwork() {
		passtCtx, cancelPasst := context.WithCancel(context.Background())
		passtProcessCancel = cancelPasst
		passtErrorChan = make(chan error, 1)
		passtStartedChan = make(chan *os.Process, 1)
		passtCmd := exec.CommandContext(passtCtx, i.passtBinaryPath, i.passtcfg.CommandArgs()...)
		passtCmd.Cancel = func() error {
			if passtCmd.Process == nil {
				return nil
			}
			return passtCmd.Process.Signal(syscall.SIGTERM)
		}
		passtCmd.SysProcAttr = &syscall.SysProcAttr{}
		if err = sys.ApplySysProAttrPGid(passtCmd.SysProcAttr); err != nil {
			cancelPasst()
			return fmt.Errorf("failed to set passt process group id: %w", err)
		}
		if err = sys.ApplySysProAttrPdeathsig(passtCmd.SysProcAttr, syscall.SIGTERM); err != nil {
			cancelPasst()
			return fmt.Errorf("failed to set passt pdeathsig(%s): %w", syscall.SIGTERM, err)
		}

		_ = os.Remove(i.passtcfg.SocketPath)
		go func() {
			startedClosed := false
			closeStarted := func() {
				if !startedClosed {
					close(passtStartedChan)
					startedClosed = true
				}
			}
			defer close(passtErrorChan)
			defer closeStarted()
			defer util.InfoLog.Printf("passt stopped")

			invokeErr := i.invoke(passtCmd, i.passtcfg.PidPath, func(process *os.Process) {
				passtStartedChan <- process
				closeStarted()
			})
			if invokeErr != nil {
				passtErrorChan <- fmt.Errorf("passt failed: %w", invokeErr)
			}
		}()
	}
	passtStarted := func() *os.Process {
		if passtProcess != nil || passtStartedChan == nil {
			return passtProcess
		}
		select {
		case process, ok := <-passtStartedChan:
			if ok {
				passtProcess = process
			}
		default:
		}
		return passtProcess
	}
	waitPasstResult := func() error {
		if passtErrorChan == nil {
			return nil
		}
		ch := passtErrorChan
		passtErrorChan = nil
		if invokeErr, ok := <-ch; ok {
			return invokeErr
		}
		return nil
	}
	tryPasstResult := func() (error, bool) {
		if passtErrorChan == nil {
			return nil, true
		}
		select {
		case invokeErr, ok := <-passtErrorChan:
			passtErrorChan = nil
			if ok {
				return invokeErr, true
			}
			return nil, true
		default:
			return nil, false
		}
	}

	stopAncillaries := func() {
		cancel()
		ancillaryCancel()
		if passtProcessCancel != nil {
			passtProcessCancel()
		}
		if process := passtStarted(); process != nil {
			_ = process.Signal(syscall.SIGTERM)
			stopCtx, stopDone := context.WithTimeout(context.Background(), time.Second)
			waitErr := sys.WaitUntilProcessFinished(stopCtx, process.Pid)
			stopDone()
			if errors.Is(waitErr, context.DeadlineExceeded) || errors.Is(waitErr, context.Canceled) {
				_ = process.Kill()
			}
		}
		if statusMonitorDoneChan != nil {
			<-statusMonitorDoneChan
			statusMonitorDoneChan = nil
		}
		if recoilerClosedChan != nil {
			<-recoilerClosedChan
			recoilerClosedChan = nil
		}
		if passtErrorChan != nil {
			select {
			case <-passtErrorChan:
				passtErrorChan = nil
			case <-time.After(time.Second):
				if process := passtStarted(); process != nil {
					_ = process.Kill()
				}
				passtErrorChan = nil
			}
		}
		_ = os.Remove(i.passtcfg.SocketPath)
		_ = os.Remove(i.passtcfg.PidPath)
	}
	stopHypervisorImmediately := func() error {
		stopAncillaries()
		process := hypervisorStarted()
		if process == nil {
			processCancel()
			if vmErrorChan != nil {
				select {
				case invokeErr, ok := <-vmErrorChan:
					vmErrorChan = nil
					if ok {
						return errors.Join(invokeErr, waitPasstResult())
					}
					return waitPasstResult()
				case <-time.After(250 * time.Millisecond):
				}
			}
			return errors.Join(waitHypervisorResult(), waitPasstResult())
		}
		if invokeErr, done := tryHypervisorResult(); done {
			return errors.Join(invokeErr, waitPasstResult())
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
		errs = appendErr(errs, waitPasstResult())
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
	fatalPasstExit := func(passtErr error) error {
		if passtErr == nil {
			passtErr = fmt.Errorf("passt exited unexpectedly")
		}
		return errors.Join(fmt.Errorf("passt failed after vm.create: %w", passtErr), gracefulShutdown())
	}

	waitForHypervisorReady := func() error {
		var lastErr error
		for iter := 0; iter < availableAwaitingLimit; iter++ {
			if _, lastErr = i.cli.VmmPing(ctx); lastErr == nil {
				return nil
			}
			if invokeErr, done := tryHypervisorResult(); done {
				if invokeErr != nil {
					return invokeErr
				}
				return fmt.Errorf("hypervisor exited before readiness completed")
			}
			if passtErr, done := tryPasstResult(); done && i.usesPasstNetwork() {
				if passtErr != nil {
					return passtErr
				}
				return fmt.Errorf("passt exited before socket readiness completed")
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
			case passtErr, ok := <-passtErrorChan:
				if !timer.Stop() {
					<-timer.C
				}
				passtErrorChan = nil
				if ok {
					return passtErr
				}
				return fmt.Errorf("passt exited before socket readiness completed")
			case <-timer.C:
			}
		}
		return fmt.Errorf("hypervisor unavailable: %w", lastErr)
	}

	if err := waitForHypervisorReady(); err != nil {
		if parentCtx.Err() != nil {
			return stopHypervisorImmediately()
		}
		return errors.Join(err, stopHypervisorImmediately())
	}
	if i.usesPasstNetwork() {
		if err := waitForUnixSocketReady(ctx, i.passtcfg.SocketPath, tryPasstResult); err != nil {
			if parentCtx.Err() != nil {
				return stopHypervisorImmediately()
			}
			return errors.Join(err, stopHypervisorImmediately())
		}
	}

	process := hypervisorStarted()
	if process == nil {
		return errors.Join(fmt.Errorf("hypervisor exited before readiness completed"), waitHypervisorResult())
	}
	util.InfoLog.Printf("hypervisor started(pid: %d, i socket: %s)", process.Pid, i.cli.socketPath)
	if passtProcess := passtStarted(); passtProcess != nil {
		util.InfoLog.Printf("passt started(pid: %d, socket: %s)", passtProcess.Pid, i.passtcfg.SocketPath)
	}

	sharedMemory := i.vmcfg.Memory != nil && i.vmcfg.Memory.Shared
	thpDecision := hostTHPProbe(sharedMemory)
	for _, warning := range thpDecision.warnings {
		util.ErrLog.Printf("memory THP warning: %s", warning)
	}
	util.InfoLog.Printf("memory THP decision(%s): %s", thpDecision.state(), thpDecision.reason)

	vmCreateCfg := applyTHPDecision(i.vmcfg, thpDecision.enabled)
	logVmCreatePayload("vm.create payload", vmCreateCfg)

	if err := i.cli.VmCreate(ctx, vmCreateCfg); err != nil {
		if parentCtx.Err() != nil {
			return gracefulShutdown()
		}
		if thpDecision.enabled && ctx.Err() == nil && isTHPRelatedVmCreateError(err) {
			retryCfg := applyTHPDecision(i.vmcfg, false)
			util.ErrLog.Printf("vm.create THP-enabled request failed, retrying with THP disabled: %v", err)
			util.InfoLog.Printf("memory THP decision(disabled): retry fallback after THP-related vm.create failure")
			logVmCreatePayload("vm.create retry payload", retryCfg)
			if retryErr := i.cli.VmCreate(ctx, retryCfg); retryErr == nil {
				util.InfoLog.Printf("vm.create THP retry succeeded")
			} else {
				return errors.Join(err, fmt.Errorf("vm.create THP-disabled retry failed: %w", retryErr), stopHypervisorImmediately())
			}
		} else {
			return errors.Join(err, stopHypervisorImmediately())
		}
	}

	recoilerClosedChan = make(chan struct{})
	go i.virtiofsdRecoiler(ancillaryCtx, recoilerClosedChan)
	statusMonitorDoneChan = make(chan struct{})
	go func() {
		defer close(statusMonitorDoneChan)
		i.hypervisorStatusMonitor(ctx)
	}()

	bootResultChan := make(chan error, 1)
	go func() { bootResultChan <- i.cli.VmBoot(ctx) }()
	select {
	case bootErr := <-bootResultChan:
		if bootErr != nil {
			if parentCtx.Err() != nil {
				return gracefulShutdown()
			}
			return errors.Join(bootErr, stopHypervisorImmediately())
		}
	case <-parentCtx.Done():
		return gracefulShutdown()
	case invokeErr, ok := <-vmErrorChan:
		vmErrorChan = nil
		stopAncillaries()
		if ok {
			return invokeErr
		}
		return nil
	case passtErr, ok := <-passtErrorChan:
		passtErrorChan = nil
		if !ok {
			return fatalPasstExit(nil)
		}
		return fatalPasstExit(passtErr)
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
	case passtErr, ok := <-passtErrorChan:
		passtErrorChan = nil
		if !ok {
			return fatalPasstExit(nil)
		}
		return fatalPasstExit(passtErr)
	}
}

func logVmCreatePayload(label string, cfg model.VmConfig) {
	payload, err := json.Marshal(cfg.Memory)
	if err != nil {
		util.ErrLog.Printf("%s memory JSON marshal failed: %v", label, err)
		return
	}
	util.InfoLog.Printf("%s memory JSON: %s", label, payload)
}

func waitForUnixSocketReady(ctx context.Context, path string, tryResult func() (error, bool)) error {
	deadline := time.Now().Add(availableAwaitingLimit * time.Second)
	var lastErr error
	for {
		if err := unixSocketReady(path); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if invokeErr, done := tryResult(); done {
			if invokeErr != nil {
				return invokeErr
			}
			return fmt.Errorf("helper exited before socket readiness completed")
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if time.Now().After(deadline) {
			break
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
	return fmt.Errorf("socket unavailable: %w", lastErr)
}

func unixSocketReady(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("%q is not a unix socket", path)
	}
	conn, err := net.DialTimeout("unix", path, 100*time.Millisecond)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
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
			name := fmt.Sprintf("virtiofsd-%s", filepath.Base(cfg.Directory))
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
						util.ErrLog.Printf("%s endpoint unavailable", name)
					} else if from == gobreaker.StateHalfOpen && to == gobreaker.StateClosed {
						util.ErrLog.Printf("%s endpoint is returning available", name)
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
