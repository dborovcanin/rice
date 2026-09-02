package session

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dborovcanin/rice/internal/diff"
	"github.com/dborovcanin/rice/internal/generation"
)

// ErrNothingToCompare is returned when there is no committed generation for
// the draft to be measured against.
var ErrNothingToCompare = errors.New("nothing is deployed yet to compare against")

// Diff renders the draft and reports what it would change about the generation
// currently deployed, as a unified diff. It writes nothing.
//
// This answers the question the editor cannot otherwise answer: not what the
// draft says, but what it would do.
func (s *Session) Diff(context int) (string, error) {
	if s.currentDir == "" {
		return "", ErrNothingToCompare
	}
	if _, err := os.Stat(s.currentDir); err != nil {
		return "", ErrNothingToCompare
	}

	// Render with the deployed generation's own number. Every generated file
	// carries that number in a header comment, so a different one would report
	// a change in every file and bury the ones that matter.
	number := 0
	if manifest, err := generation.ReadManifest(s.currentDir); err == nil {
		number = manifest.Generation
	}

	files, err := s.builder.Render(s.resolved.Config, s.resolved.Theme, number)
	if err != nil {
		return "", err
	}

	next := make(map[string]string, len(files))
	paths := make([]string, 0, len(files))
	for _, f := range files {
		next[f.Path] = string(f.Content)
		paths = append(paths, f.Path)
	}
	sort.Strings(paths)

	var b strings.Builder
	for _, path := range paths {
		old, err := os.ReadFile(filepath.Join(s.currentDir, filepath.FromSlash(path)))
		if err != nil {
			// A file the deployed generation does not have is entirely new,
			// which is worth showing rather than skipping.
			old = nil
		}
		b.WriteString(diff.Unified("deployed/"+path, path, string(old), next[path], context))
	}
	return b.String(), nil
}
