package qt_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/dborovcanin/rice/internal/adapter"
	"github.com/dborovcanin/rice/internal/adapter/qt"
	"github.com/dborovcanin/rice/internal/config"
)

func TestFilesFollowTheConfiguration(t *testing.T) {
	a := qt.New()

	cases := []struct {
		name        string
		cfg         config.Qt
		wantFiles   []string
		wantTargets []string
	}{
		{
			name:        "qt5 only",
			cfg:         config.Qt{Qt5ct: true},
			wantFiles:   []string{"qt/qt5ct.conf"},
			wantTargets: []string{"qt5ct/qt5ct.conf"},
		},
		{
			name:      "everything",
			cfg:       config.Qt{Qt5ct: true, Qt6ct: true, Kvantum: true},
			wantFiles: []string{"qt/qt5ct.conf", "qt/qt6ct.conf", "qt/kvantum.kvconfig"},
			wantTargets: []string{
				"qt5ct/qt5ct.conf", "qt6ct/qt6ct.conf", "Kvantum/kvantum.kvconfig",
			},
		},
		{
			name: "nothing",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := config.Config{Qt: c.cfg}

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

func TestValidateIgnoresAbsentFiles(t *testing.T) {
	if err := qt.New().Validate(t.TempDir()); err != nil {
		t.Errorf("validating an empty generation: %v", err)
	}
}
