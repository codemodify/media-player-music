package skins

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/codemodify/media-player-music/internal/skins/plans"
	uiskins "goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/style"
	"goframework.io/graphics-paint2d"
)

var packNames = []string{"aluminium", "amber", "bevel", "cobalt", "console", "dossier", "fancy", "fandom", "forge", "gauge", "graphing", "headunit", "pod", "radar", "sidebar", "silver", "slate", "tally", "textmode", "utility", "wingspan"}

var packLabels = map[string]string{
	"slate": "Slate", "silver": "Silver", "dossier": "Dossier", "textmode": "Text Mode", "bevel": "Bevel",
	"amber": "Amber", "tally": "Tally", "headunit": "Head Unit", "graphing": "Graphing", "radar": "Radar", "forge": "Forge",
	"wingspan": "Wingspan", "pod": "Pod", "gauge": "Gauge", "fandom": "Fandom", "aluminium": "Aluminium",
	"cobalt": "Cobalt", "utility": "Utility", "console": "Console", "sidebar": "Sidebar", "fancy": "Fancy",
}

var packLayouts = map[string][]string{
	"wingspan":  {"wingspan.main"},
	"pod":       {"pod.main", "pod.playlist"},
	"gauge":     {"gauge.main", "gauge.equalizer", "gauge.playlist"},
	"fandom":    {"fandom.main", "fandom.playlist"},
	"aluminium": {"aluminium.main", "aluminium.mini", "aluminium.equalizer"},
	"cobalt":    {"cobalt.main", "cobalt.transport", "cobalt.equalizer", "cobalt.playlist", "cobalt.compact"},
	"utility":   {"utility.main"},
	"console":   {"console.main", "console.playlist", "console.equalizer"},
	"sidebar":   {"sidebar.main"},
	"fancy":     {"fancy.main", "fancy.equalizer", "fancy.playlist", "fancy.compact"},
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
			sk, err := uiskins.LoadSkinFS(pack, name)
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
			layouts := []string{"panel.strip", "panel.equaliser", "panel.playlist"}
			if l, ok := packLayouts[name]; ok {
				layouts = l
			}
			if len(sk.Layouts) != len(layouts) {
				t.Errorf("layout count = %d, want %d", len(sk.Layouts), len(layouts))
			}
			for _, layout := range layouts {
				if sk.Layouts[layout] == nil {
					t.Errorf("missing player layout %q", layout)
				}
			}
			for _, w := range uiskins.LintSkin(sk) {
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
	for _, pack := range uiskins.ListSkins() {
		listed[pack.Name] = pack.Label
	}
	for _, name := range packNames {
		if listed[name] != packLabels[name] {
			t.Errorf("registered pack %s missing from skin enumeration: %q", name, listed[name])
		}
		sk, ok := uiskins.LoadSkin(name)
		if !ok {
			t.Errorf("registered pack %s does not load", name)
			continue
		}
		if sk.Name != name {
			t.Errorf("pack %s loaded as %q", name, sk.Name)
		}
		// Lint decodes every embedded PNG and checks sprite bounds,
		// so successful manifest registration also proves its art is usable.
		for _, w := range uiskins.LintSkin(sk) {
			if w.Key != "sprites" {
				t.Errorf("registered pack: %s", w)
			}
		}
		if sk.Dir != "" || sk.Path != "" || uiskins.SkinSourceFile(name) != "" {
			t.Errorf("embedded %s unexpectedly has a filesystem source: %+v", name, sk)
		}
		if pack, ok := style.LoadTheme(name); !ok || pack.Label != sk.Label {
			t.Errorf("registered skin %s did not supply its theme pack: %+v, %v", name, pack, ok)
		}
	}
	if _, err := os.Stat(style.ConfigDir()); !os.IsNotExist(err) {
		t.Fatalf("registration wrote toolkit configuration: %v", err)
	}

	userDir := uiskins.UserSkinDir("slate")
	if err := os.MkdirAll(userDir, 0o700); err != nil {
		t.Fatal(err)
	}
	userManifest := filepath.Join(userDir, "skin.json")
	if err := os.WriteFile(userManifest, []byte(`{"skin":1,"label":"My Slate","base":"win95"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	uiskins.InvalidateSkinCache()
	if sk, ok := uiskins.LoadSkin("slate"); !ok || sk.Label != "My Slate" {
		t.Fatalf("user skin did not override the app pack: %#v, %v", sk, ok)
	}
	if pack, ok := style.LoadTheme("slate"); !ok || pack.Label != "My Slate" {
		t.Fatalf("user skin did not override the app theme: %+v, %v", pack, ok)
	}
	if got := uiskins.SkinSourceFile("slate"); got != userManifest {
		t.Errorf("override source = %q, want %q", got, userManifest)
	}
	if err := os.RemoveAll(userDir); err != nil {
		t.Fatal(err)
	}
	uiskins.InvalidateSkinCache()
	if sk, ok := uiskins.LoadSkin("slate"); !ok || sk.Label != "Slate" {
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
		if sk, ok := uiskins.LoadSkin(name); !ok || sk.Dir != "" {
			t.Errorf("embedded %s was not available without disk extraction: %#v, %v", name, sk, ok)
		}
	}
}

func isolateSkinLookups(t *testing.T) {
	t.Helper()
	style.ResetSearchPathsForTest()
	uiskins.InvalidateSkinCache()
	t.Cleanup(func() {
		style.ResetSearchPathsForTest()
		uiskins.InvalidateSkinCache()
	})
}

// Every built-in skin registers, and every layout's face and every slot's
// art draws through the toolkit.
func TestEveryPackDrawsItsLayouts(t *testing.T) {
	if err := Register(); err != nil {
		t.Fatal(err)
	}
	for _, p := range plans.Plans() {
		t.Run(p.Name, func(t *testing.T) {
			if _, ok := uiskins.LoadSkin(p.Name); !ok {
				t.Fatal("does not load")
			}
			lk := style.Appearance{Name: p.Name}.Look()
			for _, l := range p.Layouts {
				if l.Art != "" {
					img := paint2d.NewImage(l.W, l.H)
					if !uiskins.DrawSkinSprite(lk, paint2d.NewContext(img), paint2d.XYWH(0, 0, float32(l.W), float32(l.H)), l.Art, paint2d.Color{}) {
						t.Errorf("%s: its face %s does not draw", l.Name, l.Art)
					} else if inked(img) == 0 {
						t.Errorf("%s: its face %s is empty", l.Name, l.Art)
					}
				}
				for _, slot := range l.Slots {
					if len(slot.Art) == 0 {
						continue
					}
					img := paint2d.NewImage(48, 48)
					if !uiskins.DrawSkinSlot(lk, paint2d.NewContext(img), paint2d.XYWH(0, 0, 48, 48), l.Name, slot.Name, 0) {
						t.Errorf("%s: slot %s does not draw", l.Name, slot.Name)
					} else if inked(img) == 0 {
						t.Errorf("%s: slot %s draws nothing", l.Name, slot.Name)
					}
				}
			}
		})
	}
}

func inked(img *paint2d.Image) int {
	n := 0
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] > 0 {
			n++
		}
	}
	return n
}
