package diff_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dborovcanin/rice/internal/diff"
)

func TestIdenticalTextHasNoDiff(t *testing.T) {
	const text = "one\ntwo\nthree\n"
	if got := diff.Unified("a", "b", text, text, 3); got != "" {
		t.Errorf("identical text produced a diff:\n%s", got)
	}
	if added, removed := diff.Stat(text, text); added != 0 || removed != 0 {
		t.Errorf("stat = +%d -%d, want nothing", added, removed)
	}
}

func TestStatCountsChangedLines(t *testing.T) {
	old := "a\nb\nc\n"
	new := "a\nB\nc\nd\n"

	added, removed := diff.Stat(old, new)
	if added != 2 || removed != 1 {
		t.Errorf("stat = +%d -%d, want +2 -1", added, removed)
	}
}

func TestUnifiedMarksEachSide(t *testing.T) {
	out := diff.Unified("old", "new", "a\nb\nc\n", "a\nB\nc\n", 1)

	if !strings.HasPrefix(out, "--- old\n+++ new\n") {
		t.Errorf("missing file headers:\n%s", out)
	}
	for _, want := range []string{"-b", "+B", " a", " c"} {
		if !strings.Contains(out, want+"\n") {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

// Two changes far apart are two hunks; two changes close together are one.
func TestHunksMergeWhenContextOverlaps(t *testing.T) {
	// Distinct lines, so there is only one sensible alignment to find.
	numbered := func(replace map[int]string) string {
		var b strings.Builder
		for i := range 40 {
			if s, ok := replace[i]; ok {
				b.WriteString(s + "\n")
				continue
			}
			fmt.Fprintf(&b, "line %d\n", i)
		}
		return b.String()
	}

	old := numbered(nil)

	far := diff.Unified("a", "b", old, numbered(map[int]string{2: "X", 30: "Y"}), 3)
	if got := hunks(far); got != 2 {
		t.Errorf("distant changes = %d hunks, want 2:\n%s", got, far)
	}

	near := diff.Unified("a", "b", old, numbered(map[int]string{2: "X", 4: "Y"}), 3)
	if got := hunks(near); got != 1 {
		t.Errorf("nearby changes = %d hunks, want 1:\n%s", got, near)
	}
}

// hunks counts the hunk headers in a unified diff.
func hunks(out string) int {
	n := 0
	for line := range strings.SplitSeq(out, "\n") {
		if strings.HasPrefix(line, "@@ ") {
			n++
		}
	}
	return n
}

func TestEmptySides(t *testing.T) {
	out := diff.Unified("a", "b", "", "x\ny\n", 3)
	if !strings.Contains(out, "+x") || !strings.Contains(out, "+y") {
		t.Errorf("adding to an empty file:\n%s", out)
	}

	out = diff.Unified("a", "b", "x\ny\n", "", 3)
	if !strings.Contains(out, "-x") || !strings.Contains(out, "-y") {
		t.Errorf("emptying a file:\n%s", out)
	}
}

// The output has to be a real unified diff, not something that merely looks
// like one, so it is checked against `patch`: applying it to the old text must
// produce the new text exactly.
func TestOutputIsAppliablePatch(t *testing.T) {
	patch, err := exec.LookPath("patch")
	if err != nil {
		t.Skip("patch is not installed")
	}

	cases := []struct{ name, old, new string }{
		{"change one line", "a\nb\nc\n", "a\nB\nc\n"},
		{"insert at the top", "a\nb\n", "z\na\nb\n"},
		{"delete at the top", "z\na\nb\n", "a\nb\n"},
		{"append", "a\nb\n", "a\nb\nc\n"},
		{"truncate", "a\nb\nc\n", "a\nb\n"},
		{"two distant changes", longText(40, map[int]string{3: "x", 32: "y"}), longText(40, map[int]string{3: "X", 32: "Y"})},
		{"replace everything", "a\nb\nc\n", "x\ny\nz\n"},
		{"insert a block", "a\nb\nc\n", "a\nnew1\nnew2\nnew3\nb\nc\n"},
		{"interleaved", "a\nb\nc\nd\ne\n", "a\nc\nX\ne\nf\n"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := diff.Unified("old", "new", c.old, c.new, 3)
			if out == "" {
				t.Fatal("no diff was produced for different inputs")
			}

			dir := t.TempDir()
			target := filepath.Join(dir, "file")
			if err := os.WriteFile(target, []byte(c.old), 0o644); err != nil {
				t.Fatal(err)
			}

			cmd := exec.Command(patch, "--quiet", target)
			cmd.Stdin = strings.NewReader(out)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("patch refused the diff: %v\n%s\n--- diff ---\n%s", err, output, out)
			}

			got, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != c.new {
				t.Errorf("patched file = %q, want %q\n--- diff ---\n%s", got, c.new, out)
			}
		})
	}
}

// longText builds numbered lines with the given replacements, for cases that
// need more than a handful of lines.
func longText(n int, replace map[int]string) string {
	var b strings.Builder
	for i := range n {
		if s, ok := replace[i]; ok {
			b.WriteString(s + "\n")
			continue
		}
		b.WriteString("line\n")
	}
	return b.String()
}
