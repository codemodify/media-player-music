package realplayer

import (
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
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
	b.analyser.Tint = paintengine2d.RGB(.05, .18, .65)
	b.analyser.Pixelated = true
	b.Add(b.analyser)
	return b
}
func (b *canvasPane) sync() {
	b.analyser.SetVisible(b.p.body.visualOn && b.p.Host.SpectrumAvailable())
	b.analyser.Invalidate()
	b.Invalidate()
}
func (b *canvasPane) Arrange(r paintengine2d.Rect) {
	b.SetBounds(r)
	box := b.LocalBounds()
	d := style.Dip(b.Look(), 1)
	b.analyser.Arrange(paintengine2d.XYWH(14*d, 74*d, max(0, box.Dx()-28*d), max(0, box.Dy()-88*d)))
}
func (b *canvasPane) Paint(ctx *paintengine2d.Context) {
	lk, r := b.Look(), b.LocalBounds()
	d := style.Dip(lk, 1)
	style.DrawSkinSprite(lk, ctx, r, "face.canvas", paintengine2d.White)
	style.DrawSkinSprite(lk, ctx, paintengine2d.XYWH(14*d, 14*d, 40*d, 40*d), "app.icon", paintengine2d.White)
	track := b.p.Transport.Track()
	title := track.Title
	if title == "" {
		title = "Music Player"
	}
	if b.p.Transport.List.Len() == 0 {
		title = "Open your music"
	}
	players.DrawTextIn(ctx, players.ScaledFace(lk.BoldFont(), 14*d), paintengine2d.XYWH(66*d, 12*d, max(0, r.Dx()-80*d), 24*d), title, paintengine2d.RGB(0, 0, .5), style.AlignStart)
	artist := track.Artist
	if b.p.Transport.List.Len() == 0 {
		artist = "Open files or drop a folder onto the player"
	}
	players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), 11*d), paintengine2d.XYWH(66*d, 37*d, max(0, r.Dx()-80*d), 19*d), artist, paintengine2d.Black, style.AlignStart)
	if !b.p.Host.SpectrumAvailable() {
		players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), 10*d), paintengine2d.XYWH(14*d, 74*d, max(0, r.Dx()-28*d), max(0, r.Dy()-88*d)), b.p.Host.FeatureTip("Visualizer", false), paintengine2d.RGB(.4, .4, .4), style.AlignCenter)
	}
}
func (b *canvasPane) Describe(n *a11y.Node) { n.Role, n.Name = a11y.RoleGroup, "Now playing" }
func timeFraction(length time.Duration, f float32) time.Duration {
	return time.Duration(float64(length) * float64(f))
}
