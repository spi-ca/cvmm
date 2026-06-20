package entry

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestConsolePtyPathBuildsDevPtsPath(t *testing.T) {
	if got, want := consolePtyPath(42), "/dev/pts/42"; got != want {
		t.Fatalf("consolePtyPath() = %q, want %q", got, want)
	}
}

func TestConsoleFilePanicsWhenOpenPtyFailsAfterValidation(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux PTY allocation helper required")
	}

	master, slave, ptyPath := allocateEntryTestPTY(t)
	defer master.Close()
	defer slave.Close()

	ptyID, err := strconv.Atoi(filepath.Base(ptyPath))
	if err != nil {
		t.Fatalf("invalid PTY path %q: %v", ptyPath, err)
	}

	openErr := errors.New("deterministic open failure after validation")
	oldOpenPty := consoleFileOpenPty
	consoleFileOpenPty = func(ctx context.Context, input *os.File, output *os.File, gotPath string) error {
		if gotPath != ptyPath {
			t.Fatalf("consoleFileOpenPty path = %q, want %q", gotPath, ptyPath)
		}
		return openErr
	}
	defer func() { consoleFileOpenPty = oldOpenPty }()

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("ConsoleFile() panic = nil, want OpenPty failure panic")
		}
		if !strings.Contains(recovered.(error).Error(), openErr.Error()) {
			t.Fatalf("ConsoleFile() panic = %v, want OpenPty failure context", recovered)
		}
	}()

	ConsoleFile("cvmm", ptyID)
}
