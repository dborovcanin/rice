// Package fonts enumerates the font families installed on the system. It
// shells out to fc-list rather than linking fontconfig, so Rice keeps building
// with CGO_ENABLED=0.
package fonts

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/dborovcanin/rice/internal/command"
)

// ErrUnavailable is returned when fontconfig is not installed. Font selection
// degrades to typing a family name by hand rather than failing outright.
var ErrUnavailable = errors.New("fc-list not found: install fontconfig to browse installed fonts")

// Family is one installed font family.
type Family struct {
	Name string
	// Mono reports that fontconfig considers the family monospaced, which is
	// what the terminal and bar roles want.
	Mono bool
}

// Catalog is every family fontconfig knows about, sorted by name. It is loaded
// once and reused: enumerating fonts is fast but not free, and the set does
// not change while an editing session is open.
type Catalog struct {
	families []Family
}

// Load enumerates installed families through fc-list.
func Load(ctx context.Context, runner command.Runner) (Catalog, error) {
	if !runner.Look("fc-list") {
		return Catalog{}, ErrUnavailable
	}

	all, err := query(ctx, runner, "")
	if err != nil {
		return Catalog{}, err
	}
	mono, err := query(ctx, runner, ":spacing=100")
	if err != nil {
		return Catalog{}, err
	}

	isMono := make(map[string]bool, len(mono))
	for _, name := range mono {
		isMono[name] = true
	}

	families := make([]Family, 0, len(all))
	for _, name := range all {
		families = append(families, Family{Name: name, Mono: isMono[name]})
	}
	sort.Slice(families, func(i, j int) bool {
		return strings.ToLower(families[i].Name) < strings.ToLower(families[j].Name)
	})
	return Catalog{families: families}, nil
}

// query runs fc-list over a fontconfig pattern and returns deduplicated
// primary family names.
func query(ctx context.Context, runner command.Runner, pattern string) ([]string, error) {
	args := []string{}
	if pattern != "" {
		args = append(args, pattern)
	}
	args = append(args, `--format=%{family[0]}\n`)

	out, err := runner.Output(ctx, "fc-list", args...)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var names []string
	for line := range strings.SplitSeq(string(out), "\n") {
		name := strings.TrimSpace(line)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names, nil
}

// Len is how many families the catalog holds.
func (c Catalog) Len() int { return len(c.families) }

// All returns every family.
func (c Catalog) All() []Family { return c.families }

// Has reports whether a family is installed, matched case-insensitively
// because that is how fontconfig resolves names.
func (c Catalog) Has(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, f := range c.families {
		if strings.ToLower(f.Name) == name {
			return true
		}
	}
	return false
}

// Filter returns the families matching a query. Matching is a case-insensitive
// substring test, with families whose name starts with the query first, so
// typing "jet" surfaces "JetBrainsMono Nerd Font" ahead of anything that
// merely contains it.
//
// With monoFirst set, monospaced families are grouped ahead of the rest, which
// is what the terminal and bar font roles want.
func (c Catalog) Filter(query string, monoFirst bool) []Family {
	q := strings.ToLower(strings.TrimSpace(query))

	var prefix, contains []Family
	for _, f := range c.families {
		if q == "" {
			prefix = append(prefix, f)
			continue
		}
		lower := strings.ToLower(f.Name)
		switch {
		case strings.HasPrefix(lower, q):
			prefix = append(prefix, f)
		case strings.Contains(lower, q):
			contains = append(contains, f)
		}
	}

	out := append(prefix, contains...)
	if !monoFirst {
		return out
	}

	// A stable partition keeps the prefix-before-substring ordering inside
	// each half.
	mono := make([]Family, 0, len(out))
	rest := make([]Family, 0, len(out))
	for _, f := range out {
		if f.Mono {
			mono = append(mono, f)
		} else {
			rest = append(rest, f)
		}
	}
	return append(mono, rest...)
}
