package model

import (
	"errors"
)

type LandlockMode string

const (
	LandlockModeInvalid   LandlockMode = ""
	LandlockModeReadOnly  LandlockMode = "r"
	LandlockModeWriteOnly LandlockMode = "w"
	LandlockModeReadWrite LandlockMode = "rw"
)

func LandlockModeNameOf(value string) (LandlockMode, error) {
	switch value {
	case string(LandlockModeReadOnly):
		return LandlockModeReadOnly, nil
	case string(LandlockModeWriteOnly):
		return LandlockModeWriteOnly, nil
	case string(LandlockModeReadWrite):
		return LandlockModeReadWrite, nil
	default:
		return LandlockModeInvalid, errors.New("unable to find LandlockMode")
	}
}

func (ts LandlockMode) IsValid() bool {
	switch ts {
	case LandlockModeReadOnly:
		fallthrough
	case LandlockModeWriteOnly:
		fallthrough
	case LandlockModeReadWrite:
		return true
	default:
		return false
	}
}

func (ts LandlockMode) String() string {
	if ts.IsValid() {
		return string(ts)
	} else {
		return ""
	}
}

func (ts LandlockMode) MarshalText() ([]byte, error) { return []byte(ts.String()), nil }

func (ts *LandlockMode) UnmarshalText(b []byte) error {
	length := len(b)
	if length < 0 {
		return errors.New("malformed format")
	}

	new_status, err := LandlockModeNameOf(string(b))
	if err != nil {
		return err
	}

	*ts = new_status
	return nil
}
