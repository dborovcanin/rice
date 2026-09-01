// Package reload tells running applications to pick up a new generation.
// Applications differ in what that means, so the reload mode each adapter
// declares decides what happens — Rice does not pretend they behave alike.
package reload

import (
	"context"
	"fmt"

	"github.com/dborovcanin/rice/internal/adapter"
	"github.com/dborovcanin/rice/internal/command"
)

// Outcome is what happened to one component.
type Outcome int

const (
	// Reloaded means the running application was told to re-read its config.
	Reloaded Outcome = iota
	// NotRunning means there was nothing to reload.
	NotRunning
	// NotSupported means the application only reads its config at startup.
	NotSupported
	// Failed means the reload command returned an error.
	Failed
)

func (o Outcome) String() string {
	switch o {
	case Reloaded:
		return "reloaded"
	case NotRunning:
		return "not running"
	case NotSupported:
		return "new instances only"
	case Failed:
		return "failed"
	default:
		return "unknown"
	}
}

// Report is the result for one component.
type Report struct {
	Component string
	Outcome   Outcome
	// Command is what was run, for the verbose output and for tests.
	Command string
	Err     error
}

// recipe is how one component is reloaded.
type recipe struct {
	// process is the executable name to check for, via pgrep.
	process string
	// name and args are the reload command.
	name string
	args []string
}

// recipes covers the components that can reload at all. Anything absent is
// ReloadNewInstancesOnly in practice: Rofi, Foot and swaylock read their
// configuration when they start.
var recipes = map[string]recipe{
	"sway":  {process: "sway", name: "swaymsg", args: []string{"reload"}},
	"dunst": {process: "dunst", name: "dunstctl", args: []string{"reload"}},
	// Waybar reloads its configuration and stylesheet on SIGUSR2.
	"waybar": {process: "waybar", name: "pkill", args: []string{"-SIGUSR2", "-x", "waybar"}},
}

// Manager reloads components through an injected runner, so tests never touch
// a real desktop session.
type Manager struct {
	Runner command.Runner
}

// New returns a Manager over the real process runner.
func New() *Manager { return &Manager{Runner: command.New()} }

// NewWith returns a Manager over a specific runner.
func NewWith(runner command.Runner) *Manager { return &Manager{Runner: runner} }

// Reload asks each adapter's application to pick up the new configuration.
// A component that is not running, or cannot reload at all, is reported rather
// than treated as an error: neither is a failure.
func (m *Manager) Reload(ctx context.Context, adapters []adapter.Adapter) []Report {
	reports := make([]Report, 0, len(adapters))

	for _, a := range adapters {
		report := Report{Component: a.Name()}

		r, ok := recipes[a.Name()]
		if !ok || a.ReloadMode() == adapter.ReloadNewInstancesOnly || a.ReloadMode() == adapter.ReloadNone {
			report.Outcome = NotSupported
			reports = append(reports, report)
			continue
		}

		if !m.running(ctx, r.process) {
			report.Outcome = NotRunning
			reports = append(reports, report)
			continue
		}
		if !m.Runner.Look(r.name) {
			report.Outcome = Failed
			report.Err = fmt.Errorf("%s is not installed", r.name)
			reports = append(reports, report)
			continue
		}

		report.Command = command.Describe(r.name, r.args)
		if err := m.Runner.Run(ctx, r.name, r.args...); err != nil {
			report.Outcome = Failed
			report.Err = err
		} else {
			report.Outcome = Reloaded
		}
		reports = append(reports, report)
	}
	return reports
}

// running reports whether a process is up. Without pgrep Rice assumes it is,
// so a missing pgrep degrades to attempting the reload rather than skipping it.
func (m *Manager) running(ctx context.Context, process string) bool {
	if !m.Runner.Look("pgrep") {
		return true
	}
	return m.Runner.Run(ctx, "pgrep", "-x", process) == nil
}

// Failures returns the reports that failed.
func Failures(reports []Report) []Report {
	var out []Report
	for _, r := range reports {
		if r.Outcome == Failed {
			out = append(out, r)
		}
	}
	return out
}
