package plans

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	fdlayout "github.com/codemodify/media-player-music/internal/skins/fandom"
	"goframework.io/desktop-ui-skingen"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/style"
	"goframework.io/graphics-paint2d"
)

func TestFandomRetainsSourceAlphaAndIndependentHighResolution(t *testing.T) {
	p := Fandom()
	for _, name := range []string{"face.main", "face.settings", "face.playlist", "app.icon"} {
		t.Run(name, func(t *testing.T) {
			c := xboxTestCell(t, p, name)
			sh := &Sheet{W: c.W, H: c.H, Cells: []Cell{c}}
			one, two := skingen.Render(sh, 1), skingen.Render(sh, 2)
			if two.Width != 2*one.Width || two.Height != 2*one.Height {
				t.Fatal("independent raster has incorrect scale")
			}
			partial, transparent, changed := 0, 0, 0
			for y := 0; y < one.Height; y++ {
				for x := 0; x < one.Width; x++ {
					a := one.NRGBAAt(x, y).A
					if a == 0 {
						transparent++
					}
					if a > 0 && a < 255 {
						partial++
					}
				}
			}
			for y := 0; y < two.Height; y++ {
				for x := 0; x < two.Width; x++ {
					if two.NRGBAAt(x, y) != one.NRGBAAt(x/2, y/2) {
						changed++
					}
				}
			}
			if partial == 0 || transparent == 0 {
				t.Fatalf("source soft silhouette alpha lost: partial=%d transparent=%d", partial, transparent)
			}
			if changed < one.Width {
				t.Fatal("2x is only a duplicated reduced1x raster")
			}
			if name != "app.icon" && one.NRGBAAt(one.Width/2, one.Height/2).A < 250 {
				t.Fatal("the display well became a transparent hole")
			}
		})
	}
}

func TestFandomGeneratedLayoutKeepsChassisAndResponsivePlaylist(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p := Fandom()
	p.Name += "-source-layout-probe"
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
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			lk := style.WithScale(pack.Look(), scale)
			main := paint2d.XYWH(5, 7, 490*scale, 330*scale)
			check := func(layout, name string, box paint2d.Rect, x, y, w, h float32) paint2d.Rect {
				t.Helper()
				got, ok := skins.SkinSlotRect(lk, layout, name, box)
				if !ok {
					t.Fatalf("missing %s", name)
				}
				want := paint2d.XYWH(box.Min.X+x*scale, box.Min.Y+y*scale, w*scale, h*scale)
				if got != want {
					t.Fatalf("%s=%v want%v", name, got, want)
				}
				return got
			}
			check(fdlayout.LayoutMain, "shape.main", main, 132, 0, 338, 330)
			check(fdlayout.LayoutMain, "shape.settings", main, 0, 46, 122, 228)
			check(fdlayout.LayoutMain, "seek", main, 266, 257, 167, 19)
			check(fdlayout.LayoutMain, "volume", main, 282, 280, 137, 19)
			for _, size := range [][2]float32{{190, 246}, {310, 386}} {
				w, h := size[0], size[1]
				box := paint2d.XYWH(11, 13, w*scale, h*scale)
				check(fdlayout.LayoutPlaylist, "shape.playlist", box, 0, 0, w, h)
				check(fdlayout.LayoutPlaylist, "rows", box, 15, 31, w-43, h-60)
				check(fdlayout.LayoutPlaylist, "scroll", box, w-27, 31, 9, h-60)
				check(fdlayout.LayoutPlaylist, "close", box, w-16, 12, 10, 14)
				check(fdlayout.LayoutPlaylist, "status", box, 77, h-18, w-97, 14)
				check(fdlayout.LayoutPlaylist, "resize", box, w-16, h-17, 14, 15)
			}
		})
	}
}
