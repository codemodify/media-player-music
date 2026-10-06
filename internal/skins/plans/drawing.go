// These drawing and sheet-packing helpers support the application's skin
// plans. Rendering and manifest encoding remain in uitoolkit/skingen.
package plans

import "github.com/codemodify/paintengine2d"

func vgrad(ctx *paintengine2d.Context, r paintengine2d.Rect, radius float32, stops ...paintengine2d.GradientStop) {
	ctx.DrawRoundRect(r, radius, radius, paintengine2d.Linear(paintengine2d.LinearGradient{
		Start: paintengine2d.Pt(r.Min.X, r.Min.Y),
		End:   paintengine2d.Pt(r.Min.X, r.Max.Y),
		Stops: stops,
	}))
}

func stop(at float32, c paintengine2d.Color) paintengine2d.GradientStop {
	return paintengine2d.GradientStop{Offset: at, Color: c}
}

// outline strokes a rounded rect on the half-pixel grid, so a 1px line lands
// on one row of pixels instead of two half-lit ones.
func outline(ctx *paintengine2d.Context, r paintengine2d.Rect, radius, width float32, col paintengine2d.Color) {
	ctx.DrawRoundRect(r.Inset(width*0.5), radius, radius, paintengine2d.StrokePaint(col, width))
}

// innerShadow darkens the inside of a sunken face's top edge.
func innerShadow(ctx *paintengine2d.Context, r paintengine2d.Rect, radius, depth float32, col paintengine2d.Color) {
	ctx.Save()
	ctx.ClipRoundRect(r, radius, radius)
	ctx.DrawRect(paintengine2d.XYWH(r.Min.X, r.Min.Y, r.Dx(), depth), paintengine2d.Linear(paintengine2d.LinearGradient{
		Start: paintengine2d.Pt(r.Min.X, r.Min.Y),
		End:   paintengine2d.Pt(r.Min.X, r.Min.Y+depth),
		Stops: []paintengine2d.GradientStop{stop(0, col), stop(1, col.WithAlpha(0))},
	}))
	ctx.Restore()
}

// px fills whole design pixels — the pixel skin's only drawing primitive.
func px(ctx *paintengine2d.Context, x, y, w, h float32, col paintengine2d.Color) {
	ctx.DrawRect(paintengine2d.XYWH(x, y, w, h), paintengine2d.Fill(col))
}

// focusDots draws the dotted focus rectangle on the pixel grid.
func focusDots(ctx *paintengine2d.Context, x0, y0, w, h float32, col paintengine2d.Color) {
	for x := float32(0); x < w; x += 2 {
		px(ctx, x0+x, y0, 1, 1, col)
		px(ctx, x0+x, y0+h-1, 1, 1, col)
	}
	for y := float32(0); y < h; y += 2 {
		px(ctx, x0, y0+y, 1, 1, col)
		px(ctx, x0+w-1, y0+y, 1, 1, col)
	}
}

// ---- laying a sheet out ---------------------------------------------------

// lay packs cells into rows left to right and keeps the sheet's size. The
// gap between cells is not decoration: a bilinear tap reads the texel next
// to the one it wants, so a cell that abuts its neighbour can bleed into it
// under a downscale. The skin engine cuts every sprite into an image of its
// own, which removes the problem at draw time — the gap removes it at
// authoring time as well, and makes the sheet readable when a person opens
// it in an editor.
type lay struct {
	sh   *Sheet
	gap  int
	x, y int
	rowH int
	open bool
}

// row starts a new row of the given height.
func (l *lay) row(h int) {
	if l.open {
		l.y += l.rowH + l.gap
	}
	l.open = true
	l.x, l.rowH = 0, h
}

// cell places one sprite and advances.
func (l *lay) cell(name string, w, h int, slice [4]int, middle string, tint bool, draw func(*paintengine2d.Context, float32, float32)) {
	if !l.open {
		l.row(h)
	}
	if h > l.rowH {
		l.rowH = h
	}
	l.sh.Cells = append(l.sh.Cells, Cell{
		Name: name, X: l.x, Y: l.y, W: w, H: h,
		Slice: slice, Middle: middle, Tint: tint, Draw: draw,
	})
	if l.x+w > l.sh.W {
		l.sh.W = l.x + w
	}
	l.x += w + l.gap
}

// close finishes the sheet, fixing its height.
func (l *lay) close() { l.sh.H = l.y + l.rowH }
