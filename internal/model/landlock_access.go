package model

import (
	"errors"
)

// LandlockMode is an enum-like value used when translating cvmm input or cloud-hypervisor state.
type LandlockMode string

const (
	// LandlockModeInvalid is the zero value used when parsing fails.
	LandlockModeInvalid LandlockMode = ""
	// LandlockModeReadOnly permits read access.
	LandlockModeReadOnly LandlockMode = "r"
	// LandlockModeWriteOnly permits write access.
	LandlockModeWriteOnly LandlockMode = "w"
	// LandlockModeReadWrite permits read and write access.
	LandlockModeReadWrite LandlockMode = "rw"
)

// LandlockModeNameOf parses a cloud-hypervisor Landlock access mode name.
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

// IsValid reports whether the receiver is one of the supported values.
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

// String returns the cloud-hypervisor text token for a valid LandlockMode.
func (ts LandlockMode) String() string {
	if ts.IsValid() {
		return string(ts)
	} else {
		return ""
	}
}

// MarshalText returns the textual representation used by encoders.
func (ts LandlockMode) MarshalText() ([]byte, error) { return []byte(ts.String()), nil }

// UnmarshalText parses a textual value into the receiver.
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
