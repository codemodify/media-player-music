package winamp

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/playertest"
	"github.com/codemodify/media-player-music/internal/skins/plans"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

// A pack the player has never heard of can move the real controls without
// rebuilding them or changing their keyboard order and actions. Its pinned
// and stretched slots also respond to a window size the stock panel never
// uses, so this exercises application layout through the public skin format.
func TestAnExternalSkinCanRearrangeThePlayer(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, Skin, scale)
			s := p.strip
			playBefore, nextBefore := s.play.Bounds(), s.next.Bounds()
			playID, nextID := s.play.ID(), s.next.ID()
			ringBefore := skinKeyless(playertest.RingNames(p.Main, playertest.FocusRing(t, a, p.Main)))

			const custom = "winamp-third-party-layout"
			installExternalPanel(t, custom)
			p.SetSkin(custom)
			a.PumpOnce()
			if p.Worn() != custom {
				t.Fatalf("player wears %q, want the external pack %q", p.Worn(), custom)
			}
			if s.play.ID() != playID || s.next.ID() != nextID {
				t.Fatal("switching to the external layout rebuilt the controls")
			}
			for _, key := range []struct {
				name string
				c    widget.Component
				want paint2d.Rect
			}{{"play", s.play, nextBefore}, {"next", s.next, playBefore}} {
				if d := maxDiff(key.c.Bounds(), key.want); d > 0.5 {
					t.Errorf("%s did not move to the other key's slot: got %v, want %v", key.name, key.c.Bounds(), key.want)
				}
			}
			ringAfter := skinKeyless(playertest.RingNames(p.Main, playertest.FocusRing(t, a, p.Main)))
			if !sameOrder(ringBefore, ringAfter) {
				t.Errorf("moving the keys changed keyboard order: before %v, after %v", ringBefore, ringAfter)
			}

			checkFlexibleExternalSlots(t, p, scale)
			keyBefore, seekBefore := s.skinBtn.Bounds(), s.seek.Bounds()
			width, height := p.Main.Size()
			p.Main.SetSize(width+40, height)
			a.PumpOnce()
			checkFlexibleExternalSlots(t, p, scale)
			if s.skinBtn.Bounds().Min.X <= keyBefore.Min.X || s.seek.Bounds().Dx() <= seekBefore.Dx() {
				t.Fatal("the external skin's pinned and stretched controls did not follow the larger window")
			}

			tree := playertest.Audit(t, "external strip", p.Main)
			node := tree.Find(playID)
			if node == nil || node.Name != "Play" {
				t.Fatalf("the relocated control is not accessible as Play: %v", node)
			}
			if !p.Main.AccessibleAction(node.ID, a11y.ActionDefault) {
				t.Fatal("the relocated Play node refused its default action")
			}
			a.PumpOnce()
			if p.Transport.State != players.Playing {
				t.Fatal("the relocated Play control did not start the transport")
			}
			for _, win := range []*ui.Window{p.Main, p.Eq, p.List} {
				img := win.Capture()
				if img == nil || img.Width < 1 || img.Height < 1 || blank(img) {
					t.Errorf("%s painted nothing in the external skin", win.Title())
				}
			}
		})
	}
}

func checkFlexibleExternalSlots(t *testing.T, p *Player, scale float32) {
	t.Helper()
	s := p.strip
	b := s.LocalBounds()
	for _, slot := range []struct {
		name string
		c    widget.Component
		want paint2d.Rect
	}{
		{"skin", s.skinBtn, paint2d.XYWH(b.Max.X-12*scale, 8*scale, 8*scale, 40*scale)},
		{"seek", s.seek, paint2d.XYWH(21*scale, 58*scale, b.Dx()-52*scale, 10*scale)},
	} {
		if d := maxDiff(slot.c.Bounds(), slot.want); d > 0.5 {
			t.Errorf("external %s slot: got %v, want %v", slot.name, slot.c.Bounds(), slot.want)
		}
	}
}

// Copy the application's real artwork and edit only its layout. Installation
// uses the same directory format as a skin somebody supplies independently;
// no player code or toolkit registry is patched for this pack.
func installExternalPanel(t *testing.T, id string) {
	t.Helper()
	sourceRoot := t.TempDir()
	if err := plans.Write(sourceRoot, plans.Winamp()); err != nil {
		t.Fatal(err)
	}
	sourceDir := filepath.Join(sourceRoot, Skin)
	dest := skins.UserSkinDir(id)
	err := fs.WalkDir(os.DirFS(sourceDir), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		to := filepath.Join(dest, filepath.FromSlash(path))
		if entry.IsDir() {
			return os.MkdirAll(to, 0o700)
		}
		data, err := os.ReadFile(filepath.Join(sourceDir, filepath.FromSlash(path)))
		if err != nil {
			return err
		}
		return os.WriteFile(to, data, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(dest, skins.SkinFile)
	raw, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	object := func(parent map[string]any, key string) map[string]any {
		value, ok := parent[key].(map[string]any)
		if !ok {
			t.Fatalf("Classic manifest has no object %q", key)
		}
		return value
	}
	slots := object(object(object(doc, "layouts"), layoutStrip), "slots")
	play, next := object(slots, "play"), object(slots, "next")
	play["at"], next["at"] = next["at"], play["at"]
	skin := object(slots, "skin")
	skin["at"], skin["fromRight"] = []int{4, 8, 8, 40}, true
	seek := object(slots, "seek")
	seek["at"], seek["stretchX"] = []int{21, 58, 31, 10}, true
	doc["label"] = "Third-party layout"
	raw, err = json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	skins.InvalidateSkinCache()
	t.Cleanup(skins.InvalidateSkinCache)
}
