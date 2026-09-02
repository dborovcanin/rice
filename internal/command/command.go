// Package command centralizes process execution so adapters and the reload
// manager never call exec.Command directly. That keeps command construction
// testable and gives every invocation the same timeout and error handling.
package command

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// DefaultTimeout bounds any command Rice runs. Desktop helpers are expected to
// return immediately; anything slower is a hang, not slow work.
const DefaultTimeout = 10 * time.Second

// StderrLimit bounds how much of a launched program's standard error is kept,
// so a chatty application cannot grow the buffer without limit.
const StderrLimit = 8 << 10

// Runner executes external programs.
type Runner interface {
	// Run executes a command and discards its output.
	Run(ctx context.Context, name string, args ...string) error
	// Output executes a command and returns its standard output.
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
	// Pipe executes a command with stdin on its standard input.
	Pipe(ctx context.Context, stdin []byte, name string, args ...string) error
	// Start launches a program Rice does not wait for and is not timed out,
	// such as an application opened to preview a configuration.
	Start(name string, args ...string) (*Handle, error)
	// Look reports whether a program exists in PATH.
	Look(name string) bool
}

// Handle is a started process. Unlike Run and Output, a handle is not bounded
// by DefaultTimeout: the process lives until it exits or is killed.
type Handle struct {
	name   string
	args   []string
	cmd    *exec.Cmd
	stderr *limitedBuffer
}

// PID is the process id, or zero when nothing was really started.
func (h *Handle) PID() int {
	if h == nil || h.cmd == nil || h.cmd.Process == nil {
		return 0
	}
	return h.cmd.Process.Pid
}

// Wait blocks until the process exits.
func (h *Handle) Wait() error {
	if h == nil || h.cmd == nil {
		return nil
	}
	if err := h.cmd.Wait(); err != nil {
		if msg := strings.TrimSpace(h.stderr.String()); msg != "" {
			return fmt.Errorf("%s: %w: %s", h, err, msg)
		}
		return fmt.Errorf("%s: %w", h, err)
	}
	return nil
}

// Kill terminates the process. Killing an already-exited process is not an
// error, because the caller usually cannot know which happened first.
func (h *Handle) Kill() error {
	if h == nil || h.cmd == nil || h.cmd.Process == nil {
		return nil
	}
	if err := h.cmd.Process.Kill(); err != nil {
		return nil
	}
	return nil
}

// String renders the command the way it would be typed.
func (h *Handle) String() string {
	if h == nil {
		return ""
	}
	return Describe(h.name, h.args)
}

// Exec is the real Runner.
type Exec struct {
	// Timeout overrides DefaultTimeout when non-zero.
	Timeout time.Duration
}

// New returns a Runner that executes real commands.
func New() *Exec { return &Exec{} }

func (e *Exec) Run(ctx context.Context, name string, args ...string) error {
	_, err := e.Output(ctx, name, args...)
	return err
}

func (e *Exec) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	timeout := e.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return stdout.Bytes(), fmt.Errorf("%s: %w: %s", Describe(name, args), err, msg)
		}
		return stdout.Bytes(), fmt.Errorf("%s: %w", Describe(name, args), err)
	}
	return stdout.Bytes(), nil
}

func (e *Exec) Pipe(ctx context.Context, stdin []byte, name string, args ...string) error {
	timeout := e.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("%s: %w: %s", Describe(name, args), err, msg)
		}
		return fmt.Errorf("%s: %w", Describe(name, args), err)
	}
	return nil
}

func (e *Exec) Start(name string, args ...string) (*Handle, error) {
	h := &Handle{name: name, args: args, stderr: &limitedBuffer{limit: StderrLimit}}

	// No context: a previewed application outlives any timeout Rice would
	// impose. Standard output is discarded so a launched program cannot write
	// over the terminal interface that started it.
	h.cmd = exec.Command(name, args...)
	h.cmd.Stdout = io.Discard
	h.cmd.Stderr = h.stderr

	if err := h.cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", Describe(name, args), err)
	}
	return h, nil
}

func (e *Exec) Look(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// limitedBuffer keeps at most limit bytes, discarding the rest.
type limitedBuffer struct {
	limit int
	buf   bytes.Buffer
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if room := b.limit - b.buf.Len(); room > 0 {
		if len(p) > room {
			b.buf.Write(p[:room])
		} else {
			b.buf.Write(p)
		}
	}
	return len(p), nil
}

func (b *limitedBuffer) String() string { return b.buf.String() }

// Describe renders a command the way it would be typed, for error messages.
func Describe(name string, args []string) string {
	if len(args) == 0 {
		return name
	}
	return name + " " + strings.Join(args, " ")
}

// Call is one recorded invocation, used by Fake.
type Call struct {
	Name string
	Args []string
	// Stdin is what Pipe was handed, and is nil for every other call.
	Stdin []byte
}

func (c Call) String() string { return Describe(c.Name, c.Args) }

// Fake records commands instead of running them, so tests can assert on what
// Rice would have done without a desktop session.
type Fake struct {
	// Calls holds every invocation, in order.
	Calls []Call
	// Missing names programs that Look should report as absent.
	Missing map[string]bool
	// Errors maps a program name to an error Run and Output should return.
	Errors map[string]error
	// Outputs maps a program name to canned standard output.
	Outputs map[string]string
}

// NewFake returns an empty Fake runner.
func NewFake() *Fake { return &Fake{} }

func (f *Fake) Run(ctx context.Context, name string, args ...string) error {
	_, err := f.Output(ctx, name, args...)
	return err
}

func (f *Fake) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	f.Calls = append(f.Calls, Call{Name: name, Args: args})
	if err, ok := f.Errors[name]; ok {
		return nil, err
	}
	return []byte(f.Outputs[name]), nil
}

func (f *Fake) Pipe(ctx context.Context, stdin []byte, name string, args ...string) error {
	f.Calls = append(f.Calls, Call{Name: name, Args: args, Stdin: stdin})
	return f.Errors[name]
}

func (f *Fake) Start(name string, args ...string) (*Handle, error) {
	f.Calls = append(f.Calls, Call{Name: name, Args: args})
	if err, ok := f.Errors[name]; ok {
		return nil, err
	}
	// No cmd: the handle waits and kills as a no-op.
	return &Handle{name: name, args: args}, nil
}

func (f *Fake) Look(name string) bool { return !f.Missing[name] }

// Commands returns every recorded invocation as it would be typed.
func (f *Fake) Commands() []string {
	out := make([]string, 0, len(f.Calls))
	for _, c := range f.Calls {
		out = append(out, c.String())
	}
	return out
}
