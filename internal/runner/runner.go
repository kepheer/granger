package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const MaxOutput = 96 * 1024

type Result struct {
	Title   string
	Command string
	Output  string
	OK      bool
	Status  string
}

type Runner struct{}

func New() Runner { return Runner{} }

func (Runner) Run(title string, timeout time.Duration, stdin io.Reader, name string, args ...string) Result {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	if stdin != nil {
		cmd.Stdin = stdin
	}
	var buf LimitedBuffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start).Round(time.Millisecond)
	out := strings.TrimRight(buf.String(), "\n")
	if out == "" {
		out = "(no output)"
	}
	if ctx.Err() == context.DeadlineExceeded {
		return Result{Title: title, Command: display(name, args...), Output: out, OK: false, Status: "timeout " + timeout.String()}
	}
	if err != nil {
		return Result{Title: title, Command: display(name, args...), Output: out, OK: false, Status: fmt.Sprintf("%v · %s", err, elapsed)}
	}
	return Result{Title: title, Command: display(name, args...), Output: out, OK: true, Status: "ok · " + elapsed.String()}
}

func (r Runner) RunSoft(title string, timeout time.Duration, stdin io.Reader, name string, args ...string) Result {
	res := r.Run(title, timeout, stdin, name, args...)
	if !res.OK {
		res.OK = true
		res.Status = "non-critical: " + res.Status
	}
	return res
}

func display(name string, args ...string) string {
	return strings.Join(append([]string{name}, args...), " ")
}

type LimitedBuffer struct {
	buf bytes.Buffer
}

func (b *LimitedBuffer) Write(p []byte) (int, error) {
	if b.buf.Len() < MaxOutput {
		remaining := MaxOutput - b.buf.Len()
		if len(p) > remaining {
			b.buf.Write(p[:remaining])
			b.buf.WriteString("\n... output truncated ...")
			return len(p), nil
		}
		b.buf.Write(p)
	}
	return len(p), nil
}

func (b *LimitedBuffer) String() string { return b.buf.String() }

type SafeBuffer struct {
	mu  sync.Mutex
	buf LimitedBuffer
}

func (b *SafeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *SafeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
