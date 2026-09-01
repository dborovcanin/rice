// Package command centralizes process execution so adapters and the reload
// manager never call exec.Command directly. That keeps command construction
// testable and gives every invocation the same timeout and error handling.
package command

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// DefaultTimeout bounds any command Rice runs. Desktop helpers are expected to
// return immediately; anything slower is a hang, not slow work.
const DefaultTimeout = 10 * time.Second

// Runner executes external programs.
type Runner interface {
	// Run executes a command and discards its output.
	Run(ctx context.Context, name string, args ...string) error
	// Output executes a command and returns its standard output.
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
	// Look reports whether a program exists in PATH.
	Look(name string) bool
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

func (e *Exec) Look(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

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

func (f *Fake) Look(name string) bool { return !f.Missing[name] }

// Commands returns every recorded invocation as it would be typed.
func (f *Fake) Commands() []string {
	out := make([]string, 0, len(f.Calls))
	for _, c := range f.Calls {
		out = append(out, c.String())
	}
	return out
}
