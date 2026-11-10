package plans

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	fl "github.com/codemodify/media-player-music/internal/skins/fancy"
	"goframework.io/desktop-ui-skingen"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/style"
	"goframework.io/graphics-paint2d"
)

// Render the whole atlas, rather than individual cells: a cell painter must
// not clear the artwork already rendered for the other three panels.
func TestFancyAtlasKeepsShapedPanelsAndInteractiveControlCenters(t *testing.T) {
	p := Fancy()
	for _, scale := range []int{1, 2} {
		t.Run(fmt.Sprintf("%dx", scale), func(t *testing.T) {
			atlas := skingen.Render(p.Sheets[0], float32(scale))
			for _, layout := range p.Layouts {
				name := "face." + strings.TrimPrefix(layout.Name, "fancy.")
				var cell Cell
				for _, c := range p.Sheets[0].Cells {
					if c.Name == name {
						cell = c
					}
				}
				if cell.Draw == nil {
					t.Fatalf("missing panel artwork %s", name)
				}
				alpha := func(x, y int) uint8 {
					return atlas.NRGBAAt((cell.X+x)*scale, (cell.Y+y)*scale).A
				}
				for _, corner := range []image.Point{{0, 0}, {cell.W - 1, 0}, {0, cell.H - 1}, {cell.W - 1, cell.H - 1}} {
					if alpha(corner.X, corner.Y) != 0 {
						t.Fatalf("%s lost transparent corner %v", name, corner)
					}
				}
				for _, slot := range layout.Slots {
					if strings.HasPrefix(slot.Name, "shape.") {
						continue
					}
					x, y, w, h := slot.At[0], slot.At[1], slot.At[2], slot.At[3]
					if slot.StretchX {
						w = layout.W - x - w
					} else if slot.FromRight {
						x = layout.W - x - w
					}
					if slot.StretchY {
						h = layout.H - y - h
					} else if slot.FromBottom {
						y = layout.H - y - h
					}
					if alpha(x+w/2, y+h/2) < 250 {
						t.Fatalf("%s/%s falls outside the native window silhouette", layout.Name, slot.Name)
					}
				}
			}
		})
	}
}

func TestFancyPlaylistResizePreservesViewportAndPanelAnchors(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p := Fancy()
	p.Name += "-layout-probe"
	dir := t.TempDir()
	if err := Write(dir, p); err != nil {
		t.Fatal(err)
	}
	if err := skins.RegisterSkinFS(os.DirFS(filepath.Join(dir, p.Name)), p.Name); err != nil {
		t.Fatal(err)
	}
	pack, ok := style.LoadTheme(p.Name)
	if !ok {
		t.Fatal("generated theme unavailable")
	}
	for _, scale := range []float32{1, 1.75, 2} {
		lk := style.WithScale(pack.Look(), scale)
		for _, size := range [][2]float32{{420, 124}, {420, 144}, {620, 340}} {
			w, h := size[0], size[1]
			box := paint2d.XYWH(7, 9, w*scale, h*scale)
			check := func(name string, x, y, sw, sh float32) {
				t.Helper()
				got, ok := skins.SkinSlotRect(lk, fl.LayoutPlaylist, name, box)
				want := paint2d.XYWH(7+x*scale, 9+y*scale, sw*scale, sh*scale)
				if !ok || got != want {
					t.Fatalf("%gx %gx%g %s=%v want %v", scale, w, h, name, got, want)
				}
			}
			check("shape.playlist", 0, 0, w, h)
			check("rows", 45, 32, w-147, h-61)
			check("scroll", w-100, 32, 7, h-61)
			check("close", w-27, 7, 18, 17)
			check("shuffle", w-85, 35, 60, 18)
			check("repeat", w-85, 63, 60, 18)
			check("logo", w-72, 85, 24, 24)
			check("resize", w-29, h-29, 13, 13)
			check("status", 126, h-21, w-240, 14)
		}
	}
}
