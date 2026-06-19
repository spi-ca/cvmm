//go:build linux

package sys

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestWaitUntilProcessFinishedWithProcReturnsWhenProcessExits(t *testing.T) {
	helperPath := buildProcTestHelper(t)
	cmd := exec.Command(helperPath)
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start helper: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = cmd.Process.Kill()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := waitUntilProcessFinishedWithProc(ctx, cmd.Process.Pid); err != nil {
		t.Fatalf("waitUntilProcessFinishedWithProc() error = %v", err)
	}
}
