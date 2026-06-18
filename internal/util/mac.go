package util

import (
	"crypto/md5"
	"net"
	"strconv"
	"strings"
	"time"
)

// MACAddress is a validated address value used by cvmm configuration and rendering code.
type MACAddress net.HardwareAddr

const macHexDigit = "0123456789abcdef"

// MustLoadMACAddress parses a MAC address and panics on invalid input.
func MustLoadMACAddress(val string) MACAddress {
	parsed, err := LoadMACAddress(val)
	if err != nil {
		panic(err)
	}
	return parsed
}

// LoadMACAddress parses a colon-separated MAC address.
func LoadMACAddress(val string) (MACAddress, error) {
	parsed, err := net.ParseMAC(val)
	return MACAddress(parsed), err
}

// String formats the receiver as its canonical text value.
func (ts MACAddress) String() string {
	if len(ts) == 0 {
		return ""
	}

	buf := new(strings.Builder)
	for i, b := range ts {
		if i > 0 {
			buf.WriteByte(':')
		}
		buf.WriteByte(macHexDigit[b>>4])
		buf.WriteByte(macHexDigit[b&0xF])
	}
	return buf.String()
}

// MarshalText returns the textual representation used by encoders.
func (ts MACAddress) MarshalText() ([]byte, error) {
	if len(ts) == 0 {
		return nil, nil
	}

	buf := make([]byte, 0, len(ts)*3-1)
	for i, b := range ts {
		if i > 0 {
			buf = append(buf, ':')
		}
		buf = append(buf, macHexDigit[b>>4])
		buf = append(buf, macHexDigit[b&0xF])
	}
	return buf, nil
}

// UnmarshalText parses a textual value into the receiver.
func (ts *MACAddress) UnmarshalText(b []byte) error {
	parsed, err := LoadMACAddress(string(b))
	if err != nil {
		return err
	}

	*ts = parsed
	return nil
}

// GenerateIfName derives a deterministic interface name from the MAC address bytes.
func (ts *MACAddress) GenerateIfName(prefix string) string {
	buf := new(strings.Builder)

	buf.WriteString(prefix)

	for _, b := range (*ts)[3:] {
		buf.WriteByte(macHexDigit[b>>4])
		buf.WriteByte(macHexDigit[b&0xF])
	}

	return buf.String()
}

// GenerateKvmMACAddress creates a locally administered MAC address for a guest TAP device.
func GenerateKvmMACAddress() MACAddress {
	at := strconv.FormatInt(time.Now().UnixMilli(), 10)
	chksum := md5.Sum([]byte(at))
	return MACAddress{
		0x52, 0x54, 0x00, chksum[0], chksum[1], chksum[2],
	}
}
