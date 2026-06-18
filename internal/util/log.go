package util

import (
	"bufio"
	"bytes"
	"log"
	"os"
	"strings"
)

var (
	InfoLog = log.Default()
	ErrLog  = log.New(os.Stderr, "", log.LstdFlags)
)

// init prepares package-level defaults before the package is used.
func init() {
	InfoLog.SetOutput(os.Stdout)
}

// LogWriter routes log output to either stdout or stderr.
type LogWriter struct {
}

// Write forwards log bytes to the configured output writer.
func (w LogWriter) Write(b []byte) (int, error) {
	if len(b) < 1 {
		return 0, nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(b))
	for scanner.Scan() {
		trimmed := strings.TrimSpace(scanner.Text())
		if len(trimmed) < 1 {
			continue
		}

		if crIndex := strings.LastIndexByte(trimmed, '\r'); crIndex > 0 {
			trimmed = trimmed[crIndex:]
		}
		_ = InfoLog.Output(1, trimmed)
	}
	return len(b), nil
}
