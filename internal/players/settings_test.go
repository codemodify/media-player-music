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
		// ReplayGain's default, when none is saved, is track gain (the
		// owner, 2026-11-12).
		{`{"skin":"winamp"}`, Settings{Skin: WinampSkin, ReplayGain: ReplayGain{Mode: ReplayGainTrack}}},
		{`{"skin":"winamp-classified"}`, Settings{Skin: WinampClassifiedSkin, ReplayGain: ReplayGain{Mode: ReplayGainTrack}}},
		{`{"skin":"unsupported"}`, Settings{Skin: DefaultSkin, ReplayGain: ReplayGain{Mode: ReplayGainTrack}}},
		// What earlier test builds saved is not carried over: an old
		// skin name or engine is simply not the player's.
		{`{"skin":"minim-classic","audio_backend":"mpv"}`, Settings{Skin: DefaultSkin, ReplayGain: ReplayGain{Mode: ReplayGainTrack}}},
		{`{"face":"minim","skins":{"minim":"minim-classic"}}`, Settings{Skin: DefaultSkin, ReplayGain: ReplayGain{Mode: ReplayGainTrack}}},
		{`{"skin":"winamp","replaygain":{"mode":"album","preamp_db":-3.5}}`, Settings{Skin: WinampSkin, ReplayGain: ReplayGain{Mode: ReplayGainAlbum, Preamp: -3.5}}},
		{`{"skin":"winamp","replaygain":{"mode":"off"}}`, Settings{Skin: WinampSkin, ReplayGain: ReplayGain{Mode: ReplayGainOff}}},
		{`{"skin":"winamp","replaygain":{"preamp_db":2}}`, Settings{Skin: WinampSkin, ReplayGain: ReplayGain{Mode: ReplayGainTrack, Preamp: 2}}},
		{`{"skin":"winamp","replaygain":{"mode":"loudest","preamp_db":99}}`, Settings{Skin: WinampSkin, ReplayGain: ReplayGain{Mode: ReplayGainOff, Preamp: MaxPreamp}}},
		{`not json`, Settings{Skin: DefaultSkin, ReplayGain: ReplayGain{Mode: ReplayGainTrack}}},
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

// TestSettingsReplayGainDefaultsToTrack: nothing saved is track gain, and
// off, once chosen, is saved and kept (the owner, 2026-11-12).
func TestSettingsReplayGainDefaultsToTrack(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if got := LoadSettings().ReplayGain; got != (ReplayGain{Mode: ReplayGainTrack}) {
		t.Fatalf("with no settings file: %+v, want track gain", got)
	}
	if err := (Settings{Skin: DefaultSkin}).Save(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(SettingsPath())
	if err != nil || strings.Contains(string(data), "replaygain") || strings.Contains(string(data), "audio_backend") {
		t.Fatalf("saved %s, %v", data, err)
	}
	if got := LoadSettings().ReplayGain; got != (ReplayGain{Mode: ReplayGainTrack}) {
		t.Fatalf("with none saved: %+v, want track gain", got)
	}
	// what was loaded, saved again (the shell does), still holds no default
	if err := LoadSettings().Save(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(SettingsPath()); err != nil || strings.Contains(string(data), "replaygain") {
		t.Fatalf("the loaded default was written back: %s, %v", data, err)
	}
	if err := (Settings{Skin: DefaultSkin, ReplayGain: ReplayGain{Mode: ReplayGainOff}}).Save(); err != nil {
		t.Fatal(err)
	}
	if got := LoadSettings().ReplayGain; got != (ReplayGain{Mode: ReplayGainOff}) {
		t.Fatalf("off saved: %+v", got)
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
