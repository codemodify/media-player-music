package shell

import (
	"encoding/json"
	"fmt"
	"github.com/codemodify/media-player-music/internal/playback"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/platform"
)

type miniPresentation interface {
	SetMiniPlayer(bool)
	MiniPlayer() bool
	StatefulPresentation
}

func iTunesModes(t *testing.T, s *Shell) miniPresentation {
	t.Helper()
	p, ok := s.Presentation().(miniPresentation)
	if !ok {
		t.Fatalf("iTunes presentation %T does not expose full/MiniPlayer modes", s.Presentation())
	}
	return p
}

func TestITunesRequiresAllThreeLayoutsBeforeReplacingPresentation(t *testing.T) {
	for _, missing := range []string{"itunes.main", "itunes.mini", "itunes.equalizer"} {
		t.Run(missing, func(t *testing.T) {
			a, s := open(t, players.FandomSkin)
			w, model := s.MainWindow(), s.Host.Model
			dir := skins.UserSkinDir(players.ITunesSkin)
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			layouts := map[string]any{}
			for _, name := range []string{"itunes.main", "itunes.mini", "itunes.equalizer"} {
				if name != missing {
					layouts[name] = map[string]any{"size": []int{800, 560}}
				}
			}
			data, err := json.Marshal(map[string]any{"skin": 1, "label": "Incomplete iTunes", "base": "dark", "layouts": layouts})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, skins.SkinFile), data, 0600); err != nil {
				t.Fatal(err)
			}
			skins.InvalidateSkinCache()
			t.Cleanup(skins.InvalidateSkinCache)
			if err := s.SetSkin(players.ITunesSkin); err == nil || !strings.Contains(err.Error(), fmt.Sprintf("missing layout %q", missing)) {
				t.Fatalf("incomplete iTunes override: %v", err)
			}
			if s.MainWindow() != w || w.Closed() || s.Skin() != players.FandomSkin || s.Host.Model != model {
				t.Fatal("invalid iTunes choice damaged the active family")
			}
			if err := s.SetSkin(players.DefaultSkin); err != nil {
				t.Fatal(err)
			}
			s.Host.DropSkin()
			a.PumpOnce()
			if s.Skin() != players.FandomSkin {
				t.Fatal("invalid iTunes choice replaced the last working skin")
			}
		})
	}
}

func TestITunesSkinKeysCycleFromFandomAndRememberChoice(t *testing.T) {
	a, s := open(t, players.FandomSkin)
	model := s.Host.Model
	for _, tc := range []struct {
		mods platform.Modifiers
		want string
	}{
		{platform.ModCtrl, players.ITunesSkin},
		{platform.ModCtrl | platform.ModShift, players.DefaultSkin},
		{platform.ModCtrl | platform.ModShift, players.ITunesSkin},
	} {
		s.MainWindow().Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyK, Mods: tc.mods})
		a.PumpOnce()
		a.PumpOnce()
		if s.Skin() != tc.want || s.Host.Model != model {
			t.Fatalf("skin keys chose %s, want %s around the shared model", s.Skin(), tc.want)
		}
	}
	if len(s.Windows()) != 2 || iTunesModes(t, s).MiniPlayer() {
		t.Fatal("iTunes should initially open the full player and owned EQ")
	}
	for _, named := range s.Windows() {
		if named.Name == "equaliser" {
			off := named.Window.Surface().(*platform.Offscreen)
			if off.WindowRole() != platform.RoleUtility || off.Owner() != s.MainWindow().Surface() || !off.SkipsTaskbar() {
				t.Fatal("iTunes EQ lost main ownership or utility taskbar policy")
			}
		}
	}
}

func TestITunesModesKeepNativeMainPlaybackAndTray(t *testing.T) {
	a, s, tray, backend := openFakeTray(t, players.ITunesSkin)
	if err := s.LoadFiles(mediaPaths(t, 1), false); err != nil {
		t.Fatal(err)
	}
	loaded(s)
	model, transport, queue := s.Host.Model, s.Host.Model.Transport, s.Host.Model.Transport.List
	transport.Pos = 17 * time.Second
	s.Host.SetVolume(.37)
	s.Host.SetPan(-.4)
	s.presentation.ShowEqualiser(true)
	s.presentation.ShowPlaylist(true)
	s.Start()
	p := iTunesModes(t, s)
	main, native, pulse := s.MainWindow(), s.MainWindow().Surface(), s.Pulse()
	loads, stops := countCommand(backend, playback.OpLoad), countCommand(backend, playback.OpStop)
	settle := func() { a.PumpOnce(); s.presentation.Advance(0) }
	main.Move(420, 210)
	main.SetSize(940, 650)
	settle()
	p.SetMiniPlayer(true)
	settle()
	main.SetSize(430, 62)
	settle()
	for _, mini := range []bool{false, true, false, true} {
		p.SetMiniPlayer(mini)
		settle()
		w, h := main.Size()
		want := players.PresentationSize{Width: 940, Height: 650}
		if mini {
			want = players.PresentationSize{Width: 430, Height: 62}
		}
		if p.MiniPlayer() != mini || w != want.Width || h != want.Height {
			t.Fatalf("mini=%t restored %dx%d, want %+v", mini, w, h, want)
		}
		if s.MainWindow() != main || main.Surface() != native || s.Pulse() != pulse || !pulse.Running() || s.tray != tray || len(a.StatusItems()) != 1 {
			t.Fatal("mode switch replaced the native main, clock or tray item")
		}
		if x, y, known := main.Position(); !known || x != 420 || y != 210 {
			t.Fatalf("mode switch moved the main window: %d,%d,%t", x, y, known)
		}
		if s.Host.Model != model || model.Transport != transport || transport.List != queue || transport.State != players.Playing || transport.Pos != 17*time.Second || transport.Volume != .37 || transport.Pan != -.4 || !model.PlaylistVisible || !model.EqualizerVisible {
			t.Fatal("mode switch changed playback or requested area visibility")
		}
		if countCommand(backend, playback.OpLoad) != loads || countCommand(backend, playback.OpStop) != stops || backend.closed || a.Quitting() {
			t.Fatal("mode switch interrupted the live backend/application")
		}
	}
	main.Inject(platform.Event{Kind: platform.EventClose})
	a.PumpOnce()
	if main.Closed() || main.Visible() || s.trayHidden == nil || !pulse.Running() || backend.closed || !p.MiniPlayer() {
		t.Fatal("closing the MiniPlayer did not hide the intact player to its tray")
	}
	for _, named := range s.Windows() {
		if named.Window.Visible() {
			t.Fatalf("close-to-tray left %s visible", named.Name)
		}
	}
	tray.Click()
	a.DrainPosted()
	if s.MainWindow() != main || main.Surface() != native || !main.Visible() || !p.MiniPlayer() || s.trayHidden != nil || backend.closed {
		t.Fatal("tray activation did not restore the same live MiniPlayer")
	}
	if got := p.CaptureState(); got.Mode != "mini" || got.Sizes["full"] != (players.PresentationSize{Width: 940, Height: 650}) || got.Sizes["mini"] != (players.PresentationSize{Width: 430, Height: 62}) {
		t.Fatalf("mode cache lost independently resized views: %+v", got)
	}
}

func TestITunesSessionRestoresModeBeforePlacementAndKeepsOtherFamiliesSeparate(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a, s, backend := rememberedShell(t, players.DefaultSkin)
	s.MainWindow().Move(720, 80)
	s.presentation.Advance(0)
	if err := s.SetSkin(players.ITunesSkin); err != nil {
		t.Fatal(err)
	}
	s.Pulse().Stop()
	a.PumpOnce()
	s.presentation.Advance(0)
	p := iTunesModes(t, s)
	main := s.MainWindow()
	main.Move(420, 210)
	main.SetSize(940, 650)
	a.PumpOnce()
	s.presentation.Advance(0)
	p.SetMiniPlayer(true)
	a.PumpOnce()
	main.SetSize(430, 62)
	a.PumpOnce()
	s.presentation.Advance(0)
	s.presentation.ShowEqualiser(false)
	s.presentation.ShowPlaylist(false)
	want := p.CaptureState()
	for _, id := range []string{players.FandomSkin, players.DefaultSkin, players.ITunesSkin} {
		if err := s.SetSkin(id); err != nil {
			t.Fatal(err)
		}
		s.Pulse().Stop()
		a.PumpOnce()
		s.presentation.Advance(0)
		if id == players.DefaultSkin {
			if x, y, ok := s.MainWindow().Position(); !ok || x != 720 || y != 80 {
				t.Fatal("iTunes modes changed the default family's placement")
			}
		}
	}
	if got := iTunesModes(t, s).CaptureState(); !reflect.DeepEqual(got, want) || countCommand(backend, playback.OpLoad) != 0 {
		t.Fatalf("family switch lost iTunes modes or started playback: %+v", got)
	}
	s.Close()
	_, reopened, nextBackend := rememberedShell(t, "")
	if reopened.Skin() != players.ITunesSkin || !iTunesModes(t, reopened).MiniPlayer() || !reflect.DeepEqual(iTunesModes(t, reopened).CaptureState(), want) {
		t.Fatal("iTunes session lost its mode or independent full/mini sizes")
	}
	x, y, known := reopened.MainWindow().Position()
	w, h := reopened.MainWindow().Size()
	if !known || x != 420 || y != 210 || w != 430 || h != 62 || countCommand(nextBackend, playback.OpLoad) != 0 || reopened.Host.Model.EqualizerVisible || reopened.Host.Model.PlaylistVisible {
		t.Fatalf("restored MiniPlayer lost position, geometry or area preferences: %d,%d,%t %dx%d", x, y, known, w, h)
	}
	iTunesModes(t, reopened).SetMiniPlayer(false)
	reopened.App.PumpOnce()
	if w, h := reopened.MainWindow().Size(); w != 940 || h != 650 {
		t.Fatalf("restored full view lost its inactive saved size: %dx%d", w, h)
	}
}
