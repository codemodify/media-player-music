package players

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSettingsLoadOnlyWhatThePlayerHas(t *testing.T) {
	for _, test := range []struct {
		json string
		want Settings
	}{
		{`{"skin":"winamp"}`, Settings{Skin: WinampSkin}},
		{`{"skin":"winamp-classified"}`, Settings{Skin: WinampClassifiedSkin}},
		{`{"skin":"unsupported"}`, Settings{Skin: DefaultSkin}},
		// What earlier test builds saved is not carried over: an old
		// skin name or engine is simply not the player's.
		{`{"skin":"minim-classic","audio_backend":"mpv"}`, Settings{Skin: DefaultSkin}},
		{`{"face":"minim","skins":{"minim":"minim-classic"}}`, Settings{Skin: DefaultSkin}},
		{`{"skin":"winamp","replaygain":{"mode":"album","preamp_db":-3.5}}`, Settings{Skin: WinampSkin, ReplayGain: ReplayGain{Mode: ReplayGainAlbum, Preamp: -3.5}}},
		{`{"skin":"winamp","replaygain":{"mode":"loudest","preamp_db":99}}`, Settings{Skin: WinampSkin, ReplayGain: ReplayGain{Preamp: MaxPreamp}}},
		{`not json`, Settings{Skin: DefaultSkin}},
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
			if got := LoadSettings(); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("loaded %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestSettingsOmitReplayGainWhenOff(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := (Settings{Skin: DefaultSkin}).Save(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(SettingsPath())
	if err != nil || strings.Contains(string(data), "replaygain") || strings.Contains(string(data), "audio_backend") {
		t.Fatalf("saved %s, %v", data, err)
	}
}

func TestSessionSettingsRoundTripAndPermissions(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	want := Settings{
		Skin: WinampClassifiedSkin, ReplayGain: ReplayGain{Mode: ReplayGainTrack, Preamp: 2}, Playlist: &SavedPlaylist{Tracks: []Track{{Path: "/music/song.flac", Title: "Song", Length: time.Minute}}, Index: 0, Position: 3 * time.Second},
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
