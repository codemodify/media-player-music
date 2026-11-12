package console

import (
	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
	"time"
)

type canvasPane struct {
	widget.Base
	p        *Player
	analyser *players.AnalyserView
}

func newCanvasPane(p *Player) *canvasPane {
	b := &canvasPane{p: p}
	b.Init(b)
	b.SetManagesChildren(true)
	b.SetAccessibleName("Now playing")
	b.analyser = players.NewAnalyserView(p.Spectrum)
	b.analyser.Tint = paint2d.RGB(.05, .18, .65)
	b.analyser.Pixelated = true
	b.Add(b.analyser)
	return b
}
func (b *canvasPane) sync() {
	b.analyser.SetVisible(b.p.body.visualOn)
	b.analyser.Invalidate()
	b.Invalidate()
}
func (b *canvasPane) Arrange(r paint2d.Rect) {
	b.SetBounds(r)
	box := b.LocalBounds()
	d := style.Dip(b.Look(), 1)
	b.analyser.Arrange(paint2d.XYWH(14*d, 74*d, max(0, box.Dx()-28*d), max(0, box.Dy()-88*d)))
}
func (b *canvasPane) Paint(ctx *paint2d.Context) {
	lk, r := b.Look(), b.LocalBounds()
	d := style.Dip(lk, 1)
	skins.DrawSkinSprite(lk, ctx, r, "face.canvas", paint2d.White)
	skins.DrawSkinSprite(lk, ctx, paint2d.XYWH(14*d, 14*d, 40*d, 40*d), "app.icon", paint2d.White)
	track := b.p.Transport.Track()
	title := track.Title
	if title == "" {
		title = "Music Player"
	}
	if b.p.Transport.List.Len() == 0 {
		title = "Open your music"
	}
	players.DrawTextIn(ctx, players.ScaledFace(lk.BoldFont(), 14*d), paint2d.XYWH(66*d, 12*d, max(0, r.Dx()-80*d), 24*d), title, paint2d.RGB(0, 0, .5), style.AlignStart)
	artist := track.Artist
	if b.p.Transport.List.Len() == 0 {
		artist = "Open files or drop a folder onto the player"
	}
	players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), 11*d), paint2d.XYWH(66*d, 37*d, max(0, r.Dx()-80*d), 19*d), artist, paint2d.Black, style.AlignStart)
}
func (b *canvasPane) Describe(n *a11y.Node) { n.Role, n.Name = a11y.RoleGroup, "Now playing" }
func timeFraction(length time.Duration, f float32) time.Duration {
	return time.Duration(float64(length) * float64(f))
}
