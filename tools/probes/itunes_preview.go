//go:build ignore

// Render the iTunes views without touching the live session, audio or desktop.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/itunes"
	"github.com/codemodify/media-player-music/internal/skins"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui/style"
	"goframework.io/graphics-paint2d"
)

func main() {
	dir := flag.String("out", "/tmp/music-player-itunes-preview", "screenshot directory")
	scale := flag.Float64("scale", 1, "display scale")
	flag.Parse()
	if err := skins.Register(); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(*dir, 0755); err != nil {
		panic(err)
	}
	os.Setenv(style.ThemeEnv, players.ITunesSkin)
	a := ui.New(ui.Options{Headless: true, Scale: float32(*scale), DisableLookWatch: true})
	h := players.NewHost(a)
	h.ShowSettings = func() {}
	p, err := itunes.New(h, itunes.Options{Headless: true, Scale: float32(*scale)})
	if err != nil {
		panic(err)
	}
	defer p.Close()
	p.Pose(97 * time.Second)
	p.ShowPlaylist(true)
	p.ShowEqualiser(true)
	a.PumpOnce()
	write := func(name string, im *paint2d.Image) {
		path := filepath.Join(*dir, "itunes-"+name+".png")
		f, err := os.Create(path)
		if err != nil {
			panic(err)
		}
		if err = im.WritePNG(f); err != nil {
			panic(err)
		}
		if err = f.Close(); err != nil {
			panic(err)
		}
		fmt.Println("wrote", path)
	}
	write("main", p.Main.CaptureSurface())
	write("equalizer", p.Eq.CaptureSurface())
	p.Main.SetSize(940, 650)
	a.PumpOnce()
	p.Advance(0)
	write("main-resized", p.Main.CaptureSurface())
	p.SetMiniPlayer(true)
	a.PumpOnce()
	p.Advance(0)
	write("mini", p.Main.CaptureSurface())
	p.Main.SetSize(150, itunes.MiniH)
	a.PumpOnce()
	p.Advance(0)
	write("mini-compact", p.Main.CaptureSurface())
	p.SetMiniPlayer(false)
	p.Main.SetSize(itunes.MinW, itunes.MinH)
	a.PumpOnce()
	p.Advance(0)
	write("main-minimum", p.Main.CaptureSurface())
}
