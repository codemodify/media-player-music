package skins

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/codemodify/media-player-music/internal/skins/plans"
	"github.com/codemodify/uitoolkit/style"
)

var packNames = []string{"amarok", "calculator", "dosamp", "fandom", "half-life-2", "headspace", "itunes", "quake-3-arena", "radar-amp", "realplayer", "series-9", "sony-cdx-mp3", "ti-82", "vlc", "winamp", "winamp-amber", "winamp-classified", "winamp-silver", "windows-98", "xbox"}

var packLabels = map[string]string{
	"calculator": "Calculator", "dosamp": "DOSamp", "quake-3-arena": "Quake 3 Arena", "radar-amp": "Radar Amp",
	"sony-cdx-mp3": "Sony CDX-MP3", "ti-82": "TI-82", "winamp": "Winamp", "winamp-amber": "Winamp Classic Amber",
	"winamp-classified": "Winamp Classified", "winamp-silver": "Winamp Silver", "windows-98": "Windows 98",
	"headspace":   "Win Media Player - Headspace",
	"xbox":        "Win Media Player - Xbox",
	"half-life-2": "Win Media Player - Half Life 2",
	"fandom":      "Win Media Player - Fandom",
	"itunes":      "Mac OS X - iTunes",
	"series-9":    "Win Media Player - Series 9",
	"vlc":         "VLC - Classic",
	"realplayer":  "RealPlayer - Classic",
	"amarok":      "KDE - Amarok 1.4",
}

func TestBundledPacksLoadAndLint(t *testing.T) {
	entries, err := bundled.ReadDir("packs")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if !reflect.DeepEqual(names, packNames) {
		t.Fatalf("bundled packs = %v, want %v", names, packNames)
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			pack, err := fs.Sub(bundled, "packs/"+name)
			if err != nil {
				t.Fatal(err)
			}
			sk, err := style.LoadSkinFS(pack, name)
			if err != nil {
				t.Fatalf("load bundled pack: %v", err)
			}
			if sk.Label == "" || sk.Summary == "" || sk.Base == "" {
				t.Error("a bundled pack must identify itself and its fallback look")
			}
			label := packLabels[name]
			if sk.Label != label {
				t.Errorf("label = %q, want %q", sk.Label, label)
			}
			layouts := []string{"winamp.strip", "winamp.equaliser", "winamp.playlist"}
			if name == "headspace" {
				layouts = []string{"headspace.main"}
			}
			if name == "xbox" {
				layouts = []string{"xbox.main", "xbox.playlist"}
			}
			if name == "half-life-2" {
				layouts = []string{"halflife2.main", "halflife2.equalizer", "halflife2.playlist"}
			}
			if name == "fandom" {
				layouts = []string{"fandom.main", "fandom.playlist"}
			}
			if name == "itunes" {
				layouts = []string{"itunes.main", "itunes.mini", "itunes.equalizer"}
			}
			if name == "series-9" {
				layouts = []string{"series9.main", "series9.transport", "series9.equalizer", "series9.playlist", "series9.compact"}
			}
			if name == "vlc" {
				layouts = []string{"vlc.main"}
			}
			if name == "realplayer" {
				layouts = []string{"realplayer.main", "realplayer.playlist", "realplayer.equalizer"}
			}
			if name == "amarok" {
				layouts = []string{"amarok.main"}
			}
			if len(sk.Layouts) != len(layouts) {
				t.Errorf("layout count = %d, want %d", len(sk.Layouts), len(layouts))
			}
			for _, layout := range layouts {
				if sk.Layouts[layout] == nil {
					t.Errorf("missing player layout %q", layout)
				}
			}
			for _, w := range style.LintSkin(sk) {
				// The app deliberately paints panel digits, letters, lamps,
				// and sliders by sprite name instead of toolkit part binding.
				if w.Key != "sprites" {
					t.Errorf("%s", w)
				}
			}
		})
	}
}

func TestGeneratedPacksMatchBundle(t *testing.T) {
	dir := t.TempDir()
	for _, plan := range plans.Plans() {
		if err := plans.Write(dir, plan); err != nil {
			t.Fatalf("generate %s: %v", plan.Name, err)
		}
	}
	if err := fs.WalkDir(bundled, "packs", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		want, err := bundled.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel("packs", filepath.FromSlash(path))
		if err != nil {
			return err
		}
		got, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			return err
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from its plan; run go generate ./internal/skins", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRegistrationPreservesUserOverride(t *testing.T) {
	isolateSkinLookups(t)
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := Register(); err != nil {
		t.Fatal(err)
	}
	paths := style.SearchPaths()
	if len(paths) != 0 {
		t.Fatalf("in-memory registration added search paths: %v", paths)
	}
	if err := Register(); err != nil {
		t.Fatal(err)
	}
	if got := style.SearchPaths(); !reflect.DeepEqual(got, paths) {
		t.Errorf("second registration changed search paths: %v", got)
	}
	listed := make(map[string]string)
	for _, pack := range style.ListSkins() {
		listed[pack.Name] = pack.Label
	}
	for _, name := range packNames {
		if listed[name] != packLabels[name] {
			t.Errorf("registered pack %s missing from skin enumeration: %q", name, listed[name])
		}
		sk, ok := style.LoadSkin(name)
		if !ok {
			t.Errorf("registered pack %s does not load", name)
			continue
		}
		if sk.Name != name {
			t.Errorf("pack %s loaded as %q", name, sk.Name)
		}
		// Lint decodes every embedded PNG and checks sprite bounds,
		// so successful manifest registration also proves its art is usable.
		for _, w := range style.LintSkin(sk) {
			if w.Key != "sprites" {
				t.Errorf("registered pack: %s", w)
			}
		}
		if sk.Dir != "" || sk.Path != "" || style.SkinSourceFile(name) != "" {
			t.Errorf("embedded %s unexpectedly has a filesystem source: %+v", name, sk)
		}
		if pack, ok := style.LoadTheme(name); !ok || pack.Label != sk.Label {
			t.Errorf("registered skin %s did not supply its theme pack: %+v, %v", name, pack, ok)
		}
	}
	if _, err := os.Stat(style.ConfigDir()); !os.IsNotExist(err) {
		t.Fatalf("registration wrote toolkit configuration: %v", err)
	}

	userDir := style.UserSkinDir("winamp")
	if err := os.MkdirAll(userDir, 0o700); err != nil {
		t.Fatal(err)
	}
	userManifest := filepath.Join(userDir, "skin.json")
	if err := os.WriteFile(userManifest, []byte(`{"skin":1,"label":"My Winamp","base":"win95"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	style.InvalidateSkinCache()
	if sk, ok := style.LoadSkin("winamp"); !ok || sk.Label != "My Winamp" {
		t.Fatalf("user skin did not override the app pack: %#v, %v", sk, ok)
	}
	if pack, ok := style.LoadTheme("winamp"); !ok || pack.Label != "My Winamp" {
		t.Fatalf("user skin did not override the app theme: %+v, %v", pack, ok)
	}
	if got := style.SkinSourceFile("winamp"); got != userManifest {
		t.Errorf("override source = %q, want %q", got, userManifest)
	}
	if err := os.RemoveAll(userDir); err != nil {
		t.Fatal(err)
	}
	style.InvalidateSkinCache()
	if sk, ok := style.LoadSkin("winamp"); !ok || sk.Label != "Winamp" {
		t.Fatalf("removing the override did not restore the embedded app skin: %#v, %v", sk, ok)
	}
}

func TestRegistrationNeedsNoWritableDirectory(t *testing.T) {
	isolateSkinLookups(t)
	config := filepath.Join(t.TempDir(), "missing-config")
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("XDG_CONFIG_HOME", config)
	if err := Register(); err != nil {
		t.Fatalf("embedded registration required a writable temporary directory: %v", err)
	}
	if paths := style.SearchPaths(); len(paths) != 0 {
		t.Errorf("embedded registration installed search paths: %v", paths)
	}
	if _, err := os.Stat(config); !os.IsNotExist(err) {
		t.Fatalf("registration wrote configuration: %v", err)
	}
	for _, name := range packNames {
		if sk, ok := style.LoadSkin(name); !ok || sk.Dir != "" {
			t.Errorf("embedded %s was not available without disk extraction: %#v, %v", name, sk, ok)
		}
	}
}

func isolateSkinLookups(t *testing.T) {
	t.Helper()
	style.ResetSearchPathsForTest()
	style.InvalidateSkinCache()
	t.Cleanup(func() {
		style.ResetSearchPathsForTest()
		style.InvalidateSkinCache()
	})
}
