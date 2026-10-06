package main

import (
	"os"
	"runtime"
	"strings"

	"github.com/codemodify/uitoolkit/platform"
)

// playerBackend prefers X11 on Linux so the skin windows can snap and
// travel together, including when switching from the default UI. Ordinary
// Wayland toplevels cannot report positions or place their neighbours.
// Explicit toolkit backend choices and headless rendering take precedence.
func playerBackend(headless bool) string {
	if headless || runtime.GOOS != "linux" || strings.TrimSpace(os.Getenv("UITK_BACKEND")) != "" {
		return ""
	}
	if platform.Select("x11", false).Name() == "x11" {
		return "x11"
	}
	return ""
}
