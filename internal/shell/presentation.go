package shell

import (
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/aluminium"
	"github.com/codemodify/media-player-music/internal/players/cobalt"
	"github.com/codemodify/media-player-music/internal/players/console"
	"github.com/codemodify/media-player-music/internal/players/fancy"
	"github.com/codemodify/media-player-music/internal/players/fandom"
	"github.com/codemodify/media-player-music/internal/players/gauge"
	"github.com/codemodify/media-player-music/internal/players/panel"
	"github.com/codemodify/media-player-music/internal/players/pod"
	"github.com/codemodify/media-player-music/internal/players/sidebar"
	"github.com/codemodify/media-player-music/internal/players/standard"
	"github.com/codemodify/media-player-music/internal/players/utility"
	"github.com/codemodify/media-player-music/internal/players/wingspan"
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

// RackedPresentation is a face whose panels are a rack.Group: closing its
// player is the group's first member's close (rack.Member.OnClose), which
// holds whichever window the player is in, the window it opened with or the
// one it shares with its panels after the group falls back to sharing.
type RackedPresentation interface {
	SetOnClose(func() bool)
}

func openPresentation(h *players.Host, opts Options, skin string) (Presentation, error) {
	if skin == players.DefaultSkin {
		return standard.New(h, standard.Options{Headless: opts.Headless, Scale: opts.Scale})
	}
	if skin == players.WingspanSkin {
		return wingspan.New(h, wingspan.Options{Headless: opts.Headless, Scale: opts.Scale})
	}
	if skin == players.PodSkin {
		return pod.New(h, pod.Options{Headless: opts.Headless, Scale: opts.Scale})
	}
	if skin == players.GaugeSkin {
		return gauge.New(h, gauge.Options{Headless: opts.Headless, Scale: opts.Scale})
	}
	if skin == players.FandomSkin {
		return fandom.New(h, fandom.Options{Headless: opts.Headless, Scale: opts.Scale})
	}
	if skin == players.AluminiumSkin {
		return aluminium.New(h, aluminium.Options{Headless: opts.Headless, Scale: opts.Scale})
	}
	if skin == players.CobaltSkin {
		return cobalt.New(h, cobalt.Options{Headless: opts.Headless, Scale: opts.Scale})
	}
	if skin == players.UtilitySkin {
		return utility.New(h, utility.Options{Headless: opts.Headless, Scale: opts.Scale})
	}
	if skin == players.ConsoleSkin {
		return console.New(h, console.Options{Headless: opts.Headless, Scale: opts.Scale})
	}
	if skin == players.SidebarSkin {
		return sidebar.New(h, sidebar.Options{Headless: opts.Headless, Scale: opts.Scale})
	}
	if skin == players.FancySkin {
		return fancy.New(h, fancy.Options{Headless: opts.Headless, Scale: opts.Scale})
	}
	return panel.New(h, panel.Options{Headless: opts.Headless, Scale: opts.Scale})
}
