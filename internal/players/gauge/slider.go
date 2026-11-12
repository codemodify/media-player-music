package gauge

import (
	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

// The source position images encode the value as grayscale along a curved
// strip. Transform only pointer coordinates into the toolkit slider's axis;
// its ordinary dragging, user-input notification, keys and a11y stay intact.
type curvedSlider struct {
	*players.Fader
	slot    string
	mapping *paint2d.Image
	mapLook style.LookAndFeel
}

func newCurvedSlider(slot string, min, max, value float32, label string, on func(float32)) *curvedSlider {
	f := &curvedSlider{Fader: players.NewBar(min, max, value, label, on), slot: slot}
	f.Init(f)
	return f
}
func (f *curvedSlider) ArtShape(lk style.LookAndFeel, size paint2d.Point) *style.Silhouette {
	return skins.SkinSpriteShape(lk, size, skins.SlotArtRect(f, LayoutMain, f.slot), f.slot+".shape")
}
func (f *curvedSlider) updateMap() {
	r := f.LocalBounds()
	w, h := int(r.Dx()+.5), int(r.Dy()+.5)
	if w < 1 || h < 1 {
		return
	}
	lk := f.Look()
	if f.mapping != nil && f.mapping.Width == w && f.mapping.Height == h && f.mapLook == lk {
		return
	}
	im := paint2d.NewImage(w, h)
	ctx := paint2d.NewContext(im)
	ctx.Clear(paint2d.Transparent)
	if !skins.DrawSkinSprite(lk, ctx, skins.SlotArtRect(f, LayoutMain, f.slot), f.slot+".map", paint2d.Color{}) {
		f.mapping = nil
		return
	}
	f.mapping, f.mapLook = im, lk
}
func (f *curvedSlider) pointer(e widget.MouseEvent) widget.MouseEvent {
	f.updateMap()
	im := f.mapping
	if im == nil {
		return e
	}
	x, y := int(e.Pos.X), int(e.Pos.Y)
	best, value := float32(1e30), float32(0)
	if x >= 0 && y >= 0 && x < im.Width && y < im.Height && im.Pix[(y*im.Width+x)*4+3] >= 128 {
		at := (y*im.Width + x) * 4
		// The paint engine stores premultiplied RGBA. Undo coverage at
		// fractional edges so a partially covered white endpoint stays 100%.
		value = float32(im.Pix[at]) / float32(im.Pix[at+3])
	} else {
		// A captured drag can leave the curved strip. Project it to the
		// nearest mapped pixel, including the two endpoint stops.
		for py := 0; py < im.Height; py++ {
			for px := 0; px < im.Width; px++ {
				at := (py*im.Width + px) * 4
				if im.Pix[at+3] < 128 {
					continue
				}
				dx, dy := float32(px)-e.Pos.X, float32(py)-e.Pos.Y
				distance := dx*dx + dy*dy
				if distance < best {
					best = distance
					value = float32(im.Pix[at]) / float32(im.Pix[at+3])
				}
			}
		}
	}
	travel := style.Dip(f.Look(), f.Travel)
	e.Pos.X = travel + value*(f.LocalBounds().Dx()-2*travel)
	return e
}
func (f *curvedSlider) MousePress(e widget.MouseEvent) bool {
	if e.Button != platform.ButtonLeft {
		return false
	}
	return f.Fader.MousePress(f.pointer(e))
}
func (f *curvedSlider) MouseMove(e widget.MouseEvent) bool { return f.Fader.MouseMove(f.pointer(e)) }
func (f *curvedSlider) MouseRelease(e widget.MouseEvent) bool {
	if e.Button != platform.ButtonLeft {
		return false
	}
	return f.Fader.MouseRelease(e)
}
