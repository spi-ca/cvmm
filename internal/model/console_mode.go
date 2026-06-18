package model

import (
	"errors"
)

// ConsoleMode is an enum-like value used when translating cvmm input or cloud-hypervisor state.
type ConsoleMode string

const (
	// ConsoleModeInvalid is the zero value used when parsing fails.
	ConsoleModeInvalid ConsoleMode = ""
	// ConsoleModeOff disables the console.
	ConsoleModeOff ConsoleMode = "Off"
	// ConsoleModePty exposes the console through a PTY.
	ConsoleModePty ConsoleMode = "Pty"
	// ConsoleModeTty attaches the console to the process TTY.
	ConsoleModeTty ConsoleMode = "Tty"
	// ConsoleModeFile writes the console to a file.
	ConsoleModeFile ConsoleMode = "File"
	// ConsoleModeSocket exposes the console through a socket.
	ConsoleModeSocket ConsoleMode = "Socket"
	// ConsoleModeNull discards console output.
	ConsoleModeNull ConsoleMode = "Null"
)

// ConsoleModeNameOf parses a cloud-hypervisor console mode name.
func ConsoleModeNameOf(value string) (ConsoleMode, error) {
	switch value {
	case string(ConsoleModeOff):
		return ConsoleModeOff, nil
	case string(ConsoleModePty):
		return ConsoleModePty, nil
	case string(ConsoleModeTty):
		return ConsoleModeTty, nil
	case string(ConsoleModeFile):
		return ConsoleModeFile, nil
	case string(ConsoleModeSocket):
		return ConsoleModeSocket, nil
	case string(ConsoleModeNull):
		return ConsoleModeNull, nil
	default:
		return ConsoleModeInvalid, errors.New("unable to find ConsoleMode")
	}
}

// IsValid reports whether the receiver is one of the supported values.
func (ts ConsoleMode) IsValid() bool {
	switch ts {
	case ConsoleModeOff:
		fallthrough
	case ConsoleModePty:
		fallthrough
	case ConsoleModeTty:
		fallthrough
	case ConsoleModeFile:
		fallthrough
	case ConsoleModeSocket:
		fallthrough
	case ConsoleModeNull:
		return true
	default:
		return false
	}
}

// String returns the cloud-hypervisor text token for a valid ConsoleMode.
func (ts ConsoleMode) String() string {
	if ts.IsValid() {
		return string(ts)
	} else {
		return ""
	}
}

// MarshalText returns the textual representation used by encoders.
func (ts ConsoleMode) MarshalText() ([]byte, error) { return []byte(ts.String()), nil }

// UnmarshalText parses a textual value into the receiver.
func (ts *ConsoleMode) UnmarshalText(b []byte) error {
	length := len(b)
	if length < 0 {
		return errors.New("malformed format")
	}

	new_status, err := ConsoleModeNameOf(string(b))
	if err != nil {
		return err
	}

	*ts = new_status
	return nil
}
