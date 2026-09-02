package theme

import (
	"strings"
	"testing"
)

func TestEncodeOmitsDerivedValues(t *testing.T) {
	th, err := ParseSource([]byte(`
name = "sparse"

[colors]
background = "#101010"
foreground = "#e0e0e0"
primary = "#5599ff"
`), "sparse.toml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	data, err := Encode(th)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	out := string(data)

	// Sixteen "#00000000" entries read as broken rather than as absent.
	if strings.Contains(out, "#00000000") {
		t.Errorf("encoded theme materialized unset colours:\n%s", out)
	}
	// A table with nothing left under it should go too.
	if strings.Contains(out, "[terminal]") {
		t.Errorf("encoded theme kept an empty table:\n%s", out)
	}
	for _, want := range []string{"background = '#101010'", "primary = '#5599ff'"} {
		if !strings.Contains(out, want) {
			t.Errorf("encoded theme is missing %q:\n%s", want, out)
		}
	}

	// What comes back must be what went in.
	again, err := ParseSource(data, "sparse.toml")
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if again.Colors != th.Colors || again.Terminal != th.Terminal {
		t.Error("encoding and parsing did not round-trip")
	}
}

func TestEncodeKeepsExplicitValues(t *testing.T) {
	th, err := ParseSource([]byte(`
name = "full"

[colors]
background = "#101010"
foreground = "#e0e0e0"
primary = "#5599ff"

[terminal]
regular = ["#111111", "#222222", "#333333", "#444444", "#555555", "#666666", "#777777", "#888888"]
cursor = "#ff00ff"
`), "full.toml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	data, err := Encode(th)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	out := string(data)

	if !strings.Contains(out, "[terminal]") {
		t.Errorf("a terminal table with keys should survive:\n%s", out)
	}
	if !strings.Contains(out, "'#111111'") {
		t.Errorf("an explicit ANSI palette should survive:\n%s", out)
	}
	// Only "bright", which was left unset, disappears.
	if strings.Contains(out, "bright = [") {
		t.Errorf("the unset bright palette should have gone:\n%s", out)
	}

	again, err := ParseSource(data, "full.toml")
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if again.Terminal.Regular != th.Terminal.Regular || again.Terminal.Cursor != th.Terminal.Cursor {
		t.Error("the explicit terminal palette did not round-trip")
	}
}

// A theme where only some ANSI slots are set keeps the array, holes included:
// those holes parse back as unset and are derived, so nothing is lost.
func TestEncodeKeepsPartialArrays(t *testing.T) {
	var th Theme
	th.Name = "partial"
	th.Colors.Background = MustParseColor("#101010")
	th.Colors.Foreground = MustParseColor("#e0e0e0")
	th.Colors.Primary = MustParseColor("#5599ff")
	th.Terminal.Regular[3] = MustParseColor("#ffcc00")

	data, err := Encode(th)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	again, err := ParseSource(data, "partial.toml")
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if again.Terminal.Regular[3] != th.Terminal.Regular[3] {
		t.Error("the set slot did not survive")
	}
	if !again.Terminal.Regular[0].IsZero() {
		t.Error("an unset slot should come back unset")
	}

	// And the holes still derive.
	resolved := again.Resolved()
	if resolved.Terminal.Regular[0].IsZero() {
		t.Error("normalization should have filled the hole")
	}
	if resolved.Terminal.Regular[3] != th.Terminal.Regular[3] {
		t.Error("normalization overwrote an explicit slot")
	}
}
