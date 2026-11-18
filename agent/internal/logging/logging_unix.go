//go:build !windows

package logging

import (
	"io"
	"log/syslog"
	"os"
)

func initSyslog() (io.Writer, error) {
	w, err := syslog.New(syslog.LOG_DAEMON|syslog.LOG_INFO, "os-pangolin")
	if err != nil {
		return nil, err
	}
	return w, nil
}

func platformWriter() io.Writer {
	if w, err := initSyslog(); err == nil {
		return io.MultiWriter(os.Stderr, w)
	}
	return os.Stderr
}
