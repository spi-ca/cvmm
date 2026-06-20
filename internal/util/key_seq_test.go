package util

import (
	"bytes"
	"testing"
)

func TestCaptureEscapeKeySequenceStopsOnEscapeParen(t *testing.T) {
	var out bytes.Buffer
	CaptureEscapeKeySequence(bytes.NewBufferString("hello\x1b[12D\x1b("), &out)

	if got, want := out.String(), "hello\x1b[12D\x1b"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestCaptureEscapeKeySequenceReturnsOnEOF(t *testing.T) {
	var out bytes.Buffer
	CaptureEscapeKeySequence(bytes.NewBufferString("ab\x1bX"), &out)

	if got, want := out.String(), "ab\x1bX"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}
