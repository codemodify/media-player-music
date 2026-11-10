// Command media-player-music presents one player in an ordinary default UI
// or in its optional Winamp and Windows Media Player skins. Audio uses installed executables.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/codemodify/media-player-music/internal/appicon"
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/shell"
	"github.com/codemodify/media-player-music/internal/skins"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	skin := flag.String("skin", "", "appearance: "+players.SkinChoices()+" (default: last used, or no skin)")
	scale := flag.Float64("scale", 0, "display scale (0: desktop scale or $"+platform.EnvScale+")")
	shot := flag.String("shot", "", "paint posed screenshots into this directory and exit")
	at := flag.Duration("at", 97*time.Second, "playback position in screenshot mode")
	audioBackend := flag.String("audio-backend", "", "installed playback program: mpv, vlc, mplayer, mpg123 (default: last used, or mpv)")
	flag.Parse()
	log.SetFlags(0)
	log.SetPrefix("media-player-music: ")

	if err := skins.Register(); err != nil {
		return err
	}
	headless := *shot != ""
	opts := shell.Options{Skin: *skin, Scale: float32(*scale), Headless: headless, Remember: !headless, AudioBackend: *audioBackend}
	selected, ok := players.NormalizeSkin(shell.SkinFor(opts))
	if !ok {
		return fmt.Errorf("unknown skin %q; choose %s", *skin, players.SkinChoices())
	}
	opts.Skin = selected
	os.Setenv(style.ThemeEnv, shell.LookFor(selected))
	// Preserve the player's desktop reduced-motion behavior after the toolkit
	// made following that preference an application opt-in.
	style.SetFollowDesktopMotion(true)
	// Song and artist labels borrow missing glyphs (CJK and other scripts)
	// from installed fonts; this was the font_fallback build tag.
	style.SetFallback(true)
	iconImages, err := appicon.Images(appicon.LogoPNG())
	if err != nil {
		return err
	}
	a := ui.New(ui.Options{Headless: headless, Scale: float32(*scale), Backend: playerBackend(headless)})
	a.SetIcon(iconImages...)
	s, err := shell.New(a, opts)
	if err != nil {
		return err
	}
	defer s.Close()
	if headless {
		// Window following runs even without playback. Screenshots stay
		// posed, so stop that UI timer before setting the requested time.
		s.Pulse().Stop()
		if err := os.MkdirAll(*shot, 0o755); err != nil {
			return err
		}
		s.Pose(*at)
		a.PumpOnce()
		for _, w := range s.Windows() {
			path := filepath.Join(*shot, selected+"-"+w.Name+".png")
			if err := w.Window.WritePNG(path); err != nil {
				return err
			}
			fmt.Println("wrote", path)
		}
		return nil
	}
	if err := s.LoadFiles(flag.Args(), false); err != nil {
		return err
	}
	s.Start()
	// Let a desktop launcher or terminal stop the player through the same
	// cleanup path as its close button, including the saved session.
	stop := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)
	defer close(done)
	go func() {
		select {
		case <-stop:
			a.Quit()
		case <-done:
		}
	}()
	a.Post(func() {
		log.Printf("backend=%s %s scale=%.2f", a.BackendName(), s.Status(), s.MainWindow().Scale())
	})
	return a.Run()
}
