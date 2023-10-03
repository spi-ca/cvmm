package util

import (
	"net"
	"strings"
)

type MACAddress net.HardwareAddr

const macHexDigit = "0123456789abcdef"

func MustLoadMACAddress(val string) MACAddress {
	parsed, err := LoadMACAddress(val)
	if err != nil {
		panic(err)
	}
	return parsed
}

func LoadMACAddress(val string) (MACAddress, error) {
	parsed, err := net.ParseMAC(val)
	return MACAddress(parsed), err
}

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

func (ts *MACAddress) UnmarshalText(b []byte) error {
	parsed, err := LoadMACAddress(string(b))
	if err != nil {
		return err
	}

	*ts = parsed
	return nil
}
