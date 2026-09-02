package clipboard_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dborovcanin/rice/internal/clipboard"
	"github.com/dborovcanin/rice/internal/command"
)

func TestCopyPrefersWayland(t *testing.T) {
	runner := command.NewFake()

	tool, err := clipboard.Copy(context.Background(), runner, []byte("hello"))
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if tool != "wl-copy" {
		t.Errorf("tool = %q, want wl-copy", tool)
	}
	if len(runner.Calls) != 1 {
		t.Fatalf("calls = %v, want one", runner.Commands())
	}
	if got := string(runner.Calls[0].Stdin); got != "hello" {
		t.Errorf("stdin = %q, want hello", got)
	}
}

func TestCopyFallsBackToX11Tools(t *testing.T) {
	runner := command.NewFake()
	runner.Missing = map[string]bool{"wl-copy": true}

	tool, err := clipboard.Copy(context.Background(), runner, []byte("hello"))
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if tool != "xclip" {
		t.Errorf("tool = %q, want xclip", tool)
	}
	if got := runner.Commands()[0]; got != "xclip -selection clipboard" {
		t.Errorf("command = %q", got)
	}

	runner = command.NewFake()
	runner.Missing = map[string]bool{"wl-copy": true, "xclip": true}
	if tool, err := clipboard.Copy(context.Background(), runner, nil); err != nil || tool != "xsel" {
		t.Errorf("tool = %q, err = %v, want xsel", tool, err)
	}
}

func TestCopyReportsNoToolAtAll(t *testing.T) {
	runner := command.NewFake()
	runner.Missing = map[string]bool{"wl-copy": true, "xclip": true, "xsel": true}

	if _, err := clipboard.Copy(context.Background(), runner, nil); !errors.Is(err, clipboard.ErrUnavailable) {
		t.Errorf("err = %v, want ErrUnavailable", err)
	}
	if clipboard.Available(runner) {
		t.Error("Available should be false with no tool installed")
	}
}

func TestCopySurfacesToolFailure(t *testing.T) {
	runner := command.NewFake()
	runner.Errors = map[string]error{"wl-copy": errors.New("no display")}

	tool, err := clipboard.Copy(context.Background(), runner, []byte("x"))
	if err == nil {
		t.Fatal("a failing clipboard tool should be reported")
	}
	if tool != "wl-copy" {
		t.Errorf("tool = %q, want the tool that failed", tool)
	}
}
