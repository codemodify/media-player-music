package main

import (
	"os"
	"runtime"
	"strings"

	"goframework.io/desktop-ui/platform"
)

// playerBackend prefers X11 on Linux so the skin windows can snap and
// travel together, including when switching from the default UI. Ordinary
// Wayland toplevels cannot report positions or place their neighbours.
// Explicit toolkit backend choices and headless rendering take precedence.
//
// R21 (G5): kept until the four racked faces (winamp, fancy, fandom,
// halflife2) move from rack.Desk to rack.Group, which realises a group on
// Wayland as one window per part. Group cannot take them as they are yet:
// it opens every member undecorated and shares a window among a part's
// members, so Winamp's panels lose the skin frame each one gets from
// Window.SetFrameRole (strip, equaliser, playlist); and its settle puts
// back only the declared bonds, so a saved, undocked panel lands where the
// window manager put it rather than where the session left it.
func playerBackend(headless bool) string {
	if headless || runtime.GOOS != "linux" || strings.TrimSpace(os.Getenv(platform.EnvBackend)) != "" {
		return ""
	}
	if platform.Select("x11", false).Name() == "x11" {
		return "x11"
	}
	return ""
}
