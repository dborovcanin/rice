// Package diff produces unified diffs of generated configuration.
//
// Rice's whole promise is that you can see what it would do before it does it,
// and "run rice render into a temporary directory and diff it yourself" is a
// workaround, not an answer. This is small enough to own: the inputs are
// generated text files of a few hundred lines, not arbitrary source trees.
package diff

import (
	"fmt"
	"strings"
)

// DefaultContext is how many unchanged lines surround a change. Three is what
// every other diff shows, so it is what eyes expect.
const DefaultContext = 3

// Op is what happened to a line.
type Op int

const (
	// Equal means the line is in both versions.
	Equal Op = iota
	// Delete means the line is only in the old version.
	Delete
	// Insert means the line is only in the new version.
	Insert
)

// Line is one line of an edit script.
type Line struct {
	Op   Op
	Text string
}

// Lines computes the edit script between two texts, line by line.
func Lines(old, new string) []Line {
	return diffSlices(splitLines(old), splitLines(new))
}

// Unified renders the difference between two files the way `diff -u` does, and
// returns the empty string when they are identical.
func Unified(oldName, newName, old, new string, context int) string {
	if old == new {
		return ""
	}
	if context < 0 {
		context = DefaultContext
	}

	lines := diffSlices(splitLines(old), splitLines(new))
	hunks := group(lines, context)
	if len(hunks) == 0 {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "--- %s\n+++ %s\n", oldName, newName)
	for _, h := range hunks {
		fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", h.oldStart, h.oldCount, h.newStart, h.newCount)
		for _, line := range h.lines {
			switch line.Op {
			case Delete:
				b.WriteByte('-')
			case Insert:
				b.WriteByte('+')
			default:
				b.WriteByte(' ')
			}
			b.WriteString(line.Text)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// Stat counts the changed lines, for a summary that does not need the diff.
func Stat(old, new string) (added, removed int) {
	for _, line := range Lines(old, new) {
		switch line.Op {
		case Insert:
			added++
		case Delete:
			removed++
		}
	}
	return added, removed
}

// splitLines splits text into lines without a trailing empty element, so a
// file that ends in a newline does not appear to have a blank last line.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.TrimSuffix(s, "\n")
	return strings.Split(s, "\n")
}

// diffSlices is the edit script between two line slices.
//
// The common prefix and suffix are trimmed first. Generated configuration
// changes in a few places and matches everywhere else, so this usually reduces
// the quadratic part to almost nothing.
func diffSlices(a, b []string) []Line {
	var out []Line

	prefix := 0
	for prefix < len(a) && prefix < len(b) && a[prefix] == b[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(a)-prefix && suffix < len(b)-prefix &&
		a[len(a)-1-suffix] == b[len(b)-1-suffix] {
		suffix++
	}

	for _, line := range a[:prefix] {
		out = append(out, Line{Op: Equal, Text: line})
	}
	out = append(out, middle(a[prefix:len(a)-suffix], b[prefix:len(b)-suffix])...)
	for _, line := range a[len(a)-suffix:] {
		out = append(out, Line{Op: Equal, Text: line})
	}
	return out
}

// middle diffs the parts that are not a shared prefix or suffix, by longest
// common subsequence.
func middle(a, b []string) []Line {
	switch {
	case len(a) == 0 && len(b) == 0:
		return nil
	case len(a) == 0:
		out := make([]Line, 0, len(b))
		for _, line := range b {
			out = append(out, Line{Op: Insert, Text: line})
		}
		return out
	case len(b) == 0:
		out := make([]Line, 0, len(a))
		for _, line := range a {
			out = append(out, Line{Op: Delete, Text: line})
		}
		return out
	}

	// lengths[i][j] is the LCS length of a[i:] and b[j:]. Filling it backwards
	// means the walk that follows can go forwards, which keeps deletions
	// before insertions in the output and reads more naturally.
	lengths := make([][]int, len(a)+1)
	for i := range lengths {
		lengths[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lengths[i][j] = lengths[i+1][j+1] + 1
			} else {
				lengths[i][j] = max(lengths[i+1][j], lengths[i][j+1])
			}
		}
	}

	var out []Line
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			out = append(out, Line{Op: Equal, Text: a[i]})
			i++
			j++
		case lengths[i+1][j] >= lengths[i][j+1]:
			out = append(out, Line{Op: Delete, Text: a[i]})
			i++
		default:
			out = append(out, Line{Op: Insert, Text: b[j]})
			j++
		}
	}
	for ; i < len(a); i++ {
		out = append(out, Line{Op: Delete, Text: a[i]})
	}
	for ; j < len(b); j++ {
		out = append(out, Line{Op: Insert, Text: b[j]})
	}
	return out
}

// hunk is a run of changes with its surrounding context.
type hunk struct {
	oldStart, oldCount int
	newStart, newCount int
	lines              []Line
}

// group collects changed lines into hunks, each padded with up to context
// unchanged lines and merged with its neighbour when their context overlaps.
func group(lines []Line, context int) []hunk {
	changed := make([]int, 0, len(lines))
	for i, line := range lines {
		if line.Op != Equal {
			changed = append(changed, i)
		}
	}
	if len(changed) == 0 {
		return nil
	}

	// Line numbers are one-based and count each side separately.
	oldNum := make([]int, len(lines))
	newNum := make([]int, len(lines))
	o, n := 0, 0
	for i, line := range lines {
		if line.Op != Insert {
			o++
		}
		if line.Op != Delete {
			n++
		}
		oldNum[i], newNum[i] = o, n
	}

	var hunks []hunk
	start := max(changed[0]-context, 0)
	end := min(changed[0]+context, len(lines)-1)

	flush := func() {
		h := hunk{lines: lines[start : end+1]}
		for _, line := range h.lines {
			if line.Op != Insert {
				h.oldCount++
			}
			if line.Op != Delete {
				h.newCount++
			}
		}

		// A hunk that only inserts sits after the previous old line, and one
		// that only deletes sits after the previous new line; the counts above
		// already say which, so the start is taken from the first line's
		// numbering minus what it consumed.
		h.oldStart = oldNum[start]
		if lines[start].Op != Insert {
			h.oldStart--
		}
		h.newStart = newNum[start]
		if lines[start].Op != Delete {
			h.newStart--
		}
		h.oldStart++
		h.newStart++
		if h.oldCount == 0 {
			h.oldStart--
		}
		if h.newCount == 0 {
			h.newStart--
		}
		hunks = append(hunks, h)
	}

	for _, at := range changed[1:] {
		if at-context <= end+1 {
			// Close enough that the context overlaps: one hunk, not two.
			end = min(at+context, len(lines)-1)
			continue
		}
		flush()
		start = max(at-context, 0)
		end = min(at+context, len(lines)-1)
	}
	flush()
	return hunks
}
