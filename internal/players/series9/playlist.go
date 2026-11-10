package series9

import (
	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

type playlistPane struct {
	*pane
	list *playlistList
}

func newPlaylistPane(p *Player) *playlistPane {
	b := &playlistPane{}
	b.pane = newPane(p, LayoutPlaylist, "Series 9 playlist", b)
	b.list = newPlaylistList(p)
	b.slots.Bind("list", b.list)
	b.Add(b.list)
	b.button("close", "Hide playlist", players.GlyphNone, func() { p.ShowPlaylist(false) })
	b.button("shuffle", "Shuffle", players.GlyphShuffle, func() { p.Host.Command(players.CmdShuffle) }).Toggle = true
	heading := b.button("header", "Playlist file and queue actions", players.GlyphNone, func() {
		c := b.buttons["header"]
		o := widget.DeviceOrigin(c)
		b.contextMenu(c, paint2d.Pt(o.X, o.Y+c.Bounds().Dy()))
	})
	heading.Shaper = nil
	heading.Art = func(ctx *paint2d.Context, r paint2d.Rect, st style.ControlState) bool {
		ctx.DrawRoundRect(r, style.Dip(heading.Look(), 2), style.Dip(heading.Look(), 2), paint2d.Fill(paint2d.RGB(.8, .85, .93)))
		if st.Hovered() {
			ctx.DrawRect(r, paint2d.Fill(paint2d.RGB(.68, .77, .9)))
		}
		text := r
		text.Max.X -= style.Dip(heading.Look(), 14)
		text.Min.X += style.Dip(heading.Look(), 4)
		players.DrawTextIn(ctx, players.ScaledFace(heading.Look().Font(), style.Dip(heading.Look(), 10)), text, "Now Playing", paint2d.RGB(.08, .12, .2), style.AlignStart)
		x, y, d := r.Max.X-style.Dip(heading.Look(), 7), (r.Min.Y+r.Max.Y)/2, style.Dip(heading.Look(), 1)
		var arrow paint2d.Path
		arrow.MoveTo(x-3*d, y-d)
		arrow.LineTo(x+3*d, y-d)
		arrow.LineTo(x, y+2*d)
		arrow.Close()
		ctx.DrawPath(&arrow, paint2d.Fill(paint2d.RGB(.1, .15, .24)))
		return true
	}
	b.button("grip", "Hide playlist", players.GlyphNone, func() { p.ShowPlaylist(false) })
	return b
}
func (b *playlistPane) sync() {
	b.buttons["shuffle"].SetChecked(b.p.Transport.Random)
	b.list.syncQueue()
	b.list.Invalidate()
	b.Invalidate()
}
func (b *playlistPane) contextMenu(from widget.Component, at paint2d.Point) {
	b.p.body.contextMenu(from, at)
}
