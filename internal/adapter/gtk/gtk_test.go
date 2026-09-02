package gtk_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/dborovcanin/rice/internal/adapter"
	"github.com/dborovcanin/rice/internal/adapter/gtk"
	"github.com/dborovcanin/rice/internal/config"
)

func TestFilesFollowTheConfiguration(t *testing.T) {
	a := gtk.New()

	cases := []struct {
		name          string
		settings, css bool
		wantFiles     []string
		wantTargets   []string
	}{
		{
			name: "settings only", settings: true,
			wantFiles: []string{"gtk/settings.ini"},
			// One rendered file, linked into both GTK version directories.
			wantTargets: []string{"gtk-3.0/settings.ini", "gtk-4.0/settings.ini"},
		},
		{
			name: "settings and css", settings: true, css: true,
			wantFiles: []string{"gtk/settings.ini", "gtk/gtk.css"},
			wantTargets: []string{
				"gtk-3.0/settings.ini", "gtk-3.0/gtk.css",
				"gtk-4.0/settings.ini", "gtk-4.0/gtk.css",
			},
		},
		{
			name: "nothing", wantFiles: nil, wantTargets: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := config.Config{GTK: config.GTK{Settings: c.settings, CSS: c.css}}

			var files []string
			for _, f := range adapter.FilesOf(a, cfg) {
				files = append(files, f.Path)
			}
			if !slices.Equal(files, c.wantFiles) {
				t.Errorf("files = %v, want %v", files, c.wantFiles)
			}

			var targets []string
			for _, p := range adapter.ConfigPathsOf(a, cfg) {
				targets = append(targets, filepath.ToSlash(p.Target))
			}
			if !slices.Equal(targets, c.wantTargets) {
				t.Errorf("targets = %v, want %v", targets, c.wantTargets)
			}
		})
	}
}

// Validate must tolerate a generation that legitimately lacks a file, because
// which files exist is a configuration decision.
func TestValidateIgnoresAbsentFiles(t *testing.T) {
	if err := gtk.New().Validate(t.TempDir()); err != nil {
		t.Errorf("validating an empty generation: %v", err)
	}
}
