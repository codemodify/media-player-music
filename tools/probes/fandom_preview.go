//go:build ignore

// Render Fandom without touching the live session, audio or desktop windows.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/fandom"
	"github.com/codemodify/media-player-music/internal/skins"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit"
	"github.com/codemodify/uitoolkit/style"
)

func main() {
	dir := flag.String("out", "/tmp/music-player-fandom-preview", "screenshot directory")
	scale := flag.Float64("scale", 1, "display scale")
	flag.Parse()
	if err := skins.Register(); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(*dir, 0755); err != nil {
		panic(err)
	}
	os.Setenv(style.ThemeEnv, players.FandomSkin)
	a := uitoolkit.New(uitoolkit.Options{Headless: true, Scale: float32(*scale), DisableLookWatch: true})
	p, err := fandom.New(players.NewHost(a), fandom.Options{Headless: true, Scale: float32(*scale)})
	if err != nil {
		panic(err)
	}
	defer p.Close()
	p.Pose(97 * time.Second)
	p.ShowEqualiser(false)
	p.ShowPlaylist(true)
	a.PumpOnce()
	write := func(name string, im *paintengine2d.Image) {
		path := filepath.Join(*dir, "fandom-"+name+".png")
		file, err := os.Create(path)
		if err != nil {
			panic(err)
		}
		if err := im.WritePNG(file); err != nil {
			file.Close()
			panic(err)
		}
		if err := file.Close(); err != nil {
			panic(err)
		}
		fmt.Println("wrote", path)
	}
	write("main", p.Main.CaptureSurface())
	write("playlist", p.List.CaptureSurface())
	d := float32(*scale)
	im := paintengine2d.NewImage(int(680*d+.5), int(330*d+.5))
	ctx := paintengine2d.NewContext(im)
	ctx.Clear(paintengine2d.Transparent)
	for _, v := range []struct {
		im         *paintengine2d.Image
		x, y, w, h float32
	}{{p.Main.CaptureSurface(), 0, 0, 490, 330}, {p.List.CaptureSurface(), 490, 46, 190, 246}} {
		ctx.DrawImageRectPaint(v.im, paintengine2d.XYWH(0, 0, float32(v.im.Width), float32(v.im.Height)), paintengine2d.XYWH(v.x*d, v.y*d, v.w*d, v.h*d), paintengine2d.Paint{Color: paintengine2d.White, Filter: paintengine2d.FilterNearest})
	}
	write("group", im)
	p.ShowEqualiser(true)
	a.PumpOnce()
	write("equalizer", p.Main.CaptureSurface())
	p.ShowSettingsPanel(false)
	a.PumpOnce()
	write("settings-hidden", p.Main.CaptureSurface())
	p.List.SetSize(420, 360)
	a.PumpOnce()
	p.Advance(0)
	write("playlist-resized", p.List.CaptureSurface())
}
