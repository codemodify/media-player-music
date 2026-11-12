// Package testenv keeps a test binary off the person's own environment: their
// look (look.json, uitoolkit's too), their themes, icon sets, skins and fonts
// under ~/.config and ~/.local/share, the player's saved session, and the
// desktop-ui variables that choose a look for a process. A test that reached
// any of them would pass or fail by the machine it ran on (a look.json naming
// another theme failed the video player's skin tests), and could write the
// person's files.
//
// Every test package that builds on desktop-ui calls [Main] from its
// TestMain (playertest.Main does). goframework has no such call (R102): the
// variables below are its, spread over four packages, and the list goes stale
// when it grows one.
package testenv

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
)

// lookEnv are desktop-ui's variables that choose a look, a scale or a frame
// for the process: set in the shell the tests are run from, they would win
// over what a test asks for. A test that wants one sets it with t.Setenv.
// R102: goframework's list, kept here by hand.
var lookEnv = []string{
	style.ThemeEnv, style.FontEnv, style.MonoFontEnv, style.AnimationsEnv,
	style.IconThemeDirsEnv, style.SystemFontsEnv,
	ui.ColorSchemeEnv, ui.AccentEnv, ui.DirectionEnv, ui.EnvDecorationPalette,
	platform.EnvScale, platform.EnvDecorations,
}

// Main runs m with the environment isolated (Isolate) in a folder of its
// own, removed afterwards, and returns its exit code.
func Main(m *testing.M) int {
	dir, err := os.MkdirTemp("", "media-player-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(dir)
	if err := Isolate(dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return m.Run()
}

// Isolate points HOME and the XDG folders at new folders in dir and unsets
// desktop-ui's look variables, for the rest of the process. On X11 a display
// the tests were given on purpose (the opt-in native tests) keeps its
// authority file, which lived in the real home.
func Isolate(dir string) error {
	if os.Getenv("DISPLAY") != "" && os.Getenv("XAUTHORITY") == "" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			os.Setenv("XAUTHORITY", filepath.Join(home, ".Xauthority"))
		}
	}
	folders := map[string]string{
		"HOME": "home", "XDG_CONFIG_HOME": "config", "XDG_CACHE_HOME": "cache",
		"XDG_DATA_HOME": "data", "XDG_STATE_HOME": "state", "XDG_RUNTIME_DIR": "run",
	}
	if runtime.GOOS == "windows" {
		folders["USERPROFILE"], folders["APPDATA"], folders["LOCALAPPDATA"] = "home", "appdata", "localappdata"
	}
	for name, sub := range folders {
		path := filepath.Join(dir, sub)
		if err := os.MkdirAll(path, 0o700); err != nil {
			return err
		}
		if err := os.Setenv(name, path); err != nil {
			return err
		}
	}
	// The system's own folders stay (fonts in /usr/share), the person's
	// configuration folders beside them do not.
	os.Unsetenv("XDG_CONFIG_DIRS")
	for _, name := range lookEnv {
		os.Unsetenv(name)
	}
	return nil
}
