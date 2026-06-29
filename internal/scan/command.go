package scan

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"time"
)

const (
	metadataCommandTimeout = 5 * time.Second
	brewCommandTimeout     = 20 * time.Second

	maxPlistJSONBytes = 2 << 20
	maxMDLSBytes      = 4 << 10
	maxSQLiteBytes    = 8 << 20
	maxBrewListBytes  = 1 << 20
	maxBrewInfoBytes  = 8 << 20
)

var errOutputTooLarge = errors.New("command output exceeded limit")

func commandOutputLimited(timeout time.Duration, limit int64, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	limited := &limitedWriter{w: &stdout, remaining: limit}
	cmd.Stdout = limited
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	if limited.exceeded {
		return nil, errOutputTooLarge
	}
	return stdout.Bytes(), nil
}

type limitedWriter struct {
	w         io.Writer
	remaining int64
	exceeded  bool
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remaining {
		w.exceeded = true
		if w.remaining > 0 {
			_, _ = w.w.Write(p[:w.remaining])
			w.remaining = 0
		}
		return len(p), errOutputTooLarge
	}
	n, err := w.w.Write(p)
	w.remaining -= int64(n)
	return n, err
}

func executableFile(path string) bool {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return false
	}
	return fi.Mode()&0o111 != 0
}
