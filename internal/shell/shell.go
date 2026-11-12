// Package shell coordinates the player model, appearance and one clock.
package shell

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	amarokskin "github.com/codemodify/media-player-music/internal/skins/amarok"
	fancyskin "github.com/codemodify/media-player-music/internal/skins/fancy"
	fandomskin "github.com/codemodify/media-player-music/internal/skins/fandom"
	halflife2skin "github.com/codemodify/media-player-music/internal/skins/halflife2"
	headspaceskin "github.com/codemodify/media-player-music/internal/skins/headspace"
	itunesskin "github.com/codemodify/media-player-music/internal/skins/itunes"
	"github.com/codemodify/media-player-music/internal/skins/panel"
	realplayerskin "github.com/codemodify/media-player-music/internal/skins/realplayer"
	series9skin "github.com/codemodify/media-player-music/internal/skins/series9"
	vlcskin "github.com/codemodify/media-player-music/internal/skins/vlc"
	xboxskin "github.com/codemodify/media-player-music/internal/skins/xbox"
	"github.com/codemodify/media-player-music/musicapi"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
)

// Appearances are the complete set of UI choices. An ordinary themed UI is
// the default; optional skins supply their artwork and panel layouts.
var Appearances = players.AppearanceIDs()

type Options struct {
	Skin     string
	Remember bool
	Headless bool
	Scale    float32
	// Player is media-player-musicd's player, which the shell shows and
	// drives: the link's (internal/link) in the program, a service in the
	// tests' own process. Nil, the shell is headless screenshots' fixture,
	// on a simulated clock. Its states come through Show.
	Player musicapi.Player
}

type Shell struct {
	App  *ui.Application
	Host *players.Host

	opts           Options
	settings       players.Settings
	presentation   Presentation
	skin           string
	pulse          *players.Pulse
	switching      bool
	player         musicapi.Player
	daemon         synced
	applying       bool           // the daemon's state going into the model
	workers        sync.WaitGroup // cover reads, file loads
	settingsWindow *ui.Window
	settingsPanel  *players.SettingsPanel
	note           daemonNote // starting the daemon (daemon.go)
	library        musicapi.Library
	libraryView    *libraryView // the library window (library.go)
	closed         bool
	fileWindow     *ui.Window
	fileLoader     *fileLoader
	rememberIn     time.Duration
	lastSaved      []byte
	tray           platform.StatusItem
	trayHidden     map[string]bool
}

func SkinFor(opts Options) string {
	if opts.Skin != "" {
		return opts.Skin
	}
	if opts.Remember {
		return players.LoadSettings().Skin
	}
	return players.DefaultSkin
}

// LookFor maps the default UI to an ordinary toolkit theme.
func LookFor(skin string) string {
	if id, ok := players.NormalizeSkin(skin); ok {
		if id == players.DefaultSkin {
			return players.Themed
		}
		return id
	}
	return skin
}

func New(a *ui.Application, opts Options) (*Shell, error) {
	s := &Shell{App: a, opts: opts, settings: players.LoadSettings()}
	s.Host = &players.Host{
		App: a, Model: players.NewModel(),
		Run: s.Command, Shared: s.Keys,
		Packs: players.SkinnedIDs,
		ChooseSkin: func(id string) {
			a.Post(func() {
				if err := s.SetSkin(id); err != nil {
					log.Printf("change skin: %v", err)
				}
			})
		},
	}
	s.Host.LoadFiles, s.Host.DeleteTrack, s.Host.SyncPlayback = s.openFiles, s.removeTrack, s.syncPlayback
	s.Host.DropFiles = s.dropFiles
	s.Host.ClearTracks, s.Host.SavePlaylistTo = s.clearQueue, s.savePlaylist
	s.Host.ShowSettings = s.openSettings
	if opts.Player != nil {
		s.Host.ShowLibrary = s.openLibrary
	}
	if opts.Player != nil {
		s.Host.Model.Live = true
		s.Host.Model.Spectrum.Live = true
		s.Host.Model.Transport.List = players.NewPlaylist()
		s.Host.Model.Transport.State = players.Stopped
		s.player = opts.Player
		s.daemon.index = -1
		if opts.Remember {
			s.restoreSession()
		}
	}
	s.pulse = players.NewPulse(70*time.Millisecond, s.tick)
	if err := s.SetSkin(SkinFor(opts)); err != nil {
		return nil, err
	}
	if s.player != nil {
		// the daemon's state now, when it answers; else as it comes
		ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
		state, err := s.player.State(ctx)
		cancel()
		if err == nil {
			s.ShowState(state)
		}
	}
	s.openTray()
	return s, nil
}

func (s *Shell) Skin() string                   { return s.skin }
func (s *Shell) Presentation() Presentation     { return s.presentation }
func (s *Shell) MainWindow() *ui.Window         { return s.presentation.MainWindow() }
func (s *Shell) Windows() []players.NamedWindow { return s.presentation.Windows() }
func (s *Shell) Pulse() *players.Pulse          { return s.pulse }
func (s *Shell) Pose(pos time.Duration)         { s.presentation.Pose(pos) }

func (s *Shell) Start() {
	if !s.Host.Model.Live {
		s.Host.Model.Transport.Play()
	}
	s.pulse.Start(s.MainWindow())
}

func (s *Shell) Close() {
	if s.closed {
		return
	}
	s.remember()
	s.closed = true
	s.pulse.Stop()
	s.closeTray()
	s.cancelFileLoads()
	s.workers.Wait()
	if s.fileWindow != nil {
		s.fileWindow.Close()
		s.fileWindow = nil
	}
	if s.settingsWindow != nil {
		s.settingsWindow.Close()
		s.settingsWindow = nil
	}
	s.closeDaemonNote()
	s.closeLibrary()
	if s.presentation != nil {
		s.presentation.Close()
	}
}

// SetSkin retains controls within a presentation family. Switching families
// opens the new windows before closing the old ones, keeping the shared model
// and playback process alive throughout the change.
func (s *Shell) SetSkin(id string) error {
	skin, ok := players.NormalizeSkin(id)
	if !ok {
		return fmt.Errorf("unknown skin %q; choose %s", id, players.SkinChoices())
	}
	if s.switching || skin == s.skin {
		return nil
	}
	if skin != players.DefaultSkin {
		pack, ok := skins.LoadSkin(skin)
		if !ok {
			return fmt.Errorf("skin %q is unavailable", skin)
		}
		layouts := []string{headspaceskin.LayoutMain}
		if players.IsPanelSkin(skin) {
			layouts = []string{panel.LayoutStrip, panel.LayoutEqualiser, panel.LayoutPlaylist}
		} else if skin == players.PodSkin {
			layouts = []string{xboxskin.LayoutMain, xboxskin.LayoutPlaylist}
		} else if skin == players.GaugeSkin {
			layouts = []string{halflife2skin.LayoutMain, halflife2skin.LayoutEqualizer, halflife2skin.LayoutPlaylist}
		} else if skin == players.FandomSkin {
			layouts = []string{fandomskin.LayoutMain, fandomskin.LayoutPlaylist}
		} else if skin == players.AluminiumSkin {
			layouts = []string{itunesskin.LayoutMain, itunesskin.LayoutMini, itunesskin.LayoutEqualizer}
		} else if skin == players.CobaltSkin {
			layouts = []string{series9skin.LayoutMain, series9skin.LayoutTransport, series9skin.LayoutEqualizer, series9skin.LayoutPlaylist, series9skin.LayoutCompact}
		} else if skin == players.UtilitySkin {
			layouts = []string{vlcskin.LayoutMain}
		} else if skin == players.ConsoleSkin {
			layouts = []string{realplayerskin.LayoutMain, realplayerskin.LayoutPlaylist, realplayerskin.LayoutEqualizer}
		} else if skin == players.SidebarSkin {
			layouts = []string{amarokskin.LayoutMain}
		} else if skin == players.FancySkin {
			layouts = []string{fancyskin.LayoutMain, fancyskin.LayoutCompact, fancyskin.LayoutEqualizer, fancyskin.LayoutPlaylist}
		}
		for _, name := range layouts {
			if pack.Layouts[name] == nil {
				return fmt.Errorf("skin %q is missing layout %q", skin, name)
			}
		}
	}
	s.switching = true
	defer func() {
		s.switching = false
		if s.trayHidden != nil {
			s.hidePlayerWindows()
		}
	}()

	old, oldSkin := s.presentation, s.skin
	if old != nil && s.Host.Model.Live {
		s.capturePlacement(old, oldSkin)
	}
	if old != nil && presentationFamily(oldSkin) == presentationFamily(skin) {
		s.Host.ApplySkin(skin)
		s.skin = skin
		s.Host.RememberSkin(skin)
		s.refresh()
		s.remember()
		return nil
	}

	s.pulse.Stop()
	before := map[*ui.Window]bool{}
	for _, w := range s.App.Windows() {
		before[w] = true
	}
	s.Host.ApplySkin(LookFor(skin))
	next, err := openPresentation(s.Host, s.opts, skin)
	if err != nil {
		for _, w := range s.App.Windows() {
			if !before[w] {
				w.Close()
			}
		}
		if old != nil {
			s.Host.ApplySkin(LookFor(oldSkin))
			s.restart()
		}
		return fmt.Errorf("open %s: %w", skin, err)
	}
	s.presentation, s.skin = next, skin
	if s.Host.Model.Live {
		family := presentationFamily(skin)
		if stateful, ok := next.(StatefulPresentation); ok {
			stateful.RestoreState(copyPresentationState(s.settings.Presentations[family]))
		}
		next.RestorePlacement(s.settings.Windows[family])
	}
	// Programmatic Close during a skin change bypasses this user-close
	// handler. A shown tray keeps the player recoverable after hiding.
	if racked, ok := next.(RackedPresentation); ok {
		racked.SetOnClose(s.closePlayerWindow)
	} else {
		next.MainWindow().SetOnCloseRequest(s.closePlayerWindow)
	}
	if s.fileWindow != nil && !s.fileWindow.Closed() {
		s.fileWindow.SetOwner(next.MainWindow())
	}
	if s.settingsWindow != nil && !s.settingsWindow.Closed() {
		s.settingsWindow.SetOwner(next.MainWindow())
	}
	if s.note.window != nil && !s.note.window.Closed() {
		s.note.window.SetOwner(next.MainWindow())
	}
	if w := s.LibraryWindow(); w != nil {
		w.SetOwner(next.MainWindow())
	}
	s.Host.RememberSkin(skin)
	if old != nil {
		old.Close()
	}
	s.restart()
	s.refresh()
	s.remember()
	return nil
}

func (s *Shell) restart() {
	m := s.Host.Model
	m.Transport.Changed = s.refresh
	m.Equaliser.Changed = func() { s.refresh(); s.syncPlayback() }
	s.pulse.Stop()
	// Window following, startup placement and session snapshots still run
	// when playback is paused or stopped. Live time comes from the decoder.
	if s.presentation != nil && (m.Live || m.Transport.State == players.Playing || s.skin != players.DefaultSkin) {
		s.pulse.Start(s.MainWindow())
	}
}

func (s *Shell) refresh() {
	if s.presentation != nil {
		s.presentation.Refresh()
	}
}

func (s *Shell) tick(dt time.Duration) {
	m := s.Host.Model
	if !m.Live {
		m.Transport.Tick(dt)
		m.Spectrum.Advance(m.Transport.Pos, m.Transport.State == players.Playing, dt)
	} else {
		m.Spectrum.AdvanceLive(m.Transport.State == players.Playing, dt)
	}
	if s.presentation != nil {
		s.presentation.Advance(dt)
	}
	s.rememberIn += dt
	if s.rememberIn >= 2*time.Second {
		s.remember()
	}
}

func (s *Shell) Command(c players.Command) {
	if s.player == nil {
		s.Host.Model.Transport.Do(c)
	} else if name := command(c); name != "" {
		s.syncPlayback() // a track chosen, a seek, before the command
		s.call(func(ctx context.Context) (musicapi.State, error) { return s.player.Command(ctx, name) })
	}
	s.remember()
	if s.presentation != nil && s.Host.Model.Transport.State == players.Playing {
		s.pulse.Start(s.MainWindow())
	}
}

func (s *Shell) Keys(w *ui.Window, e widget.KeyEvent) bool {
	if e.Key == platform.KeyEscape || (e.Mods.Ctrl() && e.Key == platform.KeyQ) {
		s.quitPlayer()
		return true
	}
	if e.Mods.Ctrl() {
		if e.Key == platform.KeyComma {
			s.openSettings()
			return true
		}
		if e.Key == platform.KeyB && s.Host.ShowLibrary != nil {
			s.openLibrary()
			return true
		}
		if e.Key == platform.KeyO {
			s.Host.OpenFiles(w.Content(), false)
			return true
		}
		if e.Key == platform.KeyS {
			s.savePlaylistAs()
			return true
		}
		if e.Key == platform.KeyE {
			s.presentation.ShowEqualiser(!s.Host.Model.EqualizerVisible)
			return true
		}
		if e.Key == platform.KeyL {
			s.presentation.ShowPlaylist(!s.Host.Model.PlaylistVisible)
			return true
		}
		if e.Key == platform.KeyK {
			if e.Mods.Shift() {
				s.Host.DropSkin()
			} else {
				s.Host.NextSkin()
			}
			return true
		}
		return false
	}
	if players.Typing(w.Focus()) {
		return false
	}
	if c := players.CommandFor(e); c != players.CmdNone {
		s.Command(c)
		return true
	}
	return false
}

func (s *Shell) Status() string {
	t := s.Host.Model.Transport
	return fmt.Sprintf("%s · %s · %s", s.skin, t.State, players.Percent(t.Volume))
}
