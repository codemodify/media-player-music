//go:build ignore

// Render all original Series 9 arrangements without touching desktop/audio.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/series9"
	"github.com/codemodify/media-player-music/internal/skins"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui/style"
)

func main() {
	dir := flag.String("out", "/tmp/music-player-series9-preview", "screenshot directory")
	scale := flag.Float64("scale", 1, "display scale")
	flag.Parse()
	if err := skins.Register(); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(*dir, 0755); err != nil {
		panic(err)
	}
	os.Setenv(style.ThemeEnv, players.Series9Skin)
	a := ui.New(ui.Options{Headless: true, Scale: float32(*scale), DisableLookWatch: true})
	h := players.NewHost(a)
	h.ShowSettings = func() {}
	p, err := series9.New(h, series9.Options{Headless: true, Scale: float32(*scale)})
	if err != nil {
		panic(err)
	}
	defer p.Close()
	a.PumpOnce()
	p.Pose(97 * time.Second)
	pump := func() { a.PumpOnce(); p.Advance(0); a.PumpOnce() }
	write := func(name string) {
		path := filepath.Join(*dir, "series-9-"+name+".png")
		im := p.Main.CaptureSurface()
		if im == nil {
			panic("missing capture")
		}
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
	p.ShowEqualiser(false)
	p.ShowPlaylist(false)
	pump()
	write("main")
	p.ShowEqualiser(true)
	p.Equaliser.SetOn(true)
	pump()
	write("equalizer")
	p.ShowEqualiser(false)
	p.ShowPlaylist(true)
	pump()
	write("playlist")
	p.ShowEqualiser(true)
	pump()
	write("group")
	p.Main.SetSize(740, 574)
	pump()
	write("resized")
	p.SetCompact(true)
	pump()
	write("compact")
	p.SetCompact(false)
	pump()
	write("restored")
}
