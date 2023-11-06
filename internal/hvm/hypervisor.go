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

	"amuz.es/src/spi-ca/chmgr/internal/model"
	"amuz.es/src/spi-ca/chmgr/internal/util"
	"amuz.es/src/spi-ca/chmgr/internal/util/sys"
	"gopkg.in/yaml.v3"
)

type Hypervisor struct {
	name              string `yaml:"-"`
	imageRoot         string `yaml:"-"`
	nodeHome          string `yaml:"-"`
	volatileDirectory string `yaml:"-"`

	args                      model.Config
	cloudhypervisorBinaryPath string

	cli       *clientImpl
	virtiofsd virtiofsdJoiner
}

func (i *Hypervisor) load(manifestPath string) error {
	f, err := os.Open(manifestPath)
	if err != nil {
		return err
	}

	defer func() { _ = f.Close() }()

	d := yaml.NewDecoder(f)
	err = d.Decode(i)
	if err != nil {
		return err
	}

	return nil
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

func (i *Hypervisor) Close()            { i.cli.Close() }
func (i *Hypervisor) GetClient() Client { return i.cli }
func (i *Hypervisor) ImageBasePath(rest string) string {
	return filepath.Join(i.imageRoot, i.args.Image, rest)
}

func (i *Hypervisor) NodeBasePath(rest string) string {
	return filepath.Join(i.nodeHome, rest)
}

func (i *Hypervisor) VolatilePath(rest string) string {
	return filepath.Join(i.volatileDirectory, rest)
}

func (i *Hypervisor) Start(
	parentCtx context.Context,
	kernelFilename, initramfsFilename, rootfsFilename string,
	virtiofsFilenameTemplate func(string) string,
) error {

	virtiofsSocketPathResolver := func(name string) string { return i.VolatilePath(virtiofsFilenameTemplate(name)) }
	vmcfg := i.args.VMConfig(
		i.name,
		i.ImageBasePath(kernelFilename), i.ImageBasePath(initramfsFilename),
		i.ImageBasePath(rootfsFilename), i.NodeBasePath,
		virtiofsSocketPathResolver,
	)
	util.InfoLog.Printf("hypervisor config: %s", vmcfg)

	virtiofscfgs := i.args.VirtiofsConfig(i.NodeBasePath, virtiofsSocketPathResolver)

	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	vmErrorChan := make(chan error, 1)
	go i.submit(ctx, vmErrorChan, "--api-socket", fmt.Sprintf("path=%s", i.cli.socketPath))
	util.InfoLog.Printf("hypervisor started(api socket: %s)", i.cli.socketPath)

	virtiofsdErrorChan := i.virtiofsd.Execute(ctx, virtiofscfgs)
	if virtiofsdErrorChan != nil {
		util.InfoLog.Printf("virtiofsd started(#%d instnaces)", len(virtiofscfgs))
	}

	var errs []error

	select {
	case err, ok := <-virtiofsdErrorChan:
		if ok {
			errs = append(errs, err)
		}
		util.InfoLog.Printf("virtiofsd stopped")

		// wait hypervisor
		if err, ok = <-vmErrorChan; ok {
			errs = append(errs, err)
		}
		util.InfoLog.Printf("hypervisor stopped")
	case err, ok := <-vmErrorChan:
		if ok {
			errs = append(errs, err)
		}
		util.InfoLog.Printf("hypervisor stopped")
	}

	//remain virtiofsd errors
	if virtiofsdErrorChan != nil {
		for selectorErr := range virtiofsdErrorChan {
			util.InfoLog.Printf("virtiofsd stopped")
			errs = append(errs, selectorErr)
		}
	}

	return errors.Join(errs...)
}

//func (i *Hypervisor) Boot(
//	ctx context.Context,
//	kernelFilename,
//	initramfsFilename,
//	rootfsFilename,
//	apiSocketFilename string,
//) chan<- error {
//	apiSocketFilepath := i.VolatilePath(apiSocketFilename)
//	//vmcfg := i.args.VMConfig(
//	//	i.name,
//	//	i.ImageBasePath(kernelFilename), i.ImageBasePath(initramfsFilename),
//	//	i.ImageBasePath(rootfsFilename), i.NodeBasePath,
//	//	i.VirtiofsSocketPath,
//	//)
//	//// virtiofscfg
//	//virtiofscfgs := i.args.VirtiofsConfig(i.NodeBasePath, i.VirtiofsSocketPath)
//
//	args := []string{
//		"--api-socket", fmt.Sprintf("path=%s", apiSocketFilepath),
//	}
//
//	vmErrorChan := make(chan error)
//	go i.submit(ctx, args, vmErrorChan)
//
//	// daemon started
//	return vmErrorChan
//}

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

func (i *Hypervisor) submit(ctx context.Context, errorChan chan<- error, args ...string) {
	defer func() {
		if err := recover(); err != nil {
			util.ErrLog.Printf("cloud-hypervisor panic: %v", err)
		}
		close(errorChan)
	}()

	started := time.Now()
	err := i.invoke(ctx, args)
	ended := time.Now()
	if err != nil {
		errorChan <- fmt.Errorf("cloud-hypervisor failed in %s: %w", ended.Sub(started), err)
	}
}

func (i *Hypervisor) invoke(parentContext context.Context, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	invoke := exec.CommandContext(ctx, i.cloudhypervisorBinaryPath, args...)
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
		return fmt.Errorf("failed to start process(%s): %w", i.cloudhypervisorBinaryPath, err)
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
