package players

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/codemodify/uitoolkit/style"
)

// What the player remembers between runs, which is two things: the face it
// was last wearing, and the pack each face was last switched to.
//
// It is kept beside the toolkit's own settings, in
// $XDG_CONFIG_HOME/uitoolkit/media-player-music.json, and it is read before
// the application is built — the look is chosen when the application is, so
// the skin has to be known first.

// Settings is that file.
type Settings struct {
	// Face is the face to wear, by id.
	Face string `json:"face,omitempty"`
	// Skins is the pack each face was last switched to, by face id.
	Skins map[string]string `json:"skins,omitempty"`
}

// SettingsPath is where the file is.
func SettingsPath() string {
	return filepath.Join(style.ConfigDir(), "media-player-music.json")
}

// LoadSettings reads what was remembered. A missing or unreadable file is
// the zero Settings, which is "no preference" and not an error: a player
// that refused to start because its preferences were malformed would be a
// player with its priorities the wrong way round.
func LoadSettings() Settings {
	var s Settings
	b, err := os.ReadFile(SettingsPath())
	if err != nil || json.Unmarshal(b, &s) != nil {
		return Settings{}
	}
	return s
}

// SkinOf is the pack a face was last switched to, or "" for one it never
// was — or one the file names and this build cannot wear.
func (s Settings) SkinOf(face string) string {
	id := s.Skins[face]
	if id == "" {
		return ""
	}
	if _, ok := style.LoadTheme(id); !ok && !style.IsSkin(id) {
		return ""
	}
	return id
}

// WithSkin is the settings with a face's pack remembered.
func (s Settings) WithSkin(face, pack string) Settings {
	if s.Skins == nil {
		s.Skins = map[string]string{}
	}
	s.Skins[face] = pack
	return s
}

// Save writes the file, through a temporary one so a run that is killed
// half way through writing leaves the last good answer behind.
func (s Settings) Save() error {
	path := SettingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
