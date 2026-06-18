//go:build !linux && !darwin
// +build !linux,!darwin

package sys

import (
	"fmt"
	"runtime"
	"syscall"
)

// ApplySysProAttrIsolation reports unsupported namespace isolation on this platform.
func ApplySysProAttrIsolation(attr *syscall.SysProcAttr) error {
	return fmt.Errorf("this os(%s) not supported", runtime.GOOS)
}

// ApplySysProAttrPGid starts the child in a new process group when SysProcAttr supports Setpgid.
func ApplySysProAttrPGid(attr *syscall.SysProcAttr) error {
	attr.Setpgid = true
	return nil
}

// ApplySysProAttrSid starts the child in a new session when SysProcAttr supports Setsid.
func ApplySysProAttrSid(attr *syscall.SysProcAttr) error {
	attr.Setsid = true
	return nil
}

// ApplySysProAttrPdeathsig reports unsupported parent-death signal configuration on this platform.
func ApplySysProAttrPdeathsig(attr *syscall.SysProcAttr) error {
	return fmt.Errorf("this os(%s) not supported", runtime.GOOS)
}
