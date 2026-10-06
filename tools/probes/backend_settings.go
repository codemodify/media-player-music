//go:build ignore

// Headless visual check of the backend settings panel; starts no players.
package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"github.com/codemodify/media-player-music/internal/playback"
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/uitoolkit"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
)

func main() {
	base := "/tmp/music-player-backend-settings"
	if len(os.Args) > 1 {
		base = os.Args[1]
	}
	selected := "mpv"
	if len(os.Args) > 2 {
		selected = os.Args[2]
	}
	state := players.BackendSettingsState{Selected: selected}
	for _, info := range playback.Detect() {
		c := info.Capabilities
		state.Backends = append(state.Backends, players.BackendInfo{ID: info.ID, Name: info.Name, Executable: info.Executable, Available: info.Available, Equalizer: c.Equalizer, Balance: c.Balance, Visualizer: c.Spectrum})
		fmt.Printf("%s installed=%t EQ=%t balance=%t visualizer=%t\n", info.Name, info.Available, c.Equalizer, c.Balance, c.Spectrum)
	}
	for _, scale := range []float32{1, 1.75} {
		for _, size := range []struct {
			name          string
			width, height int
		}{{"preferred", 600, 470}, {"minimum", 560, 440}} {
			a := uitoolkit.New(uitoolkit.Options{Headless: true, Scale: scale, DisableLookWatch: true})
			ap := style.LookAppearance(a.Look())
			ap.Name, ap.FollowDesktop = players.Themed, false
			a.SetLook(style.WithAppearance(a.Look(), ap))
			w, err := a.NewWindow(platform.WindowOptions{Title: "Music Player Settings", Headless: true, Decorations: platform.DecorationsServer, Width: size.width, Height: size.height})
			if err != nil {
				panic(err)
			}
			panel := players.NewBackendSettings(state, func(string) {}, func() {})
			w.SetContent(panel)
			a.PumpOnce()
			path := filepath.Join(base, fmt.Sprintf("%gx", scale), size.name+".png")
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				panic(err)
			}
			f, err := os.Create(path)
			if err != nil {
				panic(err)
			}
			if err := png.Encode(f, w.Capture()); err != nil {
				panic(err)
			}
			if err := f.Close(); err != nil {
				panic(err)
			}
			fmt.Println(path)
			w.Close()
		}
	}
}
