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

// brandMainBitmap changes only the app logo in imported artwork. Other skin
// emblems (Sony, Radar and Quake) belong to the skin and remain original.
func brandMainBitmap(main *image.NRGBA, id string) {
	var box, logo image.Rectangle
	var sampleX int
	switch id {
	case "calculator":
		box, logo, sampleX = image.Rect(247, 87, 271, 112), image.Rect(249, 90, 269, 110), 240
	case "winamp-amber":
		box, logo, sampleX = image.Rect(248, 89, 271, 112), image.Rect(249, 90, 269, 110), 240
	case "winamp-classified":
		box, logo, sampleX = image.Rect(241, 83, 270, 112), image.Rect(244, 85, 268, 109), 239
	case "windows-98":
		box, logo, sampleX = image.Rect(250, 91, 271, 112), image.Rect(251, 92, 269, 110), 250
	default:
		return
	}
	for y := box.Min.Y; y < box.Max.Y; y++ {
		background := main.At(sampleX, y)
		if id == "windows-98" {
			background = main.At(250, 85)
		}
		for x := box.Min.X; x < box.Max.X; x++ {
			// The Classified resize grip is separate from its logo.
			if id == "winamp-classified" && x+y >= 367 {
				continue
			}
			main.Set(x, y, background)
		}
	}
	stamp := playerLogoImage(logo.Dx())
	draw.Draw(main, logo, stamp, image.Point{}, draw.Over)
}

// brandCaptionBitmap replaces an imported app icon with the original speaker
// logo. Preserve the outer frame and recreate the clean strip behind the mark.
func brandCaptionBitmap(band *image.NRGBA, left, right, sample int) {
	for y := 1; y < band.Bounds().Dy()-1; y++ {
		background := band.At(sample, y)
		for x := left; x < right; x++ {
			band.Set(x, y, background)
		}
	}
	size := min(10, band.Bounds().Dy()-4)
	r := image.Rect(left+(right-left-size)/2, (band.Bounds().Dy()-size)/2, 0, 0)
	r.Max = r.Min.Add(image.Pt(size, size))
	draw.Draw(band, r, playerLogoImage(size), image.Point{}, draw.Over)
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
