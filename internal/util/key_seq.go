package util

import "io"

// CaptureEscapeKeySequence copies bytes after an escape key until a control sequence terminates.
func CaptureEscapeKeySequence(r io.Reader, w io.Writer) {
	step := 0
	buf := make([]byte, 1)

	for {
		sz, err := r.Read(buf)
		if err != nil {
			return
		} else if sz == 0 {
			continue
		}

		switch step {
		case 0:
			switch buf[0] {
			case 0x1b:
				step++
			}
		case 1:
			switch buf[0] {
			case 0x28:
				return
			}
			step = 0
		default:
		}
		_, _ = w.Write(buf)
	}
}
