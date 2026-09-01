package command

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestExecRunsAndCapturesOutput(t *testing.T) {
	e := New()

	out, err := e.Output(context.Background(), "echo", "hello")
	if err != nil {
		t.Fatalf("Output: %v", err)
	}
	if strings.TrimSpace(string(out)) != "hello" {
		t.Errorf("output = %q", out)
	}
}

func TestExecReportsFailureWithStderr(t *testing.T) {
	e := New()

	err := e.Run(context.Background(), "sh", "-c", "echo boom >&2; exit 3")
	if err == nil {
		t.Fatal("want error")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error should carry stderr, got: %v", err)
	}
	if !strings.Contains(err.Error(), "sh -c") {
		t.Errorf("error should name the command, got: %v", err)
	}
}

func TestExecLook(t *testing.T) {
	e := New()
	if !e.Look("sh") {
		t.Error("sh should be found")
	}
	if e.Look("definitely-not-a-real-program-xyzzy") {
		t.Error("nonexistent program should not be found")
	}
}

func TestDescribe(t *testing.T) {
	if got := Describe("swaymsg", []string{"reload"}); got != "swaymsg reload" {
		t.Errorf("got %q", got)
	}
	if got := Describe("waybar", nil); got != "waybar" {
		t.Errorf("got %q", got)
	}
}

func TestFakeRecordsCalls(t *testing.T) {
	f := NewFake()
	f.Outputs = map[string]string{"swaymsg": "ok"}
	f.Errors = map[string]error{"dunstctl": errors.New("nope")}
	f.Missing = map[string]bool{"pgrep": true}

	out, err := f.Output(context.Background(), "swaymsg", "reload")
	if err != nil || string(out) != "ok" {
		t.Fatalf("Output = %q, %v", out, err)
	}
	if err := f.Run(context.Background(), "dunstctl", "reload"); err == nil {
		t.Error("want the canned error")
	}
	if f.Look("pgrep") {
		t.Error("pgrep should be reported missing")
	}
	if !f.Look("swaymsg") {
		t.Error("swaymsg should be reported present")
	}

	want := []string{"swaymsg reload", "dunstctl reload"}
	got := f.Commands()
	if len(got) != len(want) {
		t.Fatalf("commands = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("commands = %v, want %v", got, want)
		}
	}
}
