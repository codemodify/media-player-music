package players

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestPreferencesMigrateOnlySupportedSkinLooks(t *testing.T) {
	for _, test := range []struct {
		json string
		want string
	}{
		{`{"skin":"winamp"}`, "winamp"},
		{`{"skin":"winamp-silver"}`, "winamp-silver"},
		{`{"skin":"winamp-classified"}`, WinampClassifiedSkin},
		{`{"face":"minim","skins":{"minim":"minim-classic"}}`, "winamp"},
		{`{"face":"minim","skins":{"minim":"minim-silver"}}`, "winamp-silver"},
		{`{"face":"minim","skins":{"minim":"minim"}}`, DefaultSkin},
		{`{"face":"lantern","skins":{"minim":"minim-classic","lantern":"lantern"}}`, DefaultSkin},
		{`{"skin":"unsupported"}`, DefaultSkin},
		{`not json`, DefaultSkin},
	} {
		t.Run(test.json, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			path := SettingsPath()
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(test.json), 0o600); err != nil {
				t.Fatal(err)
			}
			if got := LoadSettings().Skin; got != test.want {
				t.Fatalf("loaded %q, want %q", got, test.want)
			}
		})
	}
}

func TestSettingsSavedUnderUitoolkitCarryOverUntilTheFirstSave(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	old := filepath.Join(config, "uitoolkit", "media-player-music.json")
	if err := os.MkdirAll(filepath.Dir(old), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(old, []byte(`{"skin":"winamp"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := LoadSettings().Skin; got != WinampSkin {
		t.Fatalf("uitoolkit's settings loaded %q, want %q", got, WinampSkin)
	}
	if err := (Settings{Skin: WinampSilverSkin}).Save(); err != nil {
		t.Fatal(err)
	}
	if SettingsPath() == old {
		t.Fatal("saved over uitoolkit's settings")
	}
	if got := LoadSettings().Skin; got != WinampSilverSkin {
		t.Fatalf("after saving loaded %q, want %q", got, WinampSilverSkin)
	}
	if data, err := os.ReadFile(old); err != nil || string(data) != `{"skin":"winamp"}` {
		t.Fatalf("uitoolkit's settings changed: %q, %v", data, err)
	}
}

func TestSessionSettingsRoundTripAndPermissions(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	want := Settings{
		Skin: WinampClassifiedSkin, Playlist: &SavedPlaylist{Tracks: []Track{{Path: "/music/song.flac", Title: "Song", Length: time.Minute}}, Index: 0, Position: 3 * time.Second},
		Panels: &SavedPanels{Equaliser: false, Playlist: true},
		Windows: map[string]map[string]WindowPlacement{"winamp": {
			"main":     {X: 0, Y: -20, Positioned: true},
			"playlist": {X: 0, Y: 96, Positioned: true, Dock: &WindowDock{Parent: "main", Side: "bottom", Along: 0}},
		}},
	}
	if err := want.Save(); err != nil {
		t.Fatal(err)
	}
	if got := LoadSettings(); !reflect.DeepEqual(got, want) {
		t.Fatalf("lost session data: %#v", got)
	}
	info, err := os.Stat(SettingsPath())
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("session file must be private: %v, %v", info, err)
	}
	entries, err := os.ReadDir(filepath.Dir(SettingsPath()))
	if err != nil || len(entries) != 1 {
		t.Fatal("saving left behind a temporary file")
	}
}

func TestAutoRemembersTheTrackAcrossViewsAndPreservesManualChanges(t *testing.T) {
	e := NewEqualizer()
	e.SetAuto(true, 2)
	if !e.Auto || e.Preset != EqPresets()[2].Name {
		t.Fatal("Auto did not apply the track's preset")
	}
	e.Set(0, 7)
	e.AutoForTrack(2)
	if e.Gains[0] != 7 || e.Preset != "Custom" {
		t.Fatal("refreshing the current track erased a manual adjustment")
	}
	e.AutoForTrack(3)
	if e.Preset != EqPresets()[3].Name {
		t.Fatal("new track did not get its preset")
	}
	e.SetAuto(false, 3)
	e.Set(0, 9)
	e.AutoForTrack(4)
	if e.Gains[0] != 9 {
		t.Fatal("Auto ran while disabled")
	}
}
