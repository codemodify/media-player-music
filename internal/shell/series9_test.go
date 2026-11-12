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

type compactPresentation interface {
	SetCompact(bool)
	Compact() bool
	StatefulPresentation
}

func series9Modes(t *testing.T, s *Shell) compactPresentation {
	t.Helper()
	p, ok := s.Presentation().(compactPresentation)
	if !ok {
		t.Fatalf("Series 9 presentation %T does not expose full/compact modes", s.Presentation())
	}
	return p
}

func TestSeries9RequiresAllFiveLayoutsBeforeReplacingPresentation(t *testing.T) {
	names := []string{"series9.main", "series9.transport", "series9.equalizer", "series9.playlist", "series9.compact"}
	for _, missing := range names {
		t.Run(missing, func(t *testing.T) {
			a, s := open(t, players.ITunesSkin)
			w, model := s.MainWindow(), s.Host.Model
			dir := skins.UserSkinDir(players.Series9Skin)
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			layouts := map[string]any{}
			for _, name := range names {
				if name != missing {
					layouts[name] = map[string]any{"size": []int{346, 344}}
				}
			}
			data, err := json.Marshal(map[string]any{"skin": 1, "label": "Incomplete Series 9", "base": "dark", "layouts": layouts})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, skins.SkinFile), data, 0600); err != nil {
				t.Fatal(err)
			}
			skins.InvalidateSkinCache()
			t.Cleanup(skins.InvalidateSkinCache)
			if err := s.SetSkin(players.Series9Skin); err == nil || !strings.Contains(err.Error(), fmt.Sprintf("missing layout %q", missing)) {
				t.Fatalf("incomplete Series 9 override: %v", err)
			}
			if s.MainWindow() != w || w.Closed() || s.Skin() != players.ITunesSkin || s.Host.Model != model {
				t.Fatal("invalid Series 9 choice damaged the active family")
			}
			if err := s.SetSkin(players.DefaultSkin); err != nil {
				t.Fatal(err)
			}
			s.Host.DropSkin()
			a.PumpOnce()
			if s.Skin() != players.ITunesSkin {
				t.Fatal("invalid Series 9 choice replaced the last working skin")
			}
		})
	}
}

func TestSeries9SkinKeysCycleFromITunesAndRememberChoice(t *testing.T) {
	a, s := open(t, players.ITunesSkin)
	model := s.Host.Model
	for _, tc := range []struct {
		mods platform.Modifiers
		want string
	}{
		{platform.ModCtrl, players.Series9Skin},
		{platform.ModCtrl, players.VLCSkin},
		{platform.ModCtrl, players.RealPlayerSkin},
		{platform.ModCtrl, players.AmarokSkin},
		{platform.ModCtrl, players.FancySkin},
		{platform.ModCtrl, players.DefaultSkin},
		{platform.ModCtrl | platform.ModShift, players.FancySkin},
	} {
		s.MainWindow().Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyK, Mods: tc.mods})
		a.PumpOnce()
		a.PumpOnce()
		if s.Skin() != tc.want || s.Host.Model != model {
			t.Fatalf("skin keys chose %s, want %s around the shared model", s.Skin(), tc.want)
		}
	}
	if err := s.SetSkin(players.Series9Skin); err != nil {
		t.Fatal(err)
	}
	if len(s.Windows()) != 1 || s.Windows()[0].Name != "main" || series9Modes(t, s).Compact() {
		t.Fatal("Series 9 should initially embed its full player and drawers in one main window")
	}
}

func TestSeries9ModesKeepNativeMainPlaybackDrawerPreferencesAndTray(t *testing.T) {
	a, s, tray, backend := openFakeTray(t, players.Series9Skin)
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
	p := series9Modes(t, s)
	main, native, pulse := s.MainWindow(), s.MainWindow().Surface(), s.Pulse()
	loads, stops := countCommand(backend, playback.OpLoad), countCommand(backend, playback.OpStop)
	settle := func() { a.PumpOnce(); s.presentation.Advance(0) }
	main.Move(420, 210)
	// The full size includes the 250-pixel playlist and 124-pixel EQ drawer.
	main.SetSize(740, 574)
	settle()
	for _, compact := range []bool{true, false, true, false, true} {
		p.SetCompact(compact)
		settle()
		w, h := main.Size()
		want := players.PresentationSize{Width: 740, Height: 574}
		if compact {
			want = players.PresentationSize{Width: 346, Height: 103}
		}
		if p.Compact() != compact || w != want.Width || h != want.Height {
			t.Fatalf("compact=%t restored %dx%d, want %+v", compact, w, h, want)
		}
		if s.MainWindow() != main || main.Surface() != native || s.Pulse() != pulse || !pulse.Running() || s.tray != tray || len(a.StatusItems()) != 1 || len(s.Windows()) != 1 {
			t.Fatal("mode switch replaced the native main, clock or tray, or opened a satellite")
		}
		if x, y, known := main.Position(); !known || x != 420 || y != 210 {
			t.Fatalf("mode switch moved the main window: %d,%d,%t", x, y, known)
		}
		if s.Host.Model != model || model.Transport != transport || transport.List != queue || transport.State != players.Playing || transport.Pos != 17*time.Second || transport.Volume != .37 || transport.Pan != -.4 || !model.PlaylistVisible || !model.EqualizerVisible {
			t.Fatal("mode switch changed playback or remembered drawer visibility")
		}
		if countCommand(backend, playback.OpLoad) != loads || countCommand(backend, playback.OpStop) != stops || backend.closed || a.Quitting() {
			t.Fatal("mode switch interrupted the live backend/application")
		}
	}
	main.Inject(platform.Event{Kind: platform.EventClose})
	a.PumpOnce()
	if main.Closed() || main.Visible() || s.trayHidden == nil || !pulse.Running() || backend.closed || !p.Compact() {
		t.Fatal("closing the compact player did not hide the intact player to its tray")
	}
	tray.Click()
	a.DrainPosted()
	if s.MainWindow() != main || main.Surface() != native || !main.Visible() || !p.Compact() || s.trayHidden != nil || backend.closed || !model.PlaylistVisible || !model.EqualizerVisible {
		t.Fatal("tray activation did not restore the same compact player and desired drawers")
	}
	if got := p.CaptureState(); got.Mode != "compact" || got.Sizes["full"] != (players.PresentationSize{Width: 490, Height: 450}) {
		t.Fatalf("mode cache did not preserve the full base size without drawer extras: %+v", got)
	}
}

func TestSeries9SessionRestoresCompactBeforePlacementAndKeepsOtherFamiliesSeparate(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a, s, backend := rememberedShell(t, players.DefaultSkin)
	s.MainWindow().Move(720, 80)
	s.presentation.Advance(0)
	if err := s.SetSkin(players.Series9Skin); err != nil {
		t.Fatal(err)
	}
	s.Pulse().Stop()
	a.PumpOnce()
	s.presentation.Advance(0)
	p := series9Modes(t, s)
	main := s.MainWindow()
	main.Move(420, 210)
	s.presentation.ShowEqualiser(true)
	s.presentation.ShowPlaylist(true)
	main.SetSize(740, 574)
	a.PumpOnce()
	s.presentation.Advance(0)
	s.presentation.ShowEqualiser(false)
	s.presentation.ShowPlaylist(false)
	p.SetCompact(true)
	a.PumpOnce()
	s.presentation.Advance(0)
	want := p.CaptureState()
	for _, id := range []string{players.ITunesSkin, players.DefaultSkin, players.Series9Skin} {
		if err := s.SetSkin(id); err != nil {
			t.Fatal(err)
		}
		s.Pulse().Stop()
		a.PumpOnce()
		s.presentation.Advance(0)
		if id == players.DefaultSkin {
			if x, y, ok := s.MainWindow().Position(); !ok || x != 720 || y != 80 {
				t.Fatal("Series 9 mode changed the default family's placement")
			}
		}
	}
	if got := series9Modes(t, s).CaptureState(); !reflect.DeepEqual(got, want) || countCommand(backend, playback.OpLoad) != 0 {
		t.Fatalf("family switch lost Series 9 state or started playback: %+v", got)
	}
	s.Close()
	_, reopened, nextBackend := rememberedShell(t, "")
	if reopened.Skin() != players.Series9Skin || !series9Modes(t, reopened).Compact() || !reflect.DeepEqual(series9Modes(t, reopened).CaptureState(), want) {
		t.Fatal("Series 9 session lost its compact mode or remembered full base size")
	}
	x, y, known := reopened.MainWindow().Position()
	w, h := reopened.MainWindow().Size()
	if !known || x != 420 || y != 210 || w != 346 || h != 103 || countCommand(nextBackend, playback.OpLoad) != 0 || reopened.Host.Model.EqualizerVisible || reopened.Host.Model.PlaylistVisible {
		t.Fatalf("restored compact player lost position, geometry or drawer preferences: %d,%d,%t %dx%d", x, y, known, w, h)
	}
	series9Modes(t, reopened).SetCompact(false)
	reopened.App.PumpOnce()
	reopened.Presentation().Advance(0)
	if w, h := reopened.MainWindow().Size(); w != 490 || h != 450 {
		t.Fatalf("restored full view lost its inactive base size: %dx%d", w, h)
	}
	reopened.Presentation().ShowEqualiser(true)
	reopened.Presentation().ShowPlaylist(true)
	reopened.App.PumpOnce()
	reopened.Presentation().Advance(0)
	if w, h := reopened.MainWindow().Size(); w != 740 || h != 574 {
		t.Fatalf("restored full view did not add its drawer dimensions once: %dx%d", w, h)
	}
}
