package plans

import (
	"bytes"
	"image"
	"image/png"
	"math"

	fdlayout "github.com/codemodify/media-player-music/internal/skins/fandom"
	fdsource "github.com/codemodify/media-player-music/internal/skins/fandomart"
	"goframework.io/graphics-paint2d"
	"golang.org/x/image/draw"
)

// Fandom adapts the supplied silver reference into an app-owned presentation.
// Its original skin name and archive have not been verified. The preserved
// production raster is independently sampled at 1x and 2x, rather than
// enlarging an already reduced 1x sheet.
func Fandom() *Plan {
	source, err := png.Decode(bytes.NewReader(fdsource.Artwork()))
	if err != nil {
		panic(err)
	}
	sh := &Sheet{Name: "fandom"}
	l := &lay{sh: sh, gap: 2}
	add := func(name string, im image.Image, w, h int, slice [4]int) {
		l.row(h)
		one, two := fandomRaster(im, w, h), fandomRaster(im, w*2, h*2)
		images := []*paint2d.Image{paint2d.NewImageFromNRGBA(one), paint2d.NewImageFromNRGBA(two)}
		l.cell(name, w, h, slice, "", false, func(ctx *paint2d.Context, cw, ch float32) {
			scale := max(1, min(2, int(math.Round(float64(ctx.Matrix().ApproxScale())))))
			ctx.DrawImageRectPaint(images[scale-1], paint2d.XYWH(0, 0, float32(w*scale), float32(h*scale)), paint2d.XYWH(0, 0, cw, ch), paint2d.Paint{Color: paint2d.White, Filter: paint2d.FilterNearest})
		})
	}
	// Crop only the selected chassis. Adjacent unrelated panels and stray
	// pixels outside these crop bounds do not enter the generated artwork.
	add("face.main", fandomCrop(source, image.Rect(441, 39, 1246, 818)), 338, 330, [4]int{})
	add("face.settings", fandomCrop(source, image.Rect(46, 126, 402, 696)), 122, 228, [4]int{})
	add("face.playlist", fandomCrop(source, image.Rect(1277, 112, 1731, 724)), 190, 246, [4]int{29, 15, 27, 13})
	add("app.icon", playerLogo(), 24, 24, [4]int{})
	l.close()
	main := LayoutSpec{Name: fdlayout.LayoutMain, W: 490, H: 330}
	slot := func(name string, at [4]int, art string) {
		s := SlotSpec{Name: name, At: at}
		if art != "" {
			s.Art = [][2]string{{"normal", art}}
		}
		main.Slots = append(main.Slots, s)
	}
	slot("shape.main", [4]int{132, 0, 338, 330}, "face.main")
	slot("shape.settings", [4]int{0, 46, 122, 228}, "face.settings")
	for _, item := range []struct {
		name string
		at   [4]int
	}{
		{"header", [4]int{240, 7, 190, 21}}, {"settings.title", [4]int{10, 54, 82, 16}},
		{"metadata", [4]int{170, 42, 258, 30}}, {"analyser", [4]int{170, 82, 258, 121}}, {"status", [4]int{170, 210, 258, 15}},
		{"status.eq", [4]int{225, 228, 168, 14}}, {"eq.balance.label", [4]int{170, 212, 39, 13}},
		{"play", [4]int{167, 249, 49, 50}}, {"pause", [4]int{167, 249, 49, 50}}, {"stop", [4]int{227, 269, 26, 26}}, {"open", [4]int{227, 249, 26, 16}},
		{"prev", [4]int{266, 279, 10, 21}}, {"next", [4]int{423, 279, 10, 21}},
		{"seek", [4]int{266, 257, 167, 19}}, {"volume", [4]int{282, 280, 137, 19}},
		{"eq", [4]int{399, 228, 15, 14}}, {"pl", [4]int{415, 228, 15, 14}}, {"mute", [4]int{170, 228, 15, 14}}, {"settings", [4]int{187, 228, 15, 14}},
		{"skin", [4]int{255, 306, 101, 23}}, {"logo", [4]int{255, 306, 101, 23}},
		{"minimize", [4]int{434, 8, 16, 13}}, {"close", [4]int{452, 8, 16, 13}}, {"settings.close", [4]int{96, 54, 16, 14}},
		{"eq.preset", [4]int{170, 77, 258, 18}}, {"eq.preamp", [4]int{170, 100, 17, 93}}, {"eq.balance", [4]int{210, 212, 120, 13}},
		{"eq.on", [4]int{339, 211, 27, 14}}, {"eq.auto", [4]int{370, 211, 29, 14}},
	} {
		slot(item.name, item.at, "")
	}
	for i, name := range []string{"general", "playback", "library", "skins", "equalizer", "backends", "nowplaying", "about"} {
		slot("settings."+name, [4]int{11, 79 + i*19, 102, 19}, "")
	}
	for i := 0; i < 10; i++ {
		slot("eq."+string(rune('0'+i)), [4]int{194 + i*23, 100, 17, 93}, "")
	}
	playlist := LayoutSpec{Name: fdlayout.LayoutPlaylist, W: 190, H: 246, Art: "face.playlist", Slots: []SlotSpec{
		{Name: "shape.playlist", At: [4]int{0, 0, 0, 0}, StretchX: true, StretchY: true, Art: [][2]string{{"normal", "face.playlist"}}},
		{Name: "list", At: [4]int{13, 29, 15, 27}, StretchX: true, StretchY: true},
		{Name: "rows", At: [4]int{15, 31, 28, 29}, StretchX: true, StretchY: true},
		{Name: "scroll", At: [4]int{18, 31, 9, 29}, FromRight: true, StretchY: true},
		{Name: "close", At: [4]int{6, 12, 10, 14}, FromRight: true},
		{Name: "open", At: [4]int{13, 4, 16, 14}, FromBottom: true},
		{Name: "shuffle", At: [4]int{34, 4, 16, 14}, FromBottom: true},
		{Name: "repeat", At: [4]int{55, 4, 16, 14}, FromBottom: true},
		{Name: "resize", At: [4]int{2, 2, 14, 15}, FromRight: true, FromBottom: true},
		{Name: "title", At: [4]int{13, 12, 32, 15}, StretchX: true},
		{Name: "status", At: [4]int{77, 4, 20, 14}, StretchX: true, FromBottom: true},
	}}
	return &Plan{Name: "fandom", Label: "Fandom", Year: 2026,
		Lineage: "media-player-music",
		Summary: "Silver tablet, Settings wing and separate playlist, in the project's generated artwork.",
		Family:  "fandom", Base: "win95", Sheets: []*Sheet{sh}, Layouts: []LayoutSpec{main, playlist},
		Text:   []TextRole{{Name: "player.text", Color: "#A5E975", Size: 11}, {Name: "player.row", Color: "#E2EAE2", Size: 10}, {Name: "player.current", Color: "#ACF175"}, {Name: "player.selected", Color: "#243B27"}, {Name: "player.selectedText", Color: "#FFFFFF"}, {Name: "player.info", Color: "#20262A", Size: 10}},
		Colors: map[string]string{"focus": "#75C847"}, Metrics: map[string]float32{"menuItemH": 20}, Window: &WindowSpec{Layout: ":"},
	}
}

func fandomCrop(im image.Image, r image.Rectangle) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(out, out.Bounds(), im, r.Min, draw.Src)
	return out
}

func fandomRaster(im image.Image, w, h int) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(out, out.Bounds(), im, im.Bounds(), draw.Src, nil)
	return out
}
