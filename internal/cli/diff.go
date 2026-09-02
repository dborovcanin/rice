package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dborovcanin/rice/internal/diff"
	"github.com/dborovcanin/rice/internal/generation"
)

func newDiffCmd(app func() *App) *cobra.Command {
	var (
		themeName string
		component string
		stat      bool
		context   int
	)

	cmd := &cobra.Command{
		Use:   "diff [generation]",
		Short: "Show what applying would change",
		Long: "Compares the generation `current` points at with what Rice would\n" +
			"generate now, and prints a unified diff.\n\n" +
			"This is the question worth asking before `rice apply`: not what the\n" +
			"configuration says, but what would actually change on disk. Nothing is\n" +
			"written and nothing is switched.\n\n" +
			"With a generation number, that generation is the left-hand side\n" +
			"instead, so two points in history can be compared.",
		Example: `  # What would applying do?
  rice diff

  # Just the shape of it.
  rice diff --stat

  # What would this theme change?
  rice diff --theme tokyo-night

  # One component, in full.
  rice diff -c waybar

  # Against an older generation.
  rice diff 39`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := app()

			cfg, err := a.Config()
			if err != nil {
				return err
			}
			th, err := a.Theme(cfg, themeName)
			if err != nil {
				return err
			}
			if component != "" {
				if _, err := a.Registry.Get(component); err != nil {
					return err
				}
			}

			base, label, err := resolveDiffBase(a, args)
			if err != nil {
				return err
			}

			// The right-hand side is rendered with the *base* generation's
			// number. Every generated file carries its number in a header
			// comment, so numbering the new side differently would report a
			// change in every file and bury the ones that matter. The number
			// always changes; that is not news.
			number, err := baseNumber(a, base)
			if err != nil {
				return err
			}
			files, err := a.Builder.Render(cfg, th, number)
			if err != nil {
				return err
			}

			return writeDiff(cmd.OutOrStdout(), diffOptions{
				base:      base,
				baseLabel: label,
				files:     files,
				component: component,
				stat:      stat,
				context:   context,
			})
		},
	}

	cmd.Flags().StringVar(&themeName, "theme", "", "compare against another theme (defaults to config.toml)")
	cmd.Flags().StringVarP(&component, "component", "c", "", "only this component")
	cmd.Flags().BoolVar(&stat, "stat", false, "show a summary instead of the diff")
	cmd.Flags().IntVar(&context, "context", diff.DefaultContext, "unchanged lines to show around a change")
	return cmd
}

// resolveDiffBase works out which directory the comparison is against, and
// what to call it in the output.
func resolveDiffBase(a *App, args []string) (dir, label string, err error) {
	if len(args) == 1 {
		n, err := strconv.Atoi(args[0])
		if err != nil {
			return "", "", fmt.Errorf("invalid generation %q", args[0])
		}
		dir := a.Paths.Generation(n)
		if _, err := os.Stat(dir); err != nil {
			return "", "", fmt.Errorf("generation %d: %w", n, err)
		}
		return dir, fmt.Sprintf("generation %06d", n), nil
	}

	current, err := a.Store.Current()
	switch {
	case errors.Is(err, generation.ErrPreviewActive):
		return a.Paths.PreviewDir, "preview", nil
	case errors.Is(err, generation.ErrNoGenerations):
		// Nothing to compare against is not a failure: everything is new.
		return "", "nothing", nil
	case err != nil:
		return "", "", err
	}
	return a.Paths.Generation(current), fmt.Sprintf("generation %06d", current), nil
}

// baseNumber is the generation number to render the comparison with: the
// base's own, so its header lines match, or the next one when there is no base
// to match.
func baseNumber(a *App, base string) (int, error) {
	if base != "" {
		if manifest, err := generation.ReadManifest(base); err == nil && manifest.Generation != 0 {
			return manifest.Generation, nil
		}
	}
	return a.Store.Next()
}

type diffOptions struct {
	base      string
	baseLabel string
	files     []generation.Rendered
	component string
	stat      bool
	context   int
}

// writeDiff compares a rendered set against a directory on disk.
func writeDiff(out io.Writer, opts diffOptions) error {
	// Both sides are keyed by path, so a file that only exists on one side is
	// as visible as one that changed.
	next := map[string]string{}
	for _, f := range opts.files {
		if opts.component != "" && f.Component != opts.component {
			continue
		}
		next[f.Path] = string(f.Content)
	}

	previous := map[string]string{}
	if opts.base != "" {
		found, err := readTree(opts.base)
		if err != nil {
			return err
		}
		for path, content := range found {
			if opts.component != "" && componentOf(path) != opts.component {
				continue
			}
			previous[path] = content
		}
	}

	paths := make([]string, 0, len(next)+len(previous))
	seen := map[string]bool{}
	for path := range next {
		paths, seen[path] = append(paths, path), true
	}
	for path := range previous {
		if !seen[path] {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)

	changed := 0
	for _, path := range paths {
		old, hadOld := previous[path]
		new, hasNew := next[path]

		switch {
		case !hadOld:
			changed++
			added, _ := diff.Stat("", new)
			if opts.stat {
				fmt.Fprintf(out, "  new      %s (+%d)\n", path, added)
				continue
			}
			fmt.Fprint(out, diff.Unified("/dev/null", path, "", new, opts.context))

		case !hasNew:
			changed++
			_, removed := diff.Stat(old, "")
			if opts.stat {
				fmt.Fprintf(out, "  removed  %s (-%d)\n", path, removed)
				continue
			}
			fmt.Fprint(out, diff.Unified(path, "/dev/null", old, "", opts.context))

		case old != new:
			changed++
			added, removed := diff.Stat(old, new)
			if opts.stat {
				fmt.Fprintf(out, "  changed  %s (+%d -%d)\n", path, added, removed)
				continue
			}
			fmt.Fprint(out, diff.Unified(opts.baseLabel+"/"+path, path, old, new, opts.context))
		}
	}

	if changed == 0 {
		fmt.Fprintf(out, "No change against %s.\n", opts.baseLabel)
		return nil
	}
	if opts.stat {
		fmt.Fprintf(out, "\n%d file(s) differ from %s.\n", changed, opts.baseLabel)
	}
	return nil
}

// readTree reads every file under dir, keyed by its slash-separated path
// relative to dir. The manifest is skipped: it records hashes and a timestamp,
// so it differs on every build and says nothing about the configuration.
func readTree(dir string) (map[string]string, error) {
	out := map[string]string{}

	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == generation.ManifestName {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[rel] = string(data)
		return nil
	})
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	return out, nil
}

// componentOf is the component a generated path belongs to, which is its first
// path element.
func componentOf(path string) string {
	component, _, ok := strings.Cut(path, "/")
	if !ok {
		return ""
	}
	return component
}
