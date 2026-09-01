package theme

import "testing"

func TestParseColor(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Color
		wantErr bool
	}{
		{name: "six digits", input: "#282828", want: Color{0x28, 0x28, 0x28, 0xff}},
		{name: "no hash", input: "282828", want: Color{0x28, 0x28, 0x28, 0xff}},
		{name: "three digits", input: "#fff", want: Color{0xff, 0xff, 0xff, 0xff}},
		{name: "eight digits", input: "#28282880", want: Color{0x28, 0x28, 0x28, 0x80}},
		{name: "four digits", input: "#f008", want: Color{0xff, 0x00, 0x00, 0x88}},
		{name: "surrounding space", input: "  #ebdbb2 ", want: Color{0xeb, 0xdb, 0xb2, 0xff}},
		{name: "wrong length", input: "#12345", wantErr: true},
		{name: "not hex", input: "#gggggg", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseColor(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseColor(%q) = %v, want error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseColor(%q): %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseColor(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestColorFormats(t *testing.T) {
	c := MustParseColor("#d79921")

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"String", c.String(), "#d79921"},
		{"String with alpha", c.Alpha(0.5).String(), "#d7992180"},
		{"Hex drops alpha", c.Alpha(0.5).Hex(), "#d79921"},
		{"Bare", c.Bare(), "d79921"},
		{"BareA", c.Alpha(0.7).BareA(), "d79921b3"},
		{"ARGB", c.ARGB(), "ffd79921"},
		{"RGB", c.RGB(), "rgb(215, 153, 33)"},
		{"RGBA", c.Alpha(0.5).RGBA(), "rgba(215, 153, 33, 0.5019607843137255)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

func TestColorDerivation(t *testing.T) {
	black := MustParseColor("#000000")
	white := MustParseColor("#ffffff")

	if got := black.Lighten(1); got != white {
		t.Errorf("Lighten(1) = %v, want white", got)
	}
	if got := white.Darken(1); got != black {
		t.Errorf("Darken(1) = %v, want black", got)
	}
	if got := black.Lighten(0); got != black {
		t.Errorf("Lighten(0) = %v, want unchanged", got)
	}
	if got := black.Mix(white, 0.5).Hex(); got != "#808080" {
		t.Errorf("Mix(white, 0.5) = %s, want #808080", got)
	}

	// Clamping keeps out-of-range fractions from wrapping around.
	if got := black.Lighten(5); got != white {
		t.Errorf("Lighten(5) = %v, want white", got)
	}
	if got := white.Darken(-1); got != white {
		t.Errorf("Darken(-1) = %v, want unchanged", got)
	}

	if !black.IsDark() || white.IsDark() {
		t.Error("IsDark disagrees with luminance")
	}
	if black.Contrast() != white || white.Contrast() != black {
		t.Error("Contrast should return the readable extreme")
	}
}

func TestColorAlphaRoundTrip(t *testing.T) {
	c := MustParseColor("#1e1e2e").Alpha(0.5)
	parsed, err := ParseColor(c.String())
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if parsed != c {
		t.Errorf("round trip changed color: %+v -> %+v", c, parsed)
	}
}

func TestColorUnmarshalText(t *testing.T) {
	var c Color
	if err := c.UnmarshalText([]byte("#89b4fa")); err != nil {
		t.Fatalf("UnmarshalText: %v", err)
	}
	if c != MustParseColor("#89b4fa") {
		t.Errorf("got %v", c)
	}
	if err := c.UnmarshalText([]byte("nope")); err == nil {
		t.Error("want error for invalid color")
	}
}
