package main

import (
	"runtime"
	"testing"

	"goframework.io/desktop-ui/platform"
)

func TestBackendSupportsSkinSwitchingAndRespectsExplicitChoices(t *testing.T) {
	// Selecting a backend does not connect to the display or open windows.
	// The fixture covers an available XWayland display without touching it.
	t.Setenv("DISPLAY", ":12345")
	t.Setenv(platform.EnvBackend, "")
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
		t.Setenv(platform.EnvBackend, override)
		if got := playerBackend(false); got != "" {
			t.Fatalf("overrode %s=%s with %s", platform.EnvBackend, override, got)
		}
	}
	t.Setenv(platform.EnvBackend, "")
	t.Setenv("DISPLAY", "")
	if got := playerBackend(false); got != "" {
		t.Fatalf("missing X11 display requested backend %q", got)
	}
}
