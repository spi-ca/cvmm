package util

import (
	"bytes"
	"os"
	"testing"
)

func TestLogWriterWriteTrimsBlankLinesAndCarriageReturns(t *testing.T) {
	var buf bytes.Buffer
	oldFlags := InfoLog.Flags()
	oldPrefix := InfoLog.Prefix()
	InfoLog.SetFlags(0)
	InfoLog.SetPrefix("")
	InfoLog.SetOutput(&buf)
	defer func() {
		InfoLog.SetFlags(oldFlags)
		InfoLog.SetPrefix(oldPrefix)
		InfoLog.SetOutput(os.Stdout)
	}()

	writer := LogWriter{}
	input := []byte("\nprogress 10%\rprogress 20%\n\nfinal line\n")
	if n, err := writer.Write(input); err != nil {
		t.Fatalf("Write() error = %v", err)
	} else if n != len(input) {
		t.Fatalf("Write() n = %d, want %d", n, len(input))
	}

	if got, want := buf.String(), "progress 20%\nfinal line\n"; got != want {
		t.Fatalf("log output = %q, want %q", got, want)
	}
}
