package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// Shell completion for Rice is only useful if it knows the things Rice knows:
// which themes exist, which components are enabled, which generations are on
// disk. Cobra supplies the plumbing; these supply the answers.
//
// A completion function runs in a separate process from the command it
// completes, and before the usual setup has happened, so each one resolves the
// application itself rather than relying on the shared instance.

// completionApp resolves an App for a completion function, falling back to the
// default root when the shared one has not been built yet.
func completionApp(app func() *App) *App {
	if a := app(); a != nil {
		return a
	}
	a, err := NewApp("", "")
	if err != nil {
		return nil
	}
	return a
}

// completeThemes completes a theme name from the bundled and user themes.
func completeThemes(app func() *App) completionFunc {
	return func(cmd *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
		a := completionApp(app)
		if a == nil {
			return nil, cobra.ShellCompDirectiveError
		}

		entries, err := a.Themes.List()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		var out []string
		for _, e := range entries {
			if strings.HasPrefix(e.Name, prefix) {
				out = append(out, fmt.Sprintf("%s\t%s theme", e.Name, e.Source))
			}
		}
		// A theme may also be named by path, so leave file completion on.
		return out, cobra.ShellCompDirectiveNoSpace
	}
}

// completeComponents completes an enabled component name. Offering a disabled
// one would only produce an error a moment later.
func completeComponents(app func() *App) completionFunc {
	return func(cmd *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
		a := completionApp(app)
		if a == nil {
			return nil, cobra.ShellCompDirectiveError
		}

		cfg, err := a.Config()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		var out []string
		for _, name := range cfg.Components.Names() {
			if strings.HasPrefix(name, prefix) {
				out = append(out, name)
			}
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}

// completeGenerations completes a generation number, newest first, annotated
// with the theme it was built from.
func completeGenerations(app func() *App) completionFunc {
	return func(cmd *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		a := completionApp(app)
		if a == nil {
			return nil, cobra.ShellCompDirectiveError
		}

		list, err := a.Store.List()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		current, _ := a.Store.Current()

		var out []string
		for i := len(list) - 1; i >= 0; i-- {
			info := list[i]
			number := fmt.Sprint(info.Number)
			if !strings.HasPrefix(number, prefix) {
				continue
			}

			note := info.Manifest.Theme
			if info.Number == current {
				note += " (current)"
			}
			out = append(out, number+"\t"+note)
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}

// completionFunc is the signature Cobra wants, named so the registrations
// below fit on one line each.
type completionFunc func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective)

// registerCompletions wires the dynamic completions onto the command tree. It
// is called once, from NewRootCmd, so that adding a command does not mean
// remembering to complete it somewhere else.
func registerCompletions(root *cobra.Command, app func() *App) {
	themes := completeThemes(app)
	components := completeComponents(app)
	generations := completeGenerations(app)

	// A --theme flag means the same thing wherever it appears.
	forEachCommand(root, func(cmd *cobra.Command) {
		if cmd.Flag("theme") != nil {
			_ = cmd.RegisterFlagCompletionFunc("theme", themes)
		}
		if cmd.Flag("component") != nil {
			_ = cmd.RegisterFlagCompletionFunc("component", components)
		}
	})

	if c, _, err := root.Find([]string{"theme", "apply"}); err == nil {
		c.ValidArgsFunction = themes
	}
	if c, _, err := root.Find([]string{"theme", "show"}); err == nil {
		c.ValidArgsFunction = themes
	}
	if c, _, err := root.Find([]string{"rollback"}); err == nil {
		c.ValidArgsFunction = generations
	}
	if c, _, err := root.Find([]string{"generation", "show"}); err == nil {
		c.ValidArgsFunction = generations
	}
}

// forEachCommand visits every command in the tree, including the root.
func forEachCommand(cmd *cobra.Command, fn func(*cobra.Command)) {
	fn(cmd)
	for _, child := range cmd.Commands() {
		forEachCommand(child, fn)
	}
}
