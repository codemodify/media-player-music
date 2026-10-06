package halflife2

import (
	"fmt"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/layout"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
)

type body struct {
	widget.Base
	p             *Player
	slots         *widget.Slots
	buttons       map[string]*players.GlyphButton
	play, eq, pl  *players.GlyphButton
	seek, volume  *curvedSlider
	innerControls []widget.Component
	shutter       int
	visual        *players.AnalyserView
	glass         *displayGlass
	visualOn      bool
	lastVolume    float32
}

func newBody(p *Player) *body {
	b := &body{p: p, slots: widget.NewSlots(LayoutMain), buttons: make(map[string]*players.GlyphButton), lastVolume: max(p.Transport.Volume, .5)}
	b.Init(b)
	b.SetManagesChildren(true)
	b.SetAccessibleName("Half Life 2 music player")
	b.visual = players.NewAnalyserView(p.Spectrum)
	b.visual.Tint = orange()
	b.visual.Peaks = true
	b.visual.Drag = p.Main
	b.slots.Bind("analyser", b.visual)
	b.Add(b.visual)
	button := func(slot, name string, glyph players.Glyph, action func(), inner bool) *players.GlyphButton {
		c := skinButton(LayoutMain, slot, name, glyph, action)
		b.slots.Bind(slot, c)
		b.buttons[slot] = c
		b.Add(c)
		if inner {
			b.innerControls = append(b.innerControls, c)
		}
		return c
	}
	button("prev", "Previous track", players.GlyphPrev, func() { p.Host.Command(players.CmdPrev) }, true)
	b.play = button("play", "Play", players.GlyphPlay, func() {
		if p.Transport.State == players.Stopped {
			p.Host.Play()
		} else if p.Transport.State != players.Playing {
			p.Host.Command(players.CmdPlayPause)
		}
	}, true)
	button("pause", "Pause or resume", players.GlyphPause, func() {
		if p.Transport.State != players.Stopped {
			p.Host.Command(players.CmdPlayPause)
		}
	}, true)
	button("stop", "Stop", players.GlyphStop, func() { p.Host.Command(players.CmdStop) }, true)
	button("next", "Next track", players.GlyphNext, func() { p.Host.Command(players.CmdNext) }, true)
	button("open", "Open music files", players.GlyphEject, func() { p.Host.OpenFiles(b.buttons["open"], false) }, true)
	menu := func() {
		c := b.buttons["skin"]
		o := widget.DeviceOrigin(c)
		p.Host.ContextMenu(c, paintengine2d.Pt(o.X, o.Y+c.Bounds().Dy()))
	}
	button("logo", "Appearance and settings", players.GlyphSkin, menu, true)
	button("skin", "Appearance and settings", players.GlyphSkin, menu, false)
	button("minimize", "Minimize Music Player", players.GlyphNone, p.Main.Minimize, false)
	button("close", "Close to tray", players.GlyphNone, p.Main.RequestClose, false)
	b.eq = button("eq", "Show equalizer", players.GlyphSliders, func() { p.ShowEqualiser(!p.Host.Model.EqualizerVisible) }, false)
	b.eq.Toggle = true
	b.pl = button("pl", "Show playlist", players.GlyphList, func() { p.ShowPlaylist(!p.Host.Model.PlaylistVisible) }, false)
	b.pl.Toggle = true
	button("vis", "Show or hide spectrum", players.GlyphNone, func() { b.visualOn = !b.visualOn; b.sync(); b.RequestLayout() }, false).Toggle = true
	button("shutter", "Close inner shutter", players.GlyphNone, b.cycleShutter, false)
	button("mute", "Mute", players.GlyphNone, func() {
		if p.Transport.Volume > 0 {
			b.lastVolume = p.Transport.Volume
			p.Host.SetVolume(0)
		} else {
			p.Host.SetVolume(max(b.lastVolume, .01))
		}
	}, false).Toggle = true
	b.seek = newCurvedSlider("seek", 0, 1, 0, "Seek", p.Host.Seek)
	b.seek.Step, b.seek.Page = .02, .1
	b.seek.Format = func(v float32) string {
		return players.Clock(time.Duration(float64(p.Transport.Length())*float64(v))) + " of " + players.Clock(p.Transport.Length())
	}
	b.volume = newCurvedSlider("volume", 0, 100, p.Transport.Volume*100, "Volume", func(v float32) { p.Host.SetVolume(v / 100) })
	b.volume.Step, b.volume.Page = 2, 10
	b.volume.Format = func(v float32) string { return fmt.Sprintf("%d%%", int(v+.5)) }
	for _, item := range []struct {
		slot   string
		f      *curvedSlider
		frames int
	}{{"seek", b.seek, 33}, {"volume", b.volume, 27}} {
		item := item
		item.f.Travel = .001
		item.f.Painter = func(ctx *paintengine2d.Context, _ paintengine2d.Rect, t float32, _ style.ControlState) bool {
			return style.DrawSkinSprite(item.f.Look(), ctx, widget.SlotArtRect(item.f, LayoutMain, item.slot), fmt.Sprintf("%s.%d", item.slot, int(t*float32(item.frames-1)+.5)), paintengine2d.Color{})
		}
		b.slots.Bind(item.slot, item.f)
		b.Add(item.f)
		b.innerControls = append(b.innerControls, item.f)
	}
	// Source zIndex places the translucent glass above the transport,
	// logo and curved sliders. It paints once and passes all input through.
	b.glass = &displayGlass{}
	b.glass.Init(b.glass)
	b.glass.SetTransparent(true)
	b.slots.Bind("glass.foreground", b.glass)
	b.Add(b.glass)
	for _, slot := range []string{"skin", "minimize", "close", "eq", "pl", "vis", "shutter", "mute"} {
		c := b.buttons[slot]
		b.Remove(c)
		b.Add(c)
	}
	return b
}

type displayGlass struct{ widget.Base }

func (g *displayGlass) HitTest(paintengine2d.Point) widget.Component { return nil }
func (g *displayGlass) Paint(ctx *paintengine2d.Context) {
	style.DrawSkinSprite(g.Look(), ctx, g.LocalBounds(), "glass.foreground", paintengine2d.Color{})
}
func (g *displayGlass) Describe(n *a11y.Node) { n.Role, n.Name = a11y.RoleImage, "Display glass" }
func skinButton(role, slot, name string, glyph players.Glyph, action func()) *players.GlyphButton {
	c := players.NewGlyphButton(glyph, name, action)
	c.Art = func(ctx *paintengine2d.Context, _ paintengine2d.Rect, st style.ControlState) bool {
		return style.DrawSkinSlot(c.Look(), ctx, widget.SlotArtRect(c, role, slot), role, slot, st)
	}
	c.Shaper = func(lk style.LookAndFeel, size paintengine2d.Point) *style.Silhouette {
		return style.SkinSlotShape(lk, size, widget.SlotArtRect(c, role, slot), role, slot)
	}
	return c
}
func orange() paintengine2d.Color { return paintengine2d.RGB(1, .48, .13) }
func (b *body) cycleShutter() {
	b.shutter = (b.shutter + 1) % 3
	b.p.setShape(b.p.Main, LayoutMain)
	b.p.configureUtilityWindows()
	b.sync()
	if c := b.p.Main.Focus(); c != nil && !c.Visible() {
		b.p.Main.RequestFocus(b.buttons["shutter"])
	}
	b.RequestLayout()
}
func (b *body) sync() {
	t := b.p.Transport
	open := b.shutter == 0
	for _, c := range b.innerControls {
		c.SetVisible(open)
	}
	// Pause overlays the same play sector in the original artwork.
	b.play.SetVisible(open && t.State != players.Playing)
	b.buttons["pause"].SetVisible(open && t.State == players.Playing)
	b.buttons["logo"].SetVisible(open && !b.visualOn)
	if focused := b.p.Main.Focus(); focused != nil && !focused.Visible() {
		if open {
			b.p.Main.RequestFocus(b.transportButton())
		} else {
			b.p.Main.RequestFocus(b.buttons["shutter"])
		}
	}
	b.eq.SetChecked(b.p.Host.Model.EqualizerVisible)
	b.pl.SetChecked(b.p.Host.Model.PlaylistVisible)
	b.buttons["mute"].SetChecked(t.Volume == 0)
	b.buttons["vis"].SetChecked(b.visualOn)
	label := []string{"Close inner shutter", "Close outer shutter", "Open shutters"}[b.shutter]
	b.buttons["shutter"].SetGlyph(players.GlyphNone, label)
	b.visual.SetVisible(open && b.visualOn)
	b.glass.SetVisible(open)
	b.visual.SetAccessibleDescription(b.p.Host.FeatureTip("Visualizer", b.p.Host.SpectrumAvailable()))
	b.visual.Invalidate()
	if !b.seek.Dragging() {
		b.seek.Set(t.Fraction())
	}
	if !b.volume.Dragging() {
		b.volume.Set(t.Volume * 100)
	}
	b.Invalidate()
}
func (b *body) transportButton() *players.GlyphButton {
	if b.p.Transport.State == players.Playing {
		return b.buttons["pause"]
	}
	return b.play
}
func (b *body) Measure(c layout.Constraints) paintengine2d.Point {
	sz, _ := b.slots.Measure(b.Look(), c)
	return sz
}
func (b *body) Arrange(r paintengine2d.Rect) {
	b.SetBounds(r)
	b.slots.Arrange(b.Look(), b.LocalBounds())
}
func (b *body) Paint(ctx *paintengine2d.Context) {
	lk, box := b.Look(), b.LocalBounds()
	state := []string{"open", "inner", "closed"}[b.shutter]
	if state == "open" {
		state += ".base"
	}
	style.DrawSkinSprite(lk, ctx, box, "face.main."+state, paintengine2d.Color{})
	if b.shutter != 0 {
		return
	}
	at := func(slot string) paintengine2d.Rect { r, _ := style.SkinSlotRect(lk, LayoutMain, slot, box); return r }
	title := b.p.Transport.Track().Label()
	if title == "" {
		title = "Music Player"
	}
	if b.p.Host.Model.Error != "" {
		title = b.p.Host.StatusText()
	}
	players.DrawTextIn(ctx, players.ScaledFace(lk.Font(), style.Dip(lk, 9)), at("title"), title, orange(), style.AlignCenter)
	f := players.ScaledFace(lk.Font(), style.Dip(lk, 8))
	players.DrawTextIn(ctx, f, at("time"), players.Clock(b.p.Transport.Pos), orange(), style.AlignCenter)
	players.DrawTextIn(ctx, f, at("duration"), players.Clock(b.p.Transport.Length()), orange(), style.AlignCenter)
}
func (b *body) MousePress(e widget.MouseEvent) bool {
	if e.Button == platform.ButtonRight {
		b.p.Host.ContextMenu(b, e.Pos.Add(widget.DeviceOrigin(b)))
		return true
	}
	if e.Button == platform.ButtonLeft {
		b.p.restoredPlacement = nil
		b.p.Main.StartMove()
		return true
	}
	return false
}
func (b *body) KeyPress(e widget.KeyEvent) bool { return b.p.keys(b.p.Main, e) }
func (b *body) Describe(n *a11y.Node)           { n.Role, n.Name = a11y.RoleGroup, "Half Life 2 music player" }
func (b *body) DropTypes() []string             { return []string{"text/uri-list"} }
func (b *body) DropActionFor(offered platform.DragAction) platform.DragAction {
	return offered & platform.DragCopy
}
func (b *body) Drop(e widget.DropEvent) bool { return b.p.Host.AcceptFileDrop(e) }
