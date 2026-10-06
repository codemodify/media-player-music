//go:build ignore

// Render the original Half Life 2 three-window composition and both closed
// shutter modes without using the live session, audio, or desktop windows.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/halflife2"
	"github.com/codemodify/media-player-music/internal/skins"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
)

func main() {
	dir := flag.String("out", "/tmp/music-player-halflife2-preview", "screenshot directory")
	scale := flag.Float64("scale", 1, "display scale")
	flag.Parse()
	if err := skins.Register(); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(*dir, 0755); err != nil {
		panic(err)
	}
	os.Setenv(style.ThemeEnv, players.HalfLife2Skin)
	a := uitoolkit.New(uitoolkit.Options{Headless: true, Scale: float32(*scale), DisableLookWatch: true})
	p, err := halflife2.New(players.NewHost(a), halflife2.Options{Headless: true, Scale: float32(*scale)})
	if err != nil {
		panic(err)
	}
	defer p.Close()
	p.Pose(97 * time.Second)
	p.ShowEqualiser(true)
	p.ShowPlaylist(true)
	a.PumpOnce()
	write := func(name string, im *paintengine2d.Image) {
		path := filepath.Join(*dir, "half-life-2-"+name+".png")
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
	for _, window := range p.Windows() {
		write(window.Name, window.Window.CaptureSurface())
	}
	// Match the original composition: equalizer above the main window's
	// right-hand side, playlist immediately below the equalizer.
	d := float32(*scale)
	composition := paintengine2d.NewImage(int(769*d+.5), int(465*d+.5))
	ctx := paintengine2d.NewContext(composition)
	ctx.Clear(paintengine2d.Transparent)
	for _, panel := range []struct {
		im         *paintengine2d.Image
		x, y, w, h float32
	}{{p.Main.CaptureSurface(), 0, 80, 339, 300}, {p.Eq.CaptureSurface(), 339, 0, 430, 239}, {p.List.CaptureSurface(), 339, 239, 393, 226}} {
		ctx.DrawImageRectPaint(panel.im, paintengine2d.XYWH(0, 0, float32(panel.im.Bounds().Dx()), float32(panel.im.Bounds().Dy())), paintengine2d.XYWH(panel.x*d, panel.y*d, panel.w*d, panel.h*d), paintengine2d.Paint{Color: paintengine2d.White, Filter: paintengine2d.FilterNearest})
	}
	write("group", composition)
	for _, state := range []string{"inner-shutter", "outer-shutter"} {
		var id uint64
		p.Main.AccessibleTree().Walk(func(n *a11y.Node) bool {
			if n.Name == "Close inner shutter" || n.Name == "Close outer shutter" {
				id = n.ID
			}
			return true
		})
		if !p.Main.AccessibleAction(id, a11y.ActionDefault) {
			panic("source shutter control is inaccessible")
		}
		a.PumpOnce()
		write(state, p.Main.CaptureSurface())
	}
	// Restore open mode through keyboard activation of the same focused
	// shutter control, as a user would after closing the display.
	p.Main.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeySpace})
	p.Main.Inject(platform.Event{Kind: platform.EventKeyUp, Key: platform.KeySpace})
	a.PumpOnce()
	var visualID uint64
	p.Main.AccessibleTree().Walk(func(n *a11y.Node) bool {
		if n.Name == "Show or hide spectrum" {
			visualID = n.ID
		}
		return true
	})
	if !p.Main.AccessibleAction(visualID, a11y.ActionDefault) {
		panic("spectrum control is inaccessible")
	}
	a.PumpOnce()
	write("spectrum", p.Main.CaptureSurface())
	p.List.SetSize(610, 430)
	a.PumpOnce()
	p.Advance(0)
	write("playlist-resized", p.List.CaptureSurface())
}
