package session

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/dborovcanin/rice/internal/command"
	"github.com/dborovcanin/rice/internal/generation"
)

// sandboxGeneration is the generation number stamped into sandbox output. A
// sandbox is not a generation and never becomes one, so it is numbered zero
// rather than borrowing the number a real apply would consume.
const sandboxGeneration = 0

// Launch describes how one component is previewed: which binary is run, with
// which arguments, and anything the user should know before it starts.
type Launch struct {
	// Binary is the program to run.
	Binary string
	// Args builds the argument list against a rendered sandbox directory.
	Args func(dir string) []string
	// Note is a consequence worth stating but not worth blocking on, such as
	// a second bar overlapping the running one.
	Note string
	// Confirm is set when starting the program does something the user must
	// accept first, such as locking the screen. Empty means no confirmation.
	Confirm string
	// Blocked explains why the component cannot be previewed at all.
	Blocked string
}

// launches is the preview table. It is deliberately explicit: previewing a
// desktop component is application-specific, and pretending otherwise would
// produce a preview that quietly does the wrong thing.
var launches = map[string]Launch{
	"foot": {
		Binary: "foot",
		Args:   func(dir string) []string { return []string{"-c", filepath.Join(dir, "foot", "foot.ini")} },
	},
	"rofi": {
		Binary: "rofi",
		Args: func(dir string) []string {
			return []string{"-config", filepath.Join(dir, "rofi", "config.rasi"), "-show", "drun"}
		},
	},
	"waybar": {
		Binary: "waybar",
		Args: func(dir string) []string {
			return []string{
				"-c", filepath.Join(dir, "waybar", "config.jsonc"),
				"-s", filepath.Join(dir, "waybar", "style.css"),
			}
		},
		Note: "a second bar appears over the running one until you close the preview",
	},
	"sway": {
		Binary: "sway",
		Args:   func(dir string) []string { return []string{"-c", filepath.Join(dir, "sway", "config")} },
		Note:   "runs nested, as a window inside the current session",
	},
	"dunst": {
		Blocked: "dunst would take the D-Bus name the running notification daemon owns",
	},
	"swaylock": {
		Binary:  "swaylock",
		Args:    func(dir string) []string { return []string{"-C", filepath.Join(dir, "swaylock", "config")} },
		Confirm: "this locks the screen: you will need your password to get back in",
	},
}

// LaunchFor reports how a component would be previewed. The error explains why
// a preview is unavailable, so an interface can say so rather than hiding the
// action without a reason.
func (s *Session) LaunchFor(component string) (Launch, error) {
	if _, err := s.registry.Get(component); err != nil {
		return Launch{}, err
	}
	if !s.Draft.Config.Components.Enabled(component) {
		return Launch{}, fmt.Errorf("%s is not enabled in config.toml", component)
	}

	l, ok := launches[component]
	if !ok {
		return Launch{}, fmt.Errorf("no preview is defined for %s", component)
	}
	if l.Blocked != "" {
		return l, fmt.Errorf("cannot preview %s: %s", component, l.Blocked)
	}
	if !s.runner.Look(l.Binary) {
		return l, fmt.Errorf("cannot preview %s: %s is not installed", component, l.Binary)
	}
	return l, nil
}

// Render produces every enabled component's files from the draft, in memory.
// It renders the resolved theme, which is what a real build would use.
func (s *Session) Render() ([]generation.Rendered, error) {
	return s.builder.Render(s.resolved.Config, s.resolved.Theme, sandboxGeneration)
}

// Sandbox renders the draft into a fresh private directory and validates it,
// exactly as a real build would. The directory belongs to the caller until the
// returned cleanup runs.
//
// Nothing under ~/.config is touched and `current` is not moved: a sandbox is
// invisible to the running desktop until a program is pointed at it.
func (s *Session) Sandbox() (dir string, cleanup func() error, err error) {
	if err := os.MkdirAll(s.sandboxRoot, 0o700); err != nil {
		return "", nil, fmt.Errorf("create %s: %w", s.sandboxRoot, err)
	}

	// MkdirTemp gives a private, unpredictable name, which matters because the
	// root lives in a world-writable temporary directory.
	dir, err = os.MkdirTemp(s.sandboxRoot, "draft-")
	if err != nil {
		return "", nil, fmt.Errorf("create sandbox: %w", err)
	}
	remove := func() error { return os.RemoveAll(dir) }

	files, err := s.Render()
	if err != nil {
		_ = remove()
		return "", nil, err
	}
	for _, f := range files {
		dest := filepath.Join(dir, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
			_ = remove()
			return "", nil, fmt.Errorf("create %s: %w", filepath.Dir(dest), err)
		}
		if err := os.WriteFile(dest, f.Content, f.Mode); err != nil {
			_ = remove()
			return "", nil, fmt.Errorf("write %s: %w", dest, err)
		}
	}

	adapters, err := s.registry.Select(s.Components())
	if err != nil {
		_ = remove()
		return "", nil, err
	}
	for _, a := range adapters {
		if err := a.Validate(dir); err != nil {
			_ = remove()
			return "", nil, err
		}
	}
	return dir, remove, nil
}

// Preview is a running preview: one application, pointed at one sandbox.
type Preview struct {
	// Component is what is being previewed.
	Component string
	// Dir is the sandbox the application was pointed at.
	Dir string

	handle *command.Handle
	clean  func() error
	once   sync.Once
}

// Command renders the preview invocation the way it would be typed.
func (p *Preview) Command() string { return p.handle.String() }

// Wait blocks until the previewed application exits, then removes its sandbox.
func (p *Preview) Wait() error {
	err := p.handle.Wait()
	p.cleanup()
	return err
}

// Stop kills the previewed application and removes its sandbox. Stopping a
// preview that has already exited is not an error.
func (p *Preview) Stop() error {
	err := p.handle.Kill()
	p.cleanup()
	return err
}

func (p *Preview) cleanup() {
	p.once.Do(func() {
		if p.clean != nil {
			_ = p.clean()
		}
	})
}

// Preview renders the draft and launches one application against it. The
// caller must eventually call Wait or Stop, which is what removes the sandbox.
//
// A component whose Launch carries a Confirm is refused unless confirmed is
// set, so a preview cannot lock the screen by accident.
func (s *Session) Preview(component string, confirmed bool) (*Preview, error) {
	l, err := s.LaunchFor(component)
	if err != nil {
		return nil, err
	}
	if l.Confirm != "" && !confirmed {
		return nil, fmt.Errorf("previewing %s needs confirmation: %s", component, l.Confirm)
	}

	dir, cleanup, err := s.Sandbox()
	if err != nil {
		return nil, err
	}

	handle, err := s.runner.Start(l.Binary, l.Args(dir)...)
	if err != nil {
		_ = cleanup()
		return nil, err
	}

	p := &Preview{Component: component, Dir: dir, handle: handle, clean: cleanup}
	s.previews = append(s.previews, p)
	return p, nil
}
