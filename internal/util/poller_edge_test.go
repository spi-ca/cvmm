package util

import (
	"os"
	"testing"
)

func TestTerminalPollAddRollsBackWhenAnyFDRegistrationFails(t *testing.T) {
	pollAny, err := NewTerminalPoll()
	if err != nil {
		t.Fatalf("NewTerminalPoll() error = %v", err)
	}
	poll := pollAny.(*terminalPoll)
	defer func() { _ = poll.Close() }()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	err = poll.Add(int(reader.Fd()), -1)
	if err == nil {
		t.Fatal("Add() error = nil, want rollback on invalid fd")
	}
	if _, ok := poll.handler[int(reader.Fd())]; ok {
		t.Fatal("Add() left successfully-added fd registered after rollback")
	}
}

func TestTerminalPollRemoveDeletesRegisteredHandlers(t *testing.T) {
	pollAny, err := NewTerminalPoll()
	if err != nil {
		t.Fatalf("NewTerminalPoll() error = %v", err)
	}
	poll := pollAny.(*terminalPoll)
	defer func() { _ = poll.Close() }()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	if err := poll.Add(int(reader.Fd())); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if err := poll.Register(NewEscapeHandler(int(reader.Fd()))); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if len(poll.handler[int(reader.Fd())]) != 1 {
		t.Fatalf("registered handlers = %d, want 1", len(poll.handler[int(reader.Fd())]))
	}

	if err := poll.Remove(int(reader.Fd())); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if _, ok := poll.handler[int(reader.Fd())]; ok {
		t.Fatal("Remove() left handler entry behind")
	}
}

func TestTerminalPollClosedOperationsReturnErrors(t *testing.T) {
	pollAny, err := NewTerminalPoll()
	if err != nil {
		t.Fatalf("NewTerminalPoll() error = %v", err)
	}
	poll := pollAny.(*terminalPoll)
	if err := poll.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if err := poll.Add(1); err == nil {
		t.Fatal("Add() error = nil on closed poller")
	}
	if err := poll.Remove(1); err == nil {
		t.Fatal("Remove() error = nil on closed poller")
	}
	if err := poll.Register(NewEscapeHandler(1)); err == nil {
		t.Fatal("Register() error = nil on closed poller")
	}
}
