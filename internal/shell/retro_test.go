package shell

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/uitoolkit/style"
)

func TestClassicDesktopSkinsValidateLayoutsBeforeReplacingCurrentPlayer(t *testing.T) {
	for _, skin := range []struct {
		id    string
		roles []string
	}{
		{players.VLCSkin, []string{"vlc.main"}},
		{players.RealPlayerSkin, []string{"realplayer.main", "realplayer.playlist", "realplayer.equalizer"}},
		{players.AmarokSkin, []string{"amarok.main"}},
	} {
		for _, missing := range skin.roles {
			t.Run(skin.id+"/"+missing, func(t *testing.T) {
				_, s := open(t, players.DefaultSkin)
				main, model := s.MainWindow(), s.Host.Model
				layouts := map[string]any{}
				for _, role := range skin.roles {
					if role != missing {
						layouts[role] = map[string]any{"size": []int{720, 520}}
					}
				}
				dir := style.UserSkinDir(skin.id)
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
				data, err := json.Marshal(map[string]any{"skin": 1, "label": "Incomplete", "base": "dark", "layouts": layouts})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, style.SkinFile), data, 0600); err != nil {
					t.Fatal(err)
				}
				style.InvalidateSkinCache()
				t.Cleanup(style.InvalidateSkinCache)
				if err := s.SetSkin(skin.id); err == nil || !strings.Contains(err.Error(), fmt.Sprintf("missing layout %q", missing)) {
					t.Fatalf("missing required layout did not reject the appearance: %v", err)
				}
				if main.Closed() || s.MainWindow() != main || s.Host.Model != model || s.Skin() != players.DefaultSkin {
					t.Fatal("incomplete appearance damaged the current player")
				}
			})
		}
	}
}

func TestClassicDesktopFamiliesPreservePlaybackAndOwnTheirWindowPlacement(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a, s, tray, backend := openFakeTray(t, players.DefaultSkin)
	if err := s.LoadFiles(mediaPaths(t, 1), false); err != nil {
		t.Fatal(err)
	}
	identify(s, 61)
	model, transport, queue, pulse := s.Host.Model, s.Host.Model.Transport, s.Host.Model.Transport.List, s.Pulse()
	loads := countCommand(backend, "loadfile")
	placements := map[string]map[string]players.WindowPlacement{}
	for i, skin := range []string{players.VLCSkin, players.RealPlayerSkin, players.AmarokSkin} {
		if err := s.SetSkin(skin); err != nil {
			t.Fatal(err)
		}
		a.PumpOnce()
		s.Pulse().Stop()
		main := s.MainWindow()
		if main.Title() != "Music Player" || !main.Resizable() || len(s.Windows()) != 1 || s.Windows()[0].Name != "main" {
			t.Fatalf("%s did not create one resizable Music Player window", skin)
		}
		main.Move(120+i*80, 160+i*50)
		main.SetSize(980+i*40, 680+i*30)
		a.PumpOnce()
		s.presentation.Advance(0)
		placements[skin] = s.presentation.CapturePlacement()
		if presentationFamily(skin) != skin || s.Host.Model != model || model.Transport != transport || transport.List != queue || transport.State != players.Playing || countCommand(backend, "loadfile") != loads || s.Pulse() != pulse || s.tray != tray || backend.closed {
			t.Fatalf("%s changed shared playback, tray or its placement family", skin)
		}
	}
	for _, skin := range []string{players.VLCSkin, players.RealPlayerSkin, players.AmarokSkin, players.DefaultSkin} {
		if err := s.SetSkin(skin); err != nil {
			t.Fatal(err)
		}
		a.PumpOnce()
		s.Pulse().Stop()
		s.presentation.Advance(0)
		if skin != players.DefaultSkin && !reflect.DeepEqual(s.presentation.CapturePlacement(), placements[skin]) {
			t.Fatalf("%s did not restore its independent window position/size", skin)
		}
	}
}
