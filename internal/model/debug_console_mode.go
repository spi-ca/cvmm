package model

import (
	"errors"
)

type DebugConsoleMode string

const (
	DebugConsoleModeInvalid DebugConsoleMode = ""
	DebugConsoleModeOff     DebugConsoleMode = "Off"
	DebugConsoleModePty     DebugConsoleMode = "Pty"
	DebugConsoleModeTty     DebugConsoleMode = "Tty"
	DebugConsoleModeFile    DebugConsoleMode = "File"
	DebugConsoleModeNull    DebugConsoleMode = "Null"
)

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

func (ts DebugConsoleMode) String() string {
	if ts.IsValid() {
		return string(ts)
	} else {
		return ""
	}
}

func (ts DebugConsoleMode) MarshalText() ([]byte, error) { return []byte(ts.String()), nil }

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
