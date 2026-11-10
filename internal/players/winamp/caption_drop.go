package winamp

import (
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

// playlistCaption supplies the same file action as the playlist content.
// Its zero-origin scope adds a drop ancestor to the toolkit's caption without
// changing that caption's identity, geometry, controls or gestures.
type playlistCaption struct {
	widget.Base
	header *widgets.HeaderBar
	p      *Player
}

func wrapPlaylistCaption(p *Player, header *widgets.HeaderBar) *playlistCaption {
	c := &playlistCaption{header: header, p: p}
	c.Init(c)
	c.SetHost(p.List)
	c.Add(header)
	return c
}

// The caption remains laid out by the window. This passive scope only supplies
// an unclipped parent rectangle and never moves or lays out that caption.
func (c *playlistCaption) Bounds() paint2d.Rect {
	w, h := c.p.List.PixelSize()
	return paint2d.XYWH(0, 0, float32(w), float32(h))
}
func (c *playlistCaption) LocalBounds() paint2d.Rect { return c.Bounds() }

func (c *playlistCaption) DropTypes() []string { return []string{"text/uri-list"} }
func (c *playlistCaption) Drop(e widget.DropEvent) bool {
	return c.p.Host.AcceptFileDrop(e)
}
func (c *playlistCaption) DropActionFor(offered platform.DragAction) platform.DragAction {
	return offered & platform.DragCopy
}

func (p *Player) bindPlaylistCaption() {
	if p.List == nil || p.List.Closed() {
		return
	}
	header := p.List.Caption()
	if header == nil || p.playlistCaption != nil && p.playlistCaption.header == header && header.Parent() == p.playlistCaption {
		return
	}
	p.playlistCaption = wrapPlaylistCaption(p, header)
}
