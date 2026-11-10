package plans

import (
	"image"
	"image/color"
	"image/draw"
	"strings"
	"sync"

	"github.com/codemodify/media-player-music/internal/skins/panel"
	"goframework.io/graphics-paint2d"
	xdraw "golang.org/x/image/draw"
)

// polish preserves the complete original 1x sheets and pixel text. Chrome
// gets a separate edge-aware 2x drawing, selected through the public cell
// context's scale. Silver already has its own high-resolution paths.
func polish(p *Plan) {
	if p.Name == "winamp-silver" {
		return
	}
	pixels := &Sheet{Name: "pixels", Pixelated: true}
	var main *paint2d.Image
	var mainCell Cell
	for _, sh := range p.Sheets {
		pixels.W = max(pixels.W, sh.W)
		pixels.H = max(pixels.H, sh.H)
		sh.Pixelated = false
		for i := range sh.Cells {
			cell := sh.Cells[i]
			original := cell.Draw
			one := sync.OnceValue(func() *paint2d.Image {
				im := paint2d.NewImage(cell.W, cell.H)
				ctx := paint2d.NewContext(im)
				ctx.ClipRect(paint2d.XYWH(0, 0, float32(cell.W), float32(cell.H)))
				original(ctx, float32(cell.W), float32(cell.H))
				return im
			})
			if cell.Name == "main.face" {
				main, mainCell = one(), cell
			}
			if nearestArtwork(p.Name, cell.Name) {
				pixels.Cells = append(pixels.Cells, cell)
				continue
			}
			two := sync.OnceValue(func() *paint2d.Image {
				im := scale2x(one())
				polishLogo(im, one(), p.Name, cell.Name)
				return paint2d.NewImageFromNRGBA(im)
			})
			sh.Cells[i].Draw = func(ctx *paint2d.Context, w, h float32) {
				if ctx.Matrix().ApproxScale() <= 1.001 {
					original(ctx, w, h)
					return
				}
				im := two()
				ctx.DrawImageRectPaint(im, paint2d.XYWH(0, 0, float32(im.Width), float32(im.Height)), paint2d.XYWH(0, 0, w, h), paint2d.Paint{Color: paint2d.White, Filter: paint2d.FilterNearest})
			}
		}
	}
	// Later sheets supply the public sprite bindings for these unchanged
	// pixel pieces. The original sheets keep their original pixels at 1x.
	p.Sheets = append(p.Sheets, pixels)
	for i := range p.Layouts {
		layout := &p.Layouts[i]
		if layout.Name != panel.LayoutStrip {
			continue
		}
		var backdrops []SlotSpec
		for _, slot := range layout.Slots {
			switch slot.Name {
			case "display", "title", "rate", "freq":
				backdrops = append(backdrops, SlotSpec{Name: "backdrop." + slot.Name, At: slot.At})
			}
		}
		backdrops = append(backdrops, SlotSpec{Name: "backdrop.labels", At: [4]int{104, 24, 164, 18}})
		for _, slot := range backdrops {
			r := slot.At
			im := classifiedCrop(main, r[0], r[1], r[2], r[3])
			bitmap := paint2d.NewImageFromNRGBA(im)
			pixels.Cells = append(pixels.Cells, Cell{Name: slot.Name, X: mainCell.X + r[0], Y: mainCell.Y + r[1], W: r[2], H: r[3], Draw: func(ctx *paint2d.Context, w, h float32) {
				ctx.DrawImageRectPaint(bitmap, paint2d.XYWH(0, 0, float32(im.Bounds().Dx()), float32(im.Bounds().Dy())), paint2d.XYWH(0, 0, w, h), paint2d.Paint{Color: paint2d.White, Filter: paint2d.FilterNearest})
			}})
			slot.Art = [][2]string{{"normal", slot.Name}}
			layout.Slots = append(layout.Slots, slot)
		}
	}
}

func nearestArtwork(id, name string) bool {
	if name == "eq.face" || strings.HasPrefix(name, "font.") || strings.HasPrefix(name, "info.font.") || strings.HasPrefix(name, "led.") || strings.HasPrefix(name, "lamp.") || strings.HasPrefix(name, "state.") || strings.HasPrefix(name, "mini.") {
		return true
	}
	if strings.HasPrefix(name, "key.") {
		key := strings.Split(strings.TrimPrefix(name, "key."), ".")[0]
		switch key {
		case "prev", "play", "pause", "stop", "next", "eject":
			return false
		default:
			return true // Secondary keys often include tiny printed labels.
		}
	}
	if name == "main.face" && (id == "dosamp" || id == "radar-amp" || id == "ti-82") {
		return true // ASCII frame, wireframe marks or the textured LCD.
	}
	return id == "dosamp" && (strings.HasPrefix(name, "caption.") || strings.HasPrefix(name, "eq.caption") || strings.HasPrefix(name, "list.caption"))
}

// scale2x retains only source colors while refining connected pixel edges.
// The sprite boundary repeats its own nearest pixel, so adjacent atlas cells
// cannot leak into it. Rules: https://www.scale2x.it/algorithm.html .
func scale2x(src image.Image) *image.NRGBA {
	b := src.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx()*2, b.Dy()*2))
	at := func(x, y int) color.NRGBA {
		x, y = max(b.Min.X, min(b.Max.X-1, x)), max(b.Min.Y, min(b.Max.Y-1, y))
		return color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			e := at(x, y)
			a, c, d, f := e, e, e, e
			north, west, east, south := at(x, y-1), at(x-1, y), at(x+1, y), at(x, y+1)
			if north != south && west != east {
				if west == north {
					a = west
				}
				if north == east {
					c = east
				}
				if west == south {
					d = west
				}
				if south == east {
					f = east
				}
			}
			xx, yy := (x-b.Min.X)*2, (y-b.Min.Y)*2
			out.SetNRGBA(xx, yy, a)
			out.SetNRGBA(xx+1, yy, c)
			out.SetNRGBA(xx, yy+1, d)
			out.SetNRGBA(xx+1, yy+1, f)
		}
	}
	return out
}

func polishLogo(two *image.NRGBA, one image.Image, id, name string) {
	var x, y, size, sample int
	if name == "main.face" {
		switch id {
		case "winamp":
			x, y, size = 250, 76, 15
		case "winamp-classified":
			x, y, size = 243, 71, 24
		case "winamp-amber", "calculator":
			x, y, size = 248, 76, 20
		case "windows-98":
			x, y, size = 250, 78, 18
		default:
			return
		}
		sample = x - 2
	} else if name == "caption.normal" || name == "caption.inactive" || id == "winamp-amber" && (name == "eq.caption" || name == "eq.caption.inactive") {
		y, size = 2, 10
		switch id {
		case "winamp":
			x, sample = 5, 16
		case "winamp-classified":
			x, sample = 4, 16
		case "winamp-amber", "windows-98":
			x, sample = 1, 12
		default:
			return
		}
	} else {
		return
	}
	for yy := y; yy < y+size; yy++ {
		bg := color.NRGBAModel.Convert(one.At(sample, yy)).(color.NRGBA)
		for xx := x; xx < x+size; xx++ {
			// Match the original Classified branding crop: its diagonal
			// resize grip reaches the logo's transparent lower-right corner.
			if id == "winamp-classified" && name == "main.face" && xx+yy >= 352 {
				continue
			}
			for dy := 0; dy < 2; dy++ {
				for dx := 0; dx < 2; dx++ {
					two.SetNRGBA(xx*2+dx, yy*2+dy, bg)
				}
			}
		}
	}
	r := image.Rect(x*2, y*2, (x+size)*2, (y+size)*2)
	logo := image.NewRGBA(image.Rect(0, 0, size*2, size*2))
	xdraw.CatmullRom.Scale(logo, logo.Bounds(), playerLogo(), playerLogo().Bounds(), draw.Src, nil)
	draw.Draw(two, r, logo, image.Point{}, draw.Over)
}
