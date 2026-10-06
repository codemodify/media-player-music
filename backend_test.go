package main

import (
	"runtime"
	"testing"

	"github.com/codemodify/uitoolkit/platform"
)

func TestBackendSupportsSkinSwitchingAndRespectsExplicitChoices(t *testing.T) {
	// Selecting a backend does not connect to the display or open windows.
	// The fixture covers an available XWayland display without touching it.
	t.Setenv("DISPLAY", ":12345")
	t.Setenv("UITK_BACKEND", "")
	want := ""
	if runtime.GOOS == "linux" && platform.Select("x11", false).Name() == "x11" {
		want = "x11"
	}
	if got := playerBackend(false); got != want {
		t.Fatalf("automatic player backend = %q, want %q", got, want)
	}
	if got := playerBackend(true); got != "" {
		t.Fatalf("headless run requested desktop backend %q", got)
	}
	for _, override := range []string{"wayland", "x11", "offscreen", "auto"} {
		t.Setenv("UITK_BACKEND", override)
		if got := playerBackend(false); got != "" {
			t.Fatalf("overrode UITK_BACKEND=%s with %s", override, got)
		}
	}
	t.Setenv("UITK_BACKEND", "")
	t.Setenv("DISPLAY", "")
	if got := playerBackend(false); got != "" {
		t.Fatalf("missing X11 display requested backend %q", got)
	}
}
