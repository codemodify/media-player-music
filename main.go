// Command media-player-music is a music player's *interface*, with three
// faces over one player.
//
// It is a **visual demo**. Nothing is decoded and nothing is played: the
// clock counts, the seek bar scrubs the clock, the analyser is drawn from
// the clock, and the equaliser's faders move and filter nothing. There is no
// audio stack anywhere in it and there is not going to be one — what it
// demonstrates is the interface, the skins and the windows.
//
// The three faces are three ways of showing the same player, and switching
// between them does not restart it: the queue, the position, the volume and
// the equaliser's curve are the application's and survive the change.
//
//	media-player-music                      # the face last used, in the skin last used
//	media-player-music -face minim          # the compact strip
//	media-player-music -face marquee        # the cabinet that folds
//	media-player-music -face lantern        # the window cut to its outline
//	media-player-music -skin minim-classic  # in a skin of that face's
//	media-player-music -skin breeze-night   # with the skin dropped, in an ordinary theme
//	media-player-music -only strip          # the compact face's strip on its own
//	media-player-music -compact             # the cabinet, folded down
//	media-player-music -scale 1.75          # at a fractional scale
//	media-player-music -shot out/           # paint one frame of each window, write, exit
//
// Ctrl+F steps to the next face and Ctrl+Shift+F back; Ctrl+K steps through
// the skins the face was drawn for and Ctrl+Shift+K drops the skin for an
// ordinary theme. Space plays and pauses, Z and B change track, V stops, the
// arrows seek and set the volume, M mutes, S shuffles and R repeats. Escape
// quits.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/codemodify/media-player-music/internal/shell"
	"github.com/codemodify/uitoolkit"
	"github.com/codemodify/uitoolkit/icons"
	"github.com/codemodify/uitoolkit/style"
)

func main() {
	face := flag.String("face", "", "the face to wear: minim, marquee or lantern (default: the last one used)")
	skin := flag.String("skin", "", "the pack to wear: one of the face's skins, or any other theme (default: the last one used)")
	only := flag.String("only", "", "strip | strip+eq | strip+list — which of a face's windows to open")
	compact := flag.Bool("compact", false, "open the marquee face folded down")
	scale := flag.Float64("scale", 0, "display scale (0: the desktop's, or $UITK_SCALE)")
	shot := flag.String("shot", "", "paint one frame of each window into this directory and exit")
	at := flag.Duration("at", 97*time.Second, "where the head is in -shot mode")
	flag.Parse()
	log.SetFlags(0)
	log.SetPrefix("media-player-music: ")

	headless := *shot != ""
	opts := shell.Options{
		Face:     *face,
		Skin:     *skin,
		Compact:  *compact,
		Scale:    float32(*scale),
		Headless: headless,
		// A shot is posed rather than used, so it is never remembered.
		Remember: !headless,
	}
	switch *only {
	case "strip":
		opts.NoEq, opts.NoList = true, true
	case "strip+eq":
		opts.NoList = true
	case "strip+list":
		opts.NoEq = true
	}

	// The look is built with the application, so the pack the face wants
	// has to be chosen first. It is the same override UITK_THEME=<pack>
	// gives any toolkit app.
	id := shell.FaceFor(opts)
	pack := shell.SkinFor(id, opts)
	os.Setenv(style.ThemeEnv, pack)

	a := uitoolkit.New(uitoolkit.Options{Headless: headless, Scale: float32(*scale)})
	// The window icon the desktop shows in its title bar, task bar and
	// switcher.
	a.SetIcon(icons.AppIconRGB("list", 0x3a, 0x9a, 0x4a)...)

	s, err := shell.New(a, opts)
	if err != nil {
		log.Fatal(err)
	}

	if headless {
		if err := os.MkdirAll(*shot, 0o755); err != nil {
			log.Fatal(err)
		}
		// Posed rather than run: the same position gives the same picture
		// every time, which is what makes one shot comparable with the one
		// before it.
		s.Pose(*at)
		a.PumpOnce()
		for _, w := range s.Windows() {
			if w.Window == nil {
				continue
			}
			path := filepath.Join(*shot, s.FaceID()+"-"+w.Name+".png")
			if err := w.Window.WritePNG(path); err != nil {
				log.Fatal(err)
			}
			fmt.Println("wrote", path)
		}
		return
	}

	s.Start()
	a.Post(func() {
		log.Printf("backend=%s %s scale=%.2f", a.BackendName(), s.Status(), s.Face().MainWindow().Scale())
	})
	if err := a.Run(); err != nil {
		log.Fatal(err)
	}
}
