package util

import (
	"testing"

	"golang.org/x/sys/unix"
)

type shortEAGAINWriter struct {
	writes int
	data   []byte
}

func (w *shortEAGAINWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes == 1 {
		w.data = append(w.data, p[0])
		return 1, unix.EAGAIN
	}
	w.data = append(w.data, p...)
	return len(p), nil
}

func TestTerminalPollFlushesPendingCopierBytesWithoutSourceEvent(t *testing.T) {
	writer := &shortEAGAINWriter{}
	copier := NewTerminalPollCopier(1, writer)
	poll := &terminalPoll{handler: map[int][]TerminalPollReader{1: {copier}}}

	if closing := copier.Handle(1, []byte("abc"), false, false); closing {
		t.Fatal("Handle() closing = true, want false after EAGAIN")
	}
	if closing := poll.flushPendingLocked(false); closing {
		t.Fatal("flushPendingLocked() closing = true, want false")
	}
	if got := string(writer.data); got != "abc" {
		t.Fatalf("writer data after pending flush = %q, want %q", got, "abc")
	}
}

func TestTerminalPollCopierRetriesPendingBytesAfterEAGAIN(t *testing.T) {
	writer := &shortEAGAINWriter{}
	copier := NewTerminalPollCopier(1, writer)

	if closing := copier.Handle(1, []byte("abc"), false, false); closing {
		t.Fatal("Handle() closing = true, want false after EAGAIN")
	}
	if got := string(writer.data); got != "a" {
		t.Fatalf("writer data after first handle = %q, want %q", got, "a")
	}

	if closing := copier.Handle(1, nil, false, false); closing {
		t.Fatal("Handle() closing = true, want false after retry")
	}
	if got := string(writer.data); got != "abc" {
		t.Fatalf("writer data after retry = %q, want %q", got, "abc")
	}
}

type alwaysEAGAINWriter struct{}

func (alwaysEAGAINWriter) Write([]byte) (int, error) { return 0, unix.EAGAIN }

func TestTerminalPollCopierKeepsPendingBytesWhenEAGAINWritesNothing(t *testing.T) {
	copier := NewTerminalPollCopier(1, alwaysEAGAINWriter{})
	if closing := copier.Handle(1, []byte("abc"), false, false); closing {
		t.Fatal("Handle() closing = true, want false")
	}

	state, ok := copier.(*simpleCopier)
	if !ok {
		t.Fatalf("copier type = %T, want *simpleCopier", copier)
	}
	if got := string(state.pending); got != "abc" {
		t.Fatalf("pending = %q, want %q", got, "abc")
	}
}
