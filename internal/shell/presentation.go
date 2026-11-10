package shell

import (
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/amarok"
	"github.com/codemodify/media-player-music/internal/players/fancy"
	"github.com/codemodify/media-player-music/internal/players/fandom"
	"github.com/codemodify/media-player-music/internal/players/halflife2"
	"github.com/codemodify/media-player-music/internal/players/headspace"
	"github.com/codemodify/media-player-music/internal/players/itunes"
	"github.com/codemodify/media-player-music/internal/players/realplayer"
	"github.com/codemodify/media-player-music/internal/players/series9"
	"github.com/codemodify/media-player-music/internal/players/standard"
	"github.com/codemodify/media-player-music/internal/players/vlc"
	"github.com/codemodify/media-player-music/internal/players/winamp"
	"github.com/codemodify/media-player-music/internal/players/xbox"
	"goframework.io/desktop-ui"
)

// Presentation is the window arrangement for the player's appearance.
// Default and Headspace use one window; Xbox and Fandom have playlist satellites,
// Winamp and Half Life 2 use three panel windows in separate families.
// iTunes switches full and mini views within its main window and owns an EQ.
// Series 9 embeds its drawers and switches full/compact views in one window.
// Classic VLC, RealPlayer and Amarok each embed their sections in one window.
// Fancy uses three shaped panels and switches its main window to a compact view.
type Presentation interface {
	Close()
	MainWindow() *ui.Window
	Windows() []players.NamedWindow
	Advance(time.Duration)
	Refresh()
	Pose(time.Duration)
	ShowEqualiser(bool)
	ShowPlaylist(bool)
	CapturePlacement() map[string]players.WindowPlacement
	RestorePlacement(map[string]players.WindowPlacement)
}

// StatefulPresentation optionally remembers view state in its own family.
// Restoring it precedes placement so the current mode sets its size limits.
type StatefulPresentation interface {
	CaptureState() players.PresentationState
	RestoreState(players.PresentationState)
}

func openPresentation(h *players.Host, opts Options, skin string) (Presentation, error) {
	if skin == players.DefaultSkin {
		return standard.New(h, standard.Options{Headless: opts.Headless, Scale: opts.Scale})
	}
	if skin == players.HeadspaceSkin {
		return headspace.New(h, headspace.Options{Headless: opts.Headless, Scale: opts.Scale})
	}
	if skin == players.XboxSkin {
		return xbox.New(h, xbox.Options{Headless: opts.Headless, Scale: opts.Scale})
	}
	if skin == players.HalfLife2Skin {
		return halflife2.New(h, halflife2.Options{Headless: opts.Headless, Scale: opts.Scale})
	}
	if skin == players.FandomSkin {
		return fandom.New(h, fandom.Options{Headless: opts.Headless, Scale: opts.Scale})
	}
	if skin == players.ITunesSkin {
		return itunes.New(h, itunes.Options{Headless: opts.Headless, Scale: opts.Scale})
	}
	if skin == players.Series9Skin {
		return series9.New(h, series9.Options{Headless: opts.Headless, Scale: opts.Scale})
	}
	if skin == players.VLCSkin {
		return vlc.New(h, vlc.Options{Headless: opts.Headless, Scale: opts.Scale})
	}
	if skin == players.RealPlayerSkin {
		return realplayer.New(h, realplayer.Options{Headless: opts.Headless, Scale: opts.Scale})
	}
	if skin == players.AmarokSkin {
		return amarok.New(h, amarok.Options{Headless: opts.Headless, Scale: opts.Scale})
	}
	if skin == players.FancySkin {
		return fancy.New(h, fancy.Options{Headless: opts.Headless, Scale: opts.Scale})
	}
	return winamp.New(h, winamp.Options{Headless: opts.Headless, Scale: opts.Scale})
}
