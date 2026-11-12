package plans

import (
	"bytes"
	"image"
	"image/png"
	"sync"

	"github.com/codemodify/media-player-music/internal/appicon"
	"github.com/codemodify/media-player-music/internal/skins/panel"
	"goframework.io/graphics-paint2d"
	"golang.org/x/image/draw"
)

var playerLogo = sync.OnceValue(func() image.Image {
	im, err := png.Decode(bytes.NewReader(appicon.LogoPNG()))
	if err != nil {
		panic(err)
	}
	return im
})

func playerLogoImage(size int) *image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, size, size))
	source := playerLogo()
	draw.CatmullRom.Scale(out, out.Bounds(), source, source.Bounds(), draw.Src, nil)
	return out
}

func paintPlayerLogo(ctx *paint2d.Context, r paint2d.Rect) {
	bitmap := paint2d.NewImageFromNRGBA(playerLogoImage(64))
	ctx.DrawImageRectPaint(bitmap, paint2d.XYWH(0, 0, 64, 64), r, paint2d.Paint{Color: paint2d.White, Filter: paint2d.FilterBilinear})
}

// Main's application title has exact case. Satellite title styling remains
// the skin's choice, and any existing main silhouette stays on its variant.
func mainCaptionRole(p *Plan) {
	var text TextRole
	for _, role := range p.Text {
		if role.Name == "caption" {
			text = role
			break
		}
	}
	text.Name, text.Upper = "caption.main", false
	p.Text = append(p.Text, text)
	if p.Window.Variants == nil {
		p.Window.Variants = map[string]*WindowSpec{}
	}
	main := p.Window.Variants[panel.LayoutStrip]
	if main == nil {
		main = &WindowSpec{}
		p.Window.Variants[panel.LayoutStrip] = main
	}
	for _, part := range p.Parts {
		if part.Part == "caption" {
			part.Text = text.Name
			main.Parts = append(main.Parts, part)
			break
		}
	}
}
