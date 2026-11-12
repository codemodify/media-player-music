package cobalt

import (
	"fmt"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

type transportPane struct {
	*pane
	play         *players.GlyphButton
	seek, volume *sourceSlider
}

func newTransportPane(p *Player, role string) *transportPane {
	b := &transportPane{}
	b.pane = newPane(p, role, "Series 9 playback controls", b)
	b.addTransportControls()
	if role == LayoutTransport {
		g := newResizeGrip(p)
		b.slots.Bind("resize", g)
		b.Add(g)
	}
	return b
}
func (b *transportPane) addTransportControls() {
	p := b.p
	b.play = b.button("play", "Play", players.GlyphPlay, func() {
		if p.Transport.State == players.Stopped {
			p.Host.Play()
		} else {
			p.Host.Command(players.CmdPlayPause)
		}
	})
	b.button("stop", "Stop", players.GlyphStop, func() { p.Host.Command(players.CmdStop) })
	b.button("prev", "Previous track", players.GlyphPrev, func() { p.Host.Command(players.CmdPrev) })
	b.button("next", "Next track", players.GlyphNext, func() { p.Host.Command(players.CmdNext) })
	b.button("mute", "Mute", players.GlyphNone, func() { p.Host.Command(players.CmdMute) }).Toggle = true
	b.button("rew", "Seek backward 10 seconds", players.GlyphNone, func() { b.skip(-10 * time.Second) })
	b.button("ffwd", "Seek forward 10 seconds", players.GlyphNone, func() { b.skip(10 * time.Second) })
	b.button("compact", "Switch compact mode", players.GlyphNone, func() { p.SetCompact(!p.Compact()) })
	b.button("settings", "Appearance and settings", players.GlyphSkin, func() {
		c := b.buttons["settings"]
		o := widget.DeviceOrigin(c)
		p.Host.ContextMenu(c, paint2d.Pt(o.X, o.Y+c.Bounds().Dy()))
	})
	b.button("logo", "Playback settings", players.GlyphNone, p.Host.OpenSettings).SetEnabled(p.Host.ShowSettings != nil)
	b.volume = newSourceSlider(false, 0, 100, p.Transport.Volume*100, "Volume", func(v float32) { p.Host.SetVolume(v / 100) }, "volume")
	b.volume.Step, b.volume.Page = 2, 10
	b.volume.Format = func(v float32) string { return fmt.Sprintf("%d%%", int(v+.5)) }
	b.seek = newSourceSlider(false, 0, 1, 0, "Seek", p.Host.Seek, "seek")
	b.seek.Step, b.seek.Page = .02, .1
	b.seek.Format = func(v float32) string {
		return players.Clock(time.Duration(float64(p.Transport.Length()) * float64(v)))
	}
	for _, c := range []struct {
		slot string
		c    widget.Component
	}{{"volume", b.volume}, {"seek", b.seek}} {
		b.slots.Bind(c.slot, c.c)
		b.Add(c.c)
	}
}
func (b *transportPane) skip(offset time.Duration) {
	t := b.p.Transport
	if length := t.Length(); length > 0 {
		pos := min(max(t.Pos+offset, 0), length)
		b.p.Host.Seek(float32(float64(pos) / float64(length)))
	}
}
func (b *transportPane) sync() {
	t := b.p.Transport
	if t.State == players.Playing {
		b.play.SetGlyph(players.GlyphPause, "Pause")
	} else {
		b.play.SetGlyph(players.GlyphPlay, "Play")
	}
	b.buttons["mute"].SetChecked(t.Muted())
	b.buttons["compact"].Tip = "Compact mode (Ctrl+Shift+M)"
	if b.p.Compact() {
		b.buttons["compact"].Tip = "Return to full player (Ctrl+Shift+M)"
	}
	if !b.volume.Dragging() {
		b.volume.Set(t.Volume * 100)
	}
	if !b.seek.Dragging() {
		b.seek.Set(t.Fraction())
	}
	b.seek.SetEnabled(t.Length() > 0)
	b.buttons["rew"].SetEnabled(t.Length() > 0)
	b.buttons["ffwd"].SetEnabled(t.Length() > 0)
	b.Invalidate()
}

type compactPane struct{ *transportPane }

func newCompactPane(p *Player) *compactPane {
	b := &compactPane{transportPane: &transportPane{}}
	b.pane = newPane(p, LayoutCompact, "Series 9 compact player", b)
	b.addTransportControls()
	b.button("open", "Open music files", players.GlyphEject, func() { p.Host.OpenFiles(b.buttons["open"], false) })
	b.button("minimize", "Minimize Music Player", players.GlyphNone, p.Main.Minimize)
	b.button("close", "Close to tray", players.GlyphNone, p.Main.RequestClose)
	return b
}
func (b *compactPane) Paint(ctx *paint2d.Context) {
	eraseFrame(ctx, b.LocalBounds(), b.p)
	b.pane.Paint(ctx)
	lk, box := b.Look(), b.LocalBounds()
	at := func(slot string) paint2d.Rect {
		r, _ := skins.SkinSlotRect(lk, LayoutCompact, slot, box)
		return r
	}
	drawStatus(ctx, lk, at("metadata"), at("time"), b.p)
}

// Both components retain the toolkit's stock measuring and arranging behavior.
var _ widget.Component = (*transportPane)(nil)
var _ widget.Component = (*compactPane)(nil)
