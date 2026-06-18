//go:build linux
// +build linux

package sys

import (
	"syscall"
)

// ApplySysProAttrIsolation requests new mount, UTS, IPC, and filesystem namespaces for the child process.
func ApplySysProAttrIsolation(attr *syscall.SysProcAttr) error {
	attr.Unshareflags |= syscall.CLONE_NEWNS | syscall.CLONE_NEWUTS | syscall.CLONE_NEWIPC | syscall.CLONE_FS
	return nil
}

// ApplySysProAttrPGid starts the child in a new process group.
func ApplySysProAttrPGid(attr *syscall.SysProcAttr) error {
	attr.Setpgid = true
	return nil
}

// ApplySysProAttrSid starts the child in a new session.
func ApplySysProAttrSid(attr *syscall.SysProcAttr) error {
	attr.Setsid = true
	return nil
}

// ApplySysProAttrPdeathsig configures the signal delivered to the child when its parent dies.
func ApplySysProAttrPdeathsig(attr *syscall.SysProcAttr, pdeathsig syscall.Signal) error {
	attr.Pdeathsig = pdeathsig
	return nil
}
