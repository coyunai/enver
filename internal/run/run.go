package run

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"sync"

	"github.com/enver-dev/enver/internal/vfox"
)

// Command runs a command, streaming normalized output lines to onLine.
func Command(ctx context.Context, name string, args []string, dir string, env []string, onLine func(string)) error {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if env != nil {
		cmd.Env = env
	}
	cmd.Stdin = strings.NewReader("")
	lw := &lineWriter{emit: onLine}
	cmd.Stdout = lw
	cmd.Stderr = lw
	err := cmd.Run()
	lw.Flush()
	return err
}

// Output runs a command and returns combined stdout/stderr.
func Output(ctx context.Context, name string, args []string, dir string, env []string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if env != nil {
		cmd.Env = env
	}
	cmd.Stdin = strings.NewReader("")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return vfox.StripANSI(buf.String()), err
}

type lineWriter struct {
	mu   sync.Mutex
	buf  []byte
	emit func(string)
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)
	for {
		idx := bytes.IndexAny(w.buf, "\r\n")
		if idx < 0 {
			break
		}
		line := string(w.buf[:idx])
		w.buf = w.buf[idx+1:]
		w.deliver(line)
	}
	if len(w.buf) > 32*1024 {
		w.deliver(string(w.buf))
		w.buf = w.buf[:0]
	}
	return len(p), nil
}

func (w *lineWriter) deliver(line string) {
	line = strings.TrimSpace(vfox.StripANSI(line))
	if line != "" && w.emit != nil {
		w.emit(line)
	}
}

func (w *lineWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.buf) > 0 {
		w.deliver(string(w.buf))
		w.buf = w.buf[:0]
	}
}
