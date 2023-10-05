package hvm

import (
	"errors"
)

type ConsoleMode string

const (
	ConsoleModeInvalid ConsoleMode = ""
	ConsoleModeOff     ConsoleMode = "Off"
	ConsoleModePty     ConsoleMode = "Pty"
	ConsoleModeTty     ConsoleMode = "Tty"
	ConsoleModeFile    ConsoleMode = "File"
	ConsoleModeNull    ConsoleMode = "Null"
)

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
	case string(ConsoleModeNull):
		return ConsoleModeNull, nil
	default:
		return ConsoleModeInvalid, errors.New("unable to find ConsoleMode")
	}
}

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
	case ConsoleModeNull:
		return true
	default:
		return false
	}
}

func (ts ConsoleMode) String() string {
	if ts.IsValid() {
		return string(ts)
	} else {
		return ""
	}
}

func (ts ConsoleMode) MarshalText() ([]byte, error) { return []byte(ts.String()), nil }

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
