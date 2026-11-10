package fancy

import (
	"fmt"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/layout"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

type mainPane struct {
	widget.Base
	p                     *Player
	compact               bool
	role                  string
	slots                 *skins.Slots
	buttons               map[string]*players.GlyphButton
	play                  *players.GlyphButton
	seek, volume, balance *fancySlider
	analyser              *players.AnalyserView
}

func newMainPane(p *Player, compact bool) *mainPane {
	role := LayoutMain
	if compact {
		role = LayoutCompact
	}
	b := &mainPane{p: p, compact: compact, role: role, slots: skins.NewSlots(role), buttons: make(map[string]*players.GlyphButton)}
	b.Init(b)
	b.SetManagesChildren(true)
	b.SetAccessibleName("Fancy music player")
	add := func(slot, label, text string, glyph players.Glyph, action func()) *players.GlyphButton {
		c := button(role, slot, label, text, glyph, action)
		b.buttons[slot] = c
		b.slots.Bind(slot, c)
		b.Add(c)
		return c
	}
	b.play = add("play", "Play", "", players.GlyphPlay, func() {
		if p.Transport.State == players.Stopped {
			p.Host.Play()
		} else {
			p.Host.Command(players.CmdPlayPause)
		}
	})
	add("prev", "Previous track", "", players.GlyphPrev, func() { p.Host.Command(players.CmdPrev) })
	add("next", "Next track", "", players.GlyphNext, func() { p.Host.Command(players.CmdNext) })
	add("stop", "Stop", "", players.GlyphStop, func() { p.Host.Command(players.CmdStop) })
	add("close", "Close to tray", "", players.GlyphNone, p.Main.RequestClose)
	add("skin", "Appearance and settings", "", players.GlyphSkin, func() {
		c := b.buttons["skin"]
		p.Host.ContextMenu(c, widget.DeviceOrigin(c).Add(paint2d.Pt(0, c.Bounds().Dy())))
	})
	b.seek = slider(false, false, 0, 1, 0, "Seek", p.Host.Seek)
	b.seek.Step, b.seek.Page = .02, .1
	b.seek.Format = func(v float32) string {
		return players.Clock(time.Duration(float64(p.Transport.Length())*float64(v))) + " of " + players.Clock(p.Transport.Length())
	}
	b.slots.Bind("seek", b.seek)
	b.Add(b.seek)
	if compact {
		add("restore", "Restore full player", "", players.GlyphNone, func() { p.SetCompact(false) })
		return b
	}
	add("back", "Seek backward", "", players.GlyphPrev, func() { p.Host.Command(players.CmdSeekBack) })
	add("forward", "Seek forward", "", players.GlyphNext, func() { p.Host.Command(players.CmdSeekForward) })
	add("open", "Open music files", "", players.GlyphEject, func() { p.Host.OpenFiles(b.buttons["open"], false) })
	add("compact", "Compact player", "", players.GlyphNone, func() { p.SetCompact(true) })
	add("minimize", "Minimize Music Player", "", players.GlyphNone, p.Main.Minimize)
	add("eq", "Show equalizer", "EQ", players.GlyphNone, func() { p.ShowEqualiser(!p.Host.Model.EqualizerVisible) }).Toggle = true
	add("pl", "Show playlist", "PL", players.GlyphNone, func() { p.ShowPlaylist(!p.Host.Model.PlaylistVisible) }).Toggle = true
	add("shuffle", "Shuffle", "RND", players.GlyphNone, func() { p.Host.Command(players.CmdShuffle) }).Toggle = true
	add("repeat", "Repeat", "REP", players.GlyphNone, func() { p.Host.Command(players.CmdRepeat) }).Toggle = true
	add("mute", "Mute", "MUTE", players.GlyphNone, func() { p.Host.Command(players.CmdMute) }).Toggle = true
	add("settings", "Audio backend settings", "", players.GlyphSliders, p.Host.OpenSettings)
	b.volume = slider(false, true, 0, 100, p.Transport.Volume*100, "Volume", func(v float32) { p.Host.SetVolume(v / 100) })
	b.volume.Step, b.volume.Page = 2, 10
	b.volume.Format = func(v float32) string { return fmt.Sprintf("%d%%", int(v+.5)) }
	b.slots.Bind("volume", b.volume)
	b.Add(b.volume)
	b.balance = slider(false, true, -1, 1, p.Transport.Pan, "Balance", p.Host.SetPan)
	b.balance.Step, b.balance.Page = .1, .25
	b.balance.Format = func(v float32) string {
		if v == 0 {
			return "Center"
		}
		if v < 0 {
			return fmt.Sprintf("%d%% left", int(-v*100+.5))
		}
		return fmt.Sprintf("%d%% right", int(v*100+.5))
	}
	b.slots.Bind("balance", b.balance)
	b.Add(b.balance)
	b.analyser = players.NewAnalyserView(p.Spectrum)
	b.analyser.Tint = cyan()
	b.analyser.Peaks = false
	b.analyser.MinBarW = 1
	b.analyser.Gap = 1
	b.analyser.Drag = p.Main
	b.slots.Bind("analyser", b.analyser)
	b.Add(b.analyser)
	return b
}
func (b *mainPane) sync() {
	t := b.p.Transport
	if t.State == players.Playing {
		b.play.SetGlyph(players.GlyphPause, "Pause")
	} else {
		b.play.SetGlyph(players.GlyphPlay, "Play")
	}
	if !b.seek.Dragging() {
		b.seek.Set(t.Fraction())
	}
	b.seek.SetEnabled(t.List.Len() > 0 && t.Length() > 0)
	if !b.compact {
		b.volume.Set(t.Volume * 100)
		b.balance.Set(t.Pan)
		b.balance.SetEnabled(b.p.Host.BalanceAvailable())
		b.balance.Tip = b.p.Host.FeatureTip("Balance", b.p.Host.BalanceAvailable())
		b.buttons["eq"].SetChecked(b.p.Host.Model.EqualizerVisible)
		b.buttons["pl"].SetChecked(b.p.Host.Model.PlaylistVisible)
		b.buttons["shuffle"].SetChecked(t.Random)
		b.buttons["repeat"].SetChecked(t.Repeat != players.RepeatOff)
		b.buttons["repeat"].Tip = "Repeat: " + t.Repeat.String()
		b.buttons["mute"].SetChecked(t.Volume == 0)
		b.analyser.SetVisible(b.p.Host.SpectrumAvailable())
	}
	b.Invalidate()
}
func (b *mainPane) Measure(c layout.Constraints) paint2d.Point {
	size, _ := b.slots.Measure(b.Look(), c)
	return size
}
func (b *mainPane) Arrange(r paint2d.Rect) {
	b.SetBounds(r)
	b.slots.Arrange(b.Look(), b.LocalBounds())
}
func (b *mainPane) Paint(ctx *paint2d.Context) {
	lk, box := b.Look(), b.LocalBounds()
	paintLayout(ctx, lk, box, b.role, b.p.Main)
	track := b.p.Transport.Track().Label()
	if track == "" {
		track = "Open music files"
	}
	textSlot(ctx, lk, box, b.role, "track", track, 8, cyan(), style.AlignCenter)
	if !b.compact {
		textSlot(ctx, lk, box, b.role, "header", "Music Player", 8, ink(), style.AlignStart)
		status := b.p.Transport.State.String()
		if b.p.Host.Model.Error != "" {
			status = b.p.Host.Model.Error
		} else if b.p.Transport.List.Len() == 0 {
			status = "Open music files"
		}
		textSlot(ctx, lk, box, b.role, "status", status, 8, ink(), style.AlignCenter)
		if r, ok := skins.SkinSlotRect(lk, b.role, "logo", box); ok {
			skins.DrawSkinSlot(lk, ctx, r, b.role, "logo", 0)
		}
	}
	if r, ok := skins.SkinSlotRect(lk, b.role, "time", box); ok {
		size := float32(27)
		if b.compact {
			size = 9
		}
		players.DrawTextIn(ctx, players.ScaledFace(lk.MonoFont(), style.Dip(lk, size)), r, players.Clock(b.p.Transport.Pos), paint2d.RGB(.867, .914, .922), style.AlignCenter)
	}
}
func (b *mainPane) MousePress(e widget.MouseEvent) bool {
	if e.Button == platform.ButtonRight {
		b.p.Host.ContextMenu(b, e.Pos.Add(widget.DeviceOrigin(b)))
		return true
	}
	if e.Button == platform.ButtonLeft {
		b.p.restoredPlacement = nil
		return b.p.Main.StartMove()
	}
	return false
}
func (b *mainPane) CaptionAt(paint2d.Point) bool    { return true }
func (b *mainPane) KeyPress(e widget.KeyEvent) bool { return b.p.keys(b.p.Main, e) }
func (b *mainPane) Describe(n *a11y.Node)           { n.Role, n.Name = a11y.RoleGroup, "Fancy music player" }
