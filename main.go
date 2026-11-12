// Command media-player-music is media-player-music's desktop UI: the player
// media-player-musicd owns, in an ordinary default UI or in its optional
// skins, the same one the terminal and web UIs show. The daemon plays; this
// program shows it and sends it what the person does.
//
//	media-player-music                                 the daemon on this machine
//	media-player-music -daemon ws://host:50130/clients
//	media-player-music song.mp3 ~/Music/album          the queue in their place
//	media-player-music -shot DIR                       posed screenshots, nothing played, no daemon
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/codemodify/media-player-music/internal/appicon"
	"github.com/codemodify/media-player-music/internal/link"
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/shell"
	"github.com/codemodify/media-player-music/internal/skins"
	"github.com/codemodify/media-player-music/musicapi"
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
	version := flag.Bool("version", false, "print the version and exit")
	daemon := flag.String("daemon", "", "media-player-musicd's address: unix://… (its socket in the user's runtime folder by default) or ws://…")
	token := flag.String("token", "", "the token media-player-musicd admits a ws:// renderer by; by default the one in media-player-musicd.token in the user's runtime folder")
	flag.Parse()
	if *version {
		fmt.Println("media-player-music", players.Version)
		return nil
	}
	log.SetFlags(0)
	log.SetPrefix("media-player-music: ")

	if err := skins.Register(); err != nil {
		return err
	}
	headless := *shot != ""
	opts := shell.Options{Skin: *skin, Scale: float32(*scale), Headless: headless, Remember: !headless}
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
	a := ui.New(ui.Options{Headless: headless, Scale: float32(*scale)})
	a.SetIcon(iconImages...)
	// the daemon's states reach the shell on the UI's thread; those before
	// it is made wait in the posted queue until Run
	var s *shell.Shell
	ctx, quit := context.WithCancel(context.Background())
	defer quit()
	files, first := flag.Args(), true
	if !headless {
		address := *daemon
		if address == "" {
			if address, err = link.DefaultAddress(); err != nil {
				return err
			}
		}
		given, err := link.Token(address, *token)
		if err != nil {
			return err
		}
		post := func(show func(*shell.Shell)) {
			a.Post(func() {
				if s != nil {
					show(s)
				}
			})
		}
		opts.Player = link.Start(ctx, link.Options{
			Address: address, Name: "desktop", Version: players.Version, Token: given,
			State:  func(state musicapi.State) { post(func(s *shell.Shell) { s.ShowState(state) }) },
			Queue:  func(queue musicapi.Queue) { post(func(s *shell.Shell) { s.ShowQueue(queue) }) },
			Now:    func(now musicapi.Now) { post(func(s *shell.Shell) { s.ShowNow(now) }) },
			Status: func(text string, problem bool) { post(func(s *shell.Shell) { s.ShowLink(text, problem) }) },
			Failed: func(err error) { post(func(s *shell.Shell) { s.ShowLink(err.Error(), true) }) },
			// media-player-musicd started when it does not answer at the
			// default address (goframework R115): the faces say "Starting
			// media-player-musicd…", or why it could not be, with Retry
			Ensure: *daemon == "",
			Daemon: func(d link.Daemon) { post(func(s *shell.Shell) { s.ShowDaemon(d.Text(), d.Problem(), d.Retry) }) },
			// the files named on the command line, once the daemon answers
			Connected: func(connected bool) {
				post(func(s *shell.Shell) {
					if connected && first {
						first = false
						if err := s.LoadFiles(files, false); err != nil {
							log.Print(err)
						}
					}
				})
			},
		})
	}
	s, err = shell.New(a, opts)
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
