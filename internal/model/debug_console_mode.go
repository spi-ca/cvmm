package model

import (
	"errors"
)

// DebugConsoleMode is an enum-like value used when translating cvmm input or cloud-hypervisor state.
type DebugConsoleMode string

const (
	// DebugConsoleModeInvalid is the zero value used when parsing fails.
	DebugConsoleModeInvalid DebugConsoleMode = ""
	// DebugConsoleModeOff disables the debug console.
	DebugConsoleModeOff DebugConsoleMode = "Off"
	// DebugConsoleModePty exposes the debug console through a PTY.
	DebugConsoleModePty DebugConsoleMode = "Pty"
	// DebugConsoleModeTty attaches the debug console to the process TTY.
	DebugConsoleModeTty DebugConsoleMode = "Tty"
	// DebugConsoleModeFile writes the debug console to a file.
	DebugConsoleModeFile DebugConsoleMode = "File"
	// DebugConsoleModeNull discards debug console output.
	DebugConsoleModeNull DebugConsoleMode = "Null"
)

// DebugConsoleModeNameOf parses a cloud-hypervisor debug console mode name.
func DebugConsoleModeNameOf(value string) (DebugConsoleMode, error) {
	switch value {
	case string(DebugConsoleModeOff):
		return DebugConsoleModeOff, nil
	case string(DebugConsoleModePty):
		return DebugConsoleModePty, nil
	case string(DebugConsoleModeTty):
		return DebugConsoleModeTty, nil
	case string(DebugConsoleModeFile):
		return DebugConsoleModeFile, nil
	case string(DebugConsoleModeNull):
		return DebugConsoleModeNull, nil
	default:
		return DebugConsoleModeInvalid, errors.New("unable to find DebugConsoleMode")
	}
}

// IsValid reports whether the receiver is one of the supported values.
func (ts DebugConsoleMode) IsValid() bool {
	switch ts {
	case DebugConsoleModeOff:
		fallthrough
	case DebugConsoleModePty:
		fallthrough
	case DebugConsoleModeTty:
		fallthrough
	case DebugConsoleModeFile:
		fallthrough
	case DebugConsoleModeNull:
		return true
	default:
		return false
	}
}

// String returns the cloud-hypervisor text token for a valid DebugConsoleMode.
func (ts DebugConsoleMode) String() string {
	if ts.IsValid() {
		return string(ts)
	} else {
		return ""
	}
}

// MarshalText returns the textual representation used by encoders.
func (ts DebugConsoleMode) MarshalText() ([]byte, error) { return []byte(ts.String()), nil }

// UnmarshalText parses a textual value into the receiver.
func (ts *DebugConsoleMode) UnmarshalText(b []byte) error {
	length := len(b)
	if length < 0 {
		return errors.New("malformed format")
	}

	new_status, err := DebugConsoleModeNameOf(string(b))
	if err != nil {
		return err
	}

	*ts = new_status
	return nil
}
