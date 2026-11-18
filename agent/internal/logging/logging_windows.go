//go:build windows

package logging

import (
	"io"
	"os"
)

func platformWriter() io.Writer {
	return os.Stderr
}
