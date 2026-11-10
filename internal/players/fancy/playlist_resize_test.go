package fancy

import (
	"fmt"
	"math"
	"testing"

	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

func TestResizedPlaylistKeepsIndependentChromeSizeAndVisibleResizeGrip(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			initial := map[string]paint2d.Rect{}
			for _, size := range [][2]int{{ListW, ListH}, {620, 340}} {
				p.List().SetSize(size[0], size[1])
				a.PumpOnce()
				p.Advance(0)
				a.PumpOnce()
				pw, ph := p.List().PixelSize()
				box := paint2d.XYWH(0, 0, float32(pw), float32(ph))
				lk := p.List().Look()
				capture := p.List().CaptureSurface()
				for _, slot := range []string{"logo", "open", "save", "clear", "shuffle", "repeat"} {
					r, ok := skins.SkinSlotRect(lk, LayoutPlaylist, slot, box)
					if !ok {
						t.Fatalf("missing independent %s art", slot)
					}
					if before, ok := initial[slot]; ok {
						if r.Dx() != before.Dx() || r.Dy() != before.Dy() {
							t.Fatalf("%s stretched with playlist: %v -> %v", slot, before, r)
						}
						if slot == "logo" || slot == "shuffle" || slot == "repeat" {
							if math.Abs(float64(r.Min.X-before.Min.X-200*scale)) > .1 || r.Min.Y != before.Min.Y {
								t.Fatal("sidebar chrome lost its right anchor")
							}
						}
					} else {
						initial[slot] = r
					}
					// Verify actual captured chrome, not just authored geometry: its
					// opaque sprite pixels must be drawn in the independently sized box.
					raw := paint2d.NewImage(pw, ph)
					ctx := paint2d.NewContext(raw)
					ctx.Clear(paint2d.Transparent)
					artBox := r
					if c := p.listPane.buttons[slot]; c != nil {
						// Component bounds round to device pixels; the button
						// paints into that actual allocated control box.
						artBox = widget.DeviceBounds(c)
					}
					if !skins.DrawSkinSlot(lk, ctx, artBox, LayoutPlaylist, slot, 0) {
						t.Fatalf("%s has no resting art", slot)
					}
					checked := 0
					for y := int(r.Min.Y); y < int(r.Max.Y); y++ {
						for x := int(r.Min.X); x < int(r.Max.X); x++ {
							if slot != "logo" && float32(y) > r.Min.Y+style.Dip(lk, 3) && float32(y) < r.Max.Y-style.Dip(lk, 3) {
								continue
							}
							at := (y*pw + x) * 4
							if raw.Pix[at+3] != 255 {
								continue
							}
							got := capture.NRGBAAt(x, y)
							for channel, v := range []uint8{got.R, got.G, got.B} {
								if math.Abs(float64(int(v)-int(raw.Pix[at+channel]))) > 2 {
									t.Fatalf("%s resting art absent/stretched at%d,%d", slot, x, y)
								}
							}
							checked++
						}
					}
					if checked < 10 {
						t.Fatalf("%s capture check has no opaque art", slot)
					}
				}
				var grip *resizeGrip
				for _, c := range p.listPane.Children() {
					if g, ok := c.(*resizeGrip); ok {
						grip = g
						break
					}
				}
				if grip == nil {
					t.Fatal("missing resize grip")
				}
				r := widget.DeviceBounds(grip)
				raw := paint2d.NewImage(int(r.Dx()+1), int(r.Dy()+1))
				ctx := paint2d.NewContext(raw)
				ctx.Clear(paint2d.Transparent)
				grip.Paint(ctx)
				visible := 0
				for y := 0; y < raw.Height; y++ {
					for x := 0; x < raw.Width; x++ {
						if raw.Pix[(y*raw.Width+x)*4+3] > 128 {
							at := r.Min.Add(paint2d.Pt(float32(x), float32(y)))
							if p.List().Shape().Raster(pw, ph).Contains(int(at.X), int(at.Y)) {
								visible++
							}
						}
					}
				}
				if visible < 5 {
					t.Fatal("grip strokes fall outside visible window mask")
				}
				off := p.List().Surface().(*platform.Offscreen)
				before := len(off.FrameCalls().Resizes)
				click(p.List(), r.Center(), platform.ButtonLeft)
				a.PumpOnce()
				calls := off.FrameCalls().Resizes
				if len(calls) != before+1 || calls[len(calls)-1] != platform.EdgeBottom|platform.EdgeRight {
					t.Fatal("visible grip did not start bottom-right native resize")
				}
			}
		})
	}
}
