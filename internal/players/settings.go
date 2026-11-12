package players

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"goframework.io/desktop-ui/style"
	"goframework.io/vfs"
	disk "goframework.io/vfs-source-disk"
)

// Settings remembers the appearance, the ReplayGain settings and the desktop
// session. Window positions are logical desktop pixels and are grouped by
// presentation family.
type Settings struct {
	Skin          string                                `json:"skin"`
	ReplayGain    ReplayGain                            `json:"replaygain,omitzero"`
	Playlist      *SavedPlaylist                        `json:"playlist,omitempty"`
	Panels        *SavedPanels                          `json:"panels,omitempty"`
	Windows       map[string]map[string]WindowPlacement `json:"windows,omitempty"`
	Presentations map[string]PresentationState          `json:"presentations,omitempty"`
}

// ReplayGain is how tracks are levelled by their ReplayGain tags. Its zero
// value is the default: track gain, no preamp (the owner, 2026-11-12).
type ReplayGain struct {
	// Mode is ReplayGainOff, ReplayGainTrack or ReplayGainAlbum; "" (none
	// saved) is ReplayGainTrack, the default.
	Mode string `json:"mode,omitempty"`
	// Preamp is added to the tags' gain, in dB, from MinPreamp to MaxPreamp.
	Preamp float64 `json:"preamp_db,omitempty"`
}

// The ReplayGain modes, as saved.
const (
	ReplayGainOff   = "off"
	ReplayGainTrack = "track"
	ReplayGainAlbum = "album"
)

// The preamp's range, in dB.
const (
	MinPreamp = -15
	MaxPreamp = 15
)

// Normal returns the settings with no mode as the default, track gain, a
// mode the player does not know as off, and the preamp within its range.
func (r ReplayGain) Normal() ReplayGain {
	switch r.Mode {
	case "":
		r.Mode = ReplayGainTrack
	case ReplayGainOff, ReplayGainTrack, ReplayGainAlbum:
	default:
		r.Mode = ReplayGainOff
	}
	if !(r.Preamp >= MinPreamp) { // NaN too
		r.Preamp = MinPreamp
	}
	r.Preamp = min(r.Preamp, MaxPreamp)
	return r
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

const settingsName = "media-player-music.json"

// SettingsPath is where Save writes.
func SettingsPath() string {
	return filepath.Join(style.ConfigDir(), settingsName)
}

// NormalizeSkin returns a skin's canonical name (the default UI's look,
// Themed, as the menus give it, is DefaultSkin); ok is false for a name the
// player does not have.
func NormalizeSkin(id string) (string, bool) {
	if id == "" || id == DefaultSkin || id == Themed {
		return DefaultSkin, true
	}
	if IsSkinned(id) {
		return id, true
	}
	return "", false
}

// LoadSettings reads the saved settings. A malformed file gives the
// defaults; a skin the player does not have, the default UI.
func LoadSettings() Settings {
	var saved Settings
	data, err := os.ReadFile(SettingsPath())
	if err != nil || json.Unmarshal(data, &saved) != nil {
		return Settings{Skin: DefaultSkin, ReplayGain: ReplayGain{}.Normal()}
	}
	saved.ReplayGain = saved.ReplayGain.Normal()
	if skin, ok := NormalizeSkin(saved.Skin); ok {
		saved.Skin = skin
	} else {
		saved.Skin = DefaultSkin
	}
	return saved
}

// Save writes the settings whole or not at all (vfs-source-disk's
// WriteFile: a temporary file beside them, synced, renamed over them, the
// folder synced), private to the user. A default is not written, as
// everywhere in the file: ReplayGain's track gain is saved as nothing, so
// that a file holds only what the person chose.
func (s Settings) Save() error {
	path := SettingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	s.ReplayGain = s.ReplayGain.saved()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return disk.WriteFile(path, append(data, '\n'), vfs.Mode(0o600))
}

// saved is r as the file keeps it: the default mode, track gain, as none.
func (r ReplayGain) saved() ReplayGain {
	if r.Mode == ReplayGainTrack {
		r.Mode = ""
	}
	return r
}
