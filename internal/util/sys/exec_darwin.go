//go:build darwin
// +build darwin

package sys

import (
	"fmt"
	"runtime"
	"syscall"
)

// ApplySysProAttrIsolation reports that namespace isolation is not supported on Darwin.
func ApplySysProAttrIsolation(attr *syscall.SysProcAttr) error {
	return fmt.Errorf("this os(%s) not supported", runtime.GOOS)
}

// ApplySysProAttrPGid starts the child in a new process group on Darwin.
func ApplySysProAttrPGid(attr *syscall.SysProcAttr) error {
	attr.Setpgid = true
	return nil
}

// ApplySysProAttrSid starts the child in a new session on Darwin.
func ApplySysProAttrSid(attr *syscall.SysProcAttr) error {
	attr.Setsid = true
	return nil
}

// ApplySysProAttrPdeathsig leaves parent-death signal configuration unchanged on Darwin because SysProcAttr has no matching field.
func ApplySysProAttrPdeathsig(attr *syscall.SysProcAttr, pdeathsig syscall.Signal) error {
	return nil
}
