package util

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestValidateConsolePTYPathRejectsInvalidPaths(t *testing.T) {
	invalidFile := filepath.Join(t.TempDir(), "not-a-pty")
	if err := os.WriteFile(invalidFile, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	for _, p := range []string{"", invalidFile, "/dev/null", "/dev/pts/not-a-number", "/dev/pts/../1"} {
		if err := ValidateConsolePTYPath(p); err == nil {
			t.Fatalf("ValidateConsolePTYPath(%q) error = nil, want error", p)
		}
	}
}

func TestValidateConsolePTYPathAcceptsAllocatedPTY(t *testing.T) {
	master, slave, ptyPath := allocateValidationTestPTY(t)
	defer master.Close()
	defer slave.Close()

	if err := ValidateConsolePTYPath(ptyPath); err != nil {
		t.Fatalf("ValidateConsolePTYPath(%q) error = %v", ptyPath, err)
	}
}

func TestValidateDirectConsolePTYPathAcceptsCurrentUserPTY(t *testing.T) {
	master, slave, ptyPath := allocateValidationTestPTY(t)
	defer master.Close()
	defer slave.Close()

	if err := ValidateDirectConsolePTYPath(ptyPath); err != nil {
		t.Fatalf("ValidateDirectConsolePTYPath(%q) error = %v", ptyPath, err)
	}
}

func TestValidateConsolePTYPathInfoRejectsForeignOwnerForDirectAttach(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("owner restriction is bypassed for root")
	}

	info := fakePTYFileInfo{
		mode: os.ModeDevice | os.ModeCharDevice,
		sys:  &syscall.Stat_t{Uid: uint32(os.Geteuid() + 1)},
	}
	if err := validateConsolePTYPathInfo("/dev/pts/7", info, os.Geteuid(), true); err == nil {
		t.Fatal("validateConsolePTYPathInfo() error = nil, want owner rejection")
	}
}

func allocateValidationTestPTY(t *testing.T) (*os.File, *os.File, string) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("PTY allocation helper uses Linux ptmx ioctls")
	}

	masterFD, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Skipf("open /dev/ptmx: %v", err)
	}
	master := os.NewFile(uintptr(masterFD), "/dev/ptmx")
	if err := unix.IoctlSetPointerInt(masterFD, unix.TIOCSPTLCK, 0); err != nil {
		_ = master.Close()
		t.Skipf("unlock PTY: %v", err)
	}
	ptyID, err := unix.IoctlGetInt(masterFD, unix.TIOCGPTN)
	if err != nil {
		_ = master.Close()
		t.Skipf("lookup PTY number: %v", err)
	}

	ptyPath := filepath.Join("/dev/pts", fmt.Sprintf("%d", ptyID))
	slave, err := os.OpenFile(ptyPath, os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		_ = master.Close()
		t.Skipf("open slave PTY: %v", err)
	}
	return master, slave, ptyPath
}

type fakePTYFileInfo struct {
	mode os.FileMode
	sys  any
}

func (f fakePTYFileInfo) Name() string       { return "7" }
func (f fakePTYFileInfo) Size() int64        { return 0 }
func (f fakePTYFileInfo) Mode() os.FileMode  { return f.mode }
func (f fakePTYFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakePTYFileInfo) IsDir() bool        { return false }
func (f fakePTYFileInfo) Sys() any           { return f.sys }
