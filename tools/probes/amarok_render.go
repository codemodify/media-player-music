//go:build ignore

// Amarok headless rendering probe; creates no desktop windows or audio engine.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/amarok"
	"github.com/codemodify/media-player-music/internal/skins"
	"github.com/codemodify/media-player-music/internal/skins/plans"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui/style"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-generate" {
		if err := plans.Write("internal/skins/packs", plans.Sidebar()); err != nil {
			panic(err)
		}
		return
	}
	if err := skins.Register(); err != nil {
		panic(err)
	}
	os.Setenv(style.ThemeEnv, "sidebar")
	dir := "/tmp/music-player-amarok-render"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		panic(err)
	}
	for _, scale := range []float32{1, 1.75} {
		a := ui.New(ui.Options{Headless: true, Scale: scale, DisableLookWatch: true})
		h := players.NewHost(a)
		h.ShowSettings = func() {}
		h.Packs = players.SkinnedIDs
		p, err := amarok.New(h, amarok.Options{Headless: true, Scale: scale})
		if err != nil {
			panic(err)
		}
		p.Pose(44 * time.Second)
		for _, size := range [][2]int{{920, 620}, {740, 480}, {1100, 700}} {
			p.Main.SetSize(size[0], size[1])
			a.PumpOnce()
			p.Advance(0)
			name := fmt.Sprintf("amarok-%gx-%dx%d.png", scale, size[0], size[1])
			if err := p.Main.CaptureSurface().WritePNGFile(filepath.Join(dir, name)); err != nil {
				panic(err)
			}
			fmt.Println(filepath.Join(dir, name))
		}
		p.ShowEqualiser(false)
		a.PumpOnce()
		if err := p.Main.CaptureSurface().WritePNGFile(filepath.Join(dir, fmt.Sprintf("amarok-%gx-no-eq.png", scale))); err != nil {
			panic(err)
		}
		p.Close()
	}
}
