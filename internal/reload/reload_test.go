package reload

import (
	"context"
	"errors"
	"testing"

	"github.com/dborovcanin/rice/internal/adapter"
	"github.com/dborovcanin/rice/internal/adapter/dunst"
	"github.com/dborovcanin/rice/internal/adapter/foot"
	"github.com/dborovcanin/rice/internal/adapter/rofi"
	"github.com/dborovcanin/rice/internal/adapter/sway"
	"github.com/dborovcanin/rice/internal/adapter/swaylock"
	"github.com/dborovcanin/rice/internal/adapter/waybar"
	"github.com/dborovcanin/rice/internal/command"
)

func byComponent(reports []Report) map[string]Report {
	out := map[string]Report{}
	for _, r := range reports {
		out[r.Component] = r
	}
	return out
}

func TestReloadPerComponent(t *testing.T) {
	fake := command.NewFake()
	m := NewWith(fake)

	adapters := []adapter.Adapter{
		sway.New(), waybar.New(), dunst.New(), rofi.New(), foot.New(), swaylock.New(),
	}
	reports := byComponent(m.Reload(context.Background(), adapters))

	tests := []struct {
		component string
		outcome   Outcome
		command   string
	}{
		{"sway", Reloaded, "swaymsg reload"},
		{"dunst", Reloaded, "dunstctl reload"},
		{"waybar", Reloaded, "pkill -SIGUSR2 -x waybar"},
		{"rofi", NotSupported, ""},
		{"foot", NotSupported, ""},
		{"swaylock", NotSupported, ""},
	}

	for _, tt := range tests {
		t.Run(tt.component, func(t *testing.T) {
			got := reports[tt.component]
			if got.Outcome != tt.outcome {
				t.Errorf("outcome = %v, want %v", got.Outcome, tt.outcome)
			}
			if got.Command != tt.command {
				t.Errorf("command = %q, want %q", got.Command, tt.command)
			}
		})
	}
}

func TestReloadSkipsProcessesThatAreNotRunning(t *testing.T) {
	fake := command.NewFake()
	fake.Errors = map[string]error{"pgrep": errors.New("no process")}
	m := NewWith(fake)

	reports := byComponent(m.Reload(context.Background(), []adapter.Adapter{sway.New(), dunst.New()}))
	for _, name := range []string{"sway", "dunst"} {
		if got := reports[name].Outcome; got != NotRunning {
			t.Errorf("%s outcome = %v, want NotRunning", name, got)
		}
	}
	for _, c := range fake.Commands() {
		if c == "swaymsg reload" || c == "dunstctl reload" {
			t.Errorf("reloaded a process that is not running: %s", c)
		}
	}
}

func TestReloadWithoutPgrepStillTries(t *testing.T) {
	fake := command.NewFake()
	fake.Missing = map[string]bool{"pgrep": true}
	m := NewWith(fake)

	reports := byComponent(m.Reload(context.Background(), []adapter.Adapter{sway.New()}))
	if got := reports["sway"].Outcome; got != Reloaded {
		t.Errorf("outcome = %v, want Reloaded", got)
	}
}

func TestReloadReportsMissingTool(t *testing.T) {
	fake := command.NewFake()
	fake.Missing = map[string]bool{"swaymsg": true}
	m := NewWith(fake)

	reports := byComponent(m.Reload(context.Background(), []adapter.Adapter{sway.New()}))
	report := reports["sway"]
	if report.Outcome != Failed || report.Err == nil {
		t.Fatalf("report = %+v", report)
	}
	if len(Failures([]Report{report})) != 1 {
		t.Error("Failures should include a failed report")
	}
}

func TestReloadReportsCommandFailure(t *testing.T) {
	fake := command.NewFake()
	fake.Errors = map[string]error{"swaymsg": errors.New("connection refused")}
	m := NewWith(fake)

	reports := byComponent(m.Reload(context.Background(), []adapter.Adapter{sway.New()}))
	if got := reports["sway"].Outcome; got != Failed {
		t.Errorf("outcome = %v, want Failed", got)
	}
}

func TestOutcomeString(t *testing.T) {
	tests := map[Outcome]string{
		Reloaded:     "reloaded",
		NotRunning:   "not running",
		NotSupported: "new instances only",
		Failed:       "failed",
	}
	for outcome, want := range tests {
		if got := outcome.String(); got != want {
			t.Errorf("Outcome(%d) = %q, want %q", outcome, got, want)
		}
	}
}
