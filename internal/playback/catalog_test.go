package playback

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestDiscoveryFindsExecutablesWithoutStartingThem(t *testing.T) {
	for _, goos := range []string{"linux", "darwin", "windows"} {
		t.Run(goos, func(t *testing.T) {
			look := func(name string) (string, error) {
				if name == "mpv" || name == "mpv.exe" {
					return "installed-mpv", nil
				}
				return "", fmt.Errorf("missing")
			}
			infos := detectFor(goos, look, func(string) string { return "" })
			if len(infos) != 5 || infos[0].ID != BuiltinID || !infos[0].Available || infos[0].Executable != "" {
				t.Fatal("the built-in engine is not listed first")
			}
			if infos[1].ID != "mpv" || !infos[1].Available || infos[1].Executable != "installed-mpv" {
				t.Fatal("mpv was not detected")
			}
			for _, info := range infos[2:] {
				if info.Available || info.Executable != "" {
					t.Fatalf("missing %s marked available", info.ID)
				}
			}
		})
	}
}

func TestDiscoveryFindsMacAppAndWindowsInstallWithoutPATH(t *testing.T) {
	for _, tc := range []struct{ goos, path string }{
		{"darwin", "/Applications/VLC.app/Contents/MacOS/VLC"},
		{"windows", filepath.Join("Programs", "VideoLAN", "VLC", "vlc.exe")},
	} {
		infos := detectFor(tc.goos, func(name string) (string, error) {
			if name == tc.path {
				return name, nil
			}
			return "", fmt.Errorf("missing")
		}, func(key string) string {
			if key == "ProgramFiles" {
				return "Programs"
			}
			return ""
		})
		if !infos[2].Available || infos[2].Executable != tc.path {
			t.Fatalf("%s application executable missed", tc.goos)
		}
	}
}
