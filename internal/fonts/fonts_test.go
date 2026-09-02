package fonts_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/dborovcanin/rice/internal/command"
	"github.com/dborovcanin/rice/internal/fonts"
)

// fcRunner answers fc-list with a fixed family list, and a shorter one for the
// monospace pattern, the way fontconfig does.
type fcRunner struct {
	*command.Fake
	all  string
	mono string
}

func (r *fcRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	if _, err := r.Fake.Output(ctx, name, args...); err != nil {
		return nil, err
	}
	if slices.Contains(args, ":spacing=100") {
		return []byte(r.mono), nil
	}
	return []byte(r.all), nil
}

func newRunner() *fcRunner {
	return &fcRunner{
		Fake: command.NewFake(),
		all: "Inter\nJetBrainsMono Nerd Font\nInter\nDejaVu Sans\n" +
			"Iosevka\n\n  \nJetBrains Mono\n",
		mono: "JetBrainsMono Nerd Font\nIosevka\nJetBrains Mono\n",
	}
}

func TestLoadDeduplicatesAndMarksMonospace(t *testing.T) {
	catalog, err := fonts.Load(context.Background(), newRunner())
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if got, want := catalog.Len(), 5; got != want {
		t.Fatalf("families = %d, want %d: %v", got, want, catalog.All())
	}

	mono := map[string]bool{}
	for _, f := range catalog.All() {
		mono[f.Name] = f.Mono
	}
	for _, name := range []string{"JetBrainsMono Nerd Font", "Iosevka", "JetBrains Mono"} {
		if !mono[name] {
			t.Errorf("%s should be monospaced", name)
		}
	}
	for _, name := range []string{"Inter", "DejaVu Sans"} {
		if mono[name] {
			t.Errorf("%s should not be monospaced", name)
		}
	}
}

func TestLoadReportsMissingFontconfig(t *testing.T) {
	runner := newRunner()
	runner.Missing = map[string]bool{"fc-list": true}

	if _, err := fonts.Load(context.Background(), runner); !errors.Is(err, fonts.ErrUnavailable) {
		t.Errorf("err = %v, want ErrUnavailable", err)
	}
}

func TestFilterRanksPrefixMatchesFirst(t *testing.T) {
	catalog, err := fonts.Load(context.Background(), newRunner())
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	got := names(catalog.Filter("jetbrains", false))
	want := []string{"JetBrains Mono", "JetBrainsMono Nerd Font"}
	if !slices.Equal(got, want) {
		t.Errorf("filter = %v, want %v", got, want)
	}

	// A substring match still appears, but after the prefix matches.
	got = names(catalog.Filter("mono", false))
	if len(got) == 0 {
		t.Fatal("no match for a substring query")
	}
	if got[0] != "JetBrains Mono" && got[0] != "JetBrainsMono Nerd Font" {
		t.Errorf("first match = %q, want a JetBrains family", got[0])
	}
}

func TestFilterPutsMonospaceFirst(t *testing.T) {
	catalog, err := fonts.Load(context.Background(), newRunner())
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	got := catalog.Filter("", true)
	if len(got) != catalog.Len() {
		t.Fatalf("filter dropped families: %d of %d", len(got), catalog.Len())
	}

	seenProportional := false
	for _, f := range got {
		if !f.Mono {
			seenProportional = true
			continue
		}
		if seenProportional {
			t.Fatalf("monospaced %q appears after a proportional family: %v", f.Name, names(got))
		}
	}
}

func TestHasIsCaseInsensitive(t *testing.T) {
	catalog, err := fonts.Load(context.Background(), newRunner())
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if !catalog.Has("inter") || !catalog.Has("  Inter  ") {
		t.Error("Has should match case-insensitively and ignore surrounding space")
	}
	if catalog.Has("Comic Sans MS") {
		t.Error("Has should not invent families")
	}
}

func names(families []fonts.Family) []string {
	out := make([]string, 0, len(families))
	for _, f := range families {
		out = append(out, f.Name)
	}
	return out
}
