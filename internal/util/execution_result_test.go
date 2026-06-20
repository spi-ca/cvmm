package util

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"testing"
)

func TestExecutionResultRingBufferKeepsNewestLogs(t *testing.T) {
	res := &ExecutionResult{}
	for i := 1; i <= 12; i++ {
		res.AppendLogLine(fmt.Sprintf("line%02d", i))
	}

	got := res.LastLogLine()
	want := []string{"line03", "line04", "line05", "line06", "line07", "line08", "line09", "line10", "line11", "line12"}
	if len(got) != len(want) {
		t.Fatalf("LastLogLine() len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("LastLogLine()[%d] = %q, want %q (all=%v)", i, got[i], want[i], got)
		}
	}
}

func TestExecutionResultHandleErrorReturnsNilWhenCanceled(t *testing.T) {
	oldErrOut := ErrLog.Writer()
	ErrLog.SetOutput(io.Discard)
	defer ErrLog.SetOutput(oldErrOut)

	res := &ExecutionResult{Err: context.Canceled}
	res.AppendLogLine("canceled")

	if err := res.HandleError(); err != nil {
		t.Fatalf("HandleError() error = %v, want nil", err)
	}
	if res.Err != nil {
		t.Fatalf("HandleError() stored err = %v, want nil", res.Err)
	}
}

func TestExecutionResultHandleErrorFormatsExitErrorsWithLogs(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exit 7")
	res := &ExecutionResult{}
	res.AppendLogLine("stderr one")
	res.AppendLogLine("stderr two")
	res.Err = cmd.Run()
	if res.Err == nil {
		t.Fatal("cmd.Run() error = nil, want exit error")
	}

	err := res.HandleError()
	if err == nil {
		t.Fatal("HandleError() error = nil, want exit error")
	}
	if !strings.Contains(err.Error(), "exit status 7") {
		t.Fatalf("HandleError() error = %v, want exit status", err)
	}
	if !strings.Contains(err.Error(), "stderr one") || !strings.Contains(err.Error(), "stderr two") {
		t.Fatalf("HandleError() error = %v, want captured log lines", err)
	}
}

func TestExecutionResultHandleErrorReturnsGeneralErrorsWithLogs(t *testing.T) {
	res := &ExecutionResult{Err: errors.New("boom")}
	res.AppendLogLine("stderr tail")

	err := res.HandleError()
	if err == nil {
		t.Fatal("HandleError() error = nil, want wrapped error")
	}
	if !strings.Contains(err.Error(), "boom") || !strings.Contains(err.Error(), "stderr tail") {
		t.Fatalf("HandleError() error = %v, want wrapped error and logs", err)
	}
}

func TestExecutionResultHandleErrorReturnsNilWhenSuccessful(t *testing.T) {
	oldErrOut := ErrLog.Writer()
	ErrLog.SetOutput(io.Discard)
	defer ErrLog.SetOutput(oldErrOut)

	res := &ExecutionResult{}
	res.AppendLogLine("ok")
	if err := res.HandleError(); err != nil {
		t.Fatalf("HandleError() error = %v, want nil", err)
	}
}
