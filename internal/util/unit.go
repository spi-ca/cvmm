package util

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

type IECSize uint64

var (
	iecSizeLabel = []string{
		"",
		"K", "M", "G",
		"T", "P", "E",
		"Z", "Y", "R",
		"Q",
	}
	sizeParsePattern = regexp.MustCompile(`^([+-]?(?:[0-9]*[.])?[0-9]+)\s*(?:([A-Za-z])i?)?$`)
)

func MustLoadIECSize(val string) IECSize {
	sz, err := LoadIECSize(val)
	if err != nil {
		panic(err)
	}

	return sz
}

func LoadIECSize(val string) (IECSize, error) {
	matcher := sizeParsePattern.FindStringSubmatch(val)
	if matcher == nil {
		return 0, errors.New("no matched values")
	}
	value := matcher[1]
	unit := strings.ToUpper(matcher[2])

	times := -1
	for i := 0; i < len(iecSizeLabel); i++ {
		if iecSizeLabel[i] == unit {
			times = i
			break
		}
	}

	if times < 0 {
		return 0, errors.New("unit overflowed")
	}

	p := math.Pow(1024, float64(times))
	mantissa, _ := strconv.ParseFloat(value, 64)
	if math.IsNaN(mantissa) {
		return 0, fmt.Errorf("mantissa(%s) is NaN ", value)
	}

	return IECSize(mantissa * p), nil
}

func (ts IECSize) String() string {
	const base = 1024.0
	i := math.Floor(math.Log(float64(ts)) / math.Log(base))
	p := math.Pow(base, i)

	s := math.Ceil(float64(ts)*1000/p) / 1000
	unit := iecSizeLabel[int(i)]

	return fmt.Sprintf("%g%s", s, unit)
}

func (ts IECSize) MarshalText() ([]byte, error) {
	const base = 1024.0
	i := math.Floor(math.Log(float64(ts)) / math.Log(base))
	p := math.Pow(base, i)

	s := math.Ceil(float64(ts)*1000/p) / 1000
	unit := iecSizeLabel[int(i)]

	return fmt.Appendf(nil, "%g%s", s, unit), nil
}

func (ts *IECSize) UnmarshalText(b []byte) error {
	length := len(b)
	if length < 0 {
		return errors.New("malformed format")
	}

	matcher := sizeParsePattern.FindSubmatch(b)
	if matcher == nil {
		return errors.New("no matched values")
	}
	value := string(matcher[1])
	unit := string(bytes.ToUpper(matcher[2]))

	times := -1
	for i := 0; i < len(iecSizeLabel); i++ {
		if iecSizeLabel[i] == unit {
			times = i
			break
		}
	}

	if times < 0 {
		return errors.New("unit overflowed")
	}

	p := math.Pow(1024, float64(times))
	mantissa, _ := strconv.ParseFloat(value, 64)
	if math.IsNaN(mantissa) {
		return fmt.Errorf("mantissa(%s) is NaN ", value)
	}
	*ts = IECSize(mantissa * p)

	return nil
}
