//go:build ignore

// Render both Xbox display modes and its playlist without reading the live
// session, launching audio, or opening native desktop windows.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/pod"
	"github.com/codemodify/media-player-music/internal/skins"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui/style"
)

func main() {
	dir := flag.String("out", "/tmp/music-player-xbox-preview", "screenshot directory")
	scale := flag.Float64("scale", 1, "display scale")
	flag.Parse()
	if err := skins.Register(); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(*dir, 0755); err != nil {
		panic(err)
	}
	os.Setenv(style.ThemeEnv, players.PodSkin)
	a := ui.New(ui.Options{Headless: true, Scale: float32(*scale), DisableLookWatch: true})
	p, err := pod.New(players.NewHost(a), pod.Options{Headless: true, Scale: float32(*scale)})
	if err != nil {
		panic(err)
	}
	defer p.Close()
	p.Pose(97 * time.Second)
	p.ShowPlaylist(true)
	for _, state := range []struct {
		name string
		eq   bool
	}{{"main", false}, {"equalizer", true}} {
		p.ShowEqualiser(state.eq)
		a.PumpOnce()
		path := filepath.Join(*dir, "xbox-"+state.name+".png")
		if err := p.Main.WritePNG(path); err != nil {
			panic(err)
		}
		fmt.Println("wrote", path)
	}
	path := filepath.Join(*dir, "xbox-playlist.png")
	if err := p.List.WritePNG(path); err != nil {
		panic(err)
	}
	fmt.Println("wrote", path)
}
