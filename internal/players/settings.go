package players

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/codemodify/uitoolkit/style"
)

// Settings remembers the audio backend, appearance and desktop session. Window
// positions are logical desktop pixels and are grouped by presentation family.
type Settings struct {
	Skin          string                                `json:"skin"`
	AudioBackend  string                                `json:"audio_backend,omitempty"`
	Playlist      *SavedPlaylist                        `json:"playlist,omitempty"`
	Panels        *SavedPanels                          `json:"panels,omitempty"`
	Windows       map[string]map[string]WindowPlacement `json:"windows,omitempty"`
	Presentations map[string]PresentationState          `json:"presentations,omitempty"`
}

// NormalizeAudioBackend resolves old preferences to the default backend.
// Availability is checked separately; an installed program may disappear
// without changing the user's saved choice.
func NormalizeAudioBackend(id string) (string, bool) {
	switch id {
	case "", "mpv":
		return "mpv", true
	case "vlc", "mplayer", "mpg123":
		return id, true
	default:
		return "mpv", false
	}
}

type SavedPlaylist struct {
	Tracks   []Track       `json:"tracks"`
	Index    int           `json:"index"`
	Position time.Duration `json:"position_ns,omitempty"`
}

type SavedPanels struct {
	Equaliser bool `json:"equaliser"`
	Playlist  bool `json:"playlist"`
}

// A nil Dock means the window is detached. Positioned distinguishes a known
// origin at (0,0) from a desktop which cannot report window positions.
type WindowPlacement struct {
	X          int         `json:"x"`
	Y          int         `json:"y"`
	Width      int         `json:"width,omitempty"`
	Height     int         `json:"height,omitempty"`
	Positioned bool        `json:"positioned"`
	Dock       *WindowDock `json:"dock,omitempty"`
}

type WindowDock struct {
	Parent string `json:"parent"`
	Side   string `json:"side"`
	Along  int    `json:"along"`
}

// PresentationState keeps local view modes and their window sizes separate
// from the shared playback model and the native window's desktop placement.
// Each presentation validates its own modes and size limits when restoring.
type PresentationState struct {
	Mode  string                      `json:"mode,omitempty"`
	Sizes map[string]PresentationSize `json:"sizes,omitempty"`
}

type PresentationSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

func SettingsPath() string {
	return filepath.Join(style.ConfigDir(), "media-player-music.json")
}

// NormalizeSkin also accepts names written by older versions of the app.
func NormalizeSkin(id string) (string, bool) {
	switch id {
	case "", DefaultSkin, Themed, "dark", "breeze-night":
		return DefaultSkin, true
	case "minim-classic":
		return WinampSkin, true
	case "minim-silver":
		return WinampSilverSkin, true
	}
	if IsSkinned(id) {
		return id, true
	}
	return "", false
}

// LoadSettings migrates the old per-face preference when possible. Removed
// faces, malformed files and unknown packs fall back to the default UI.
func LoadSettings() Settings {
	var saved struct {
		Settings
		Face  string            `json:"face"`
		Skins map[string]string `json:"skins"`
	}
	data, err := os.ReadFile(SettingsPath())
	if err != nil || json.Unmarshal(data, &saved) != nil {
		return Settings{Skin: DefaultSkin}
	}
	id := saved.Skin
	if saved.AudioBackend != "" {
		saved.AudioBackend, _ = NormalizeAudioBackend(saved.AudioBackend)
	}
	if id == "" && saved.Face == "minim" {
		id = saved.Skins[saved.Face]
	}
	if skin, ok := NormalizeSkin(id); ok {
		saved.Skin = skin
		return saved.Settings
	}
	saved.Skin = DefaultSkin
	return saved.Settings
}

func (s Settings) Save() error {
	path := SettingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".media-player-music-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(append(data, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
