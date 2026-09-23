package minim

import (
	"testing"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/playertest"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/app"
	"github.com/codemodify/uitoolkit/skingen/panel"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
)

// The panel faces on the skin's own layouts: the controls are where the
// skin's slots are, in the order the player added them; the silver
// equaliser wears its own caption; the round keys take the pointer only on
// their ink and brighten under it; the titles are the player's; and a pixel
// panel at a fractional scale is whole pixels of its own art.

// Every panel control is in its slot, at every scale, within the pixel its
// whole-pixel box rounds it to — and the tab order is the one the player
// added the controls in, the same in every face.
func TestThePanelsControlsAreWhereTheSkinSays(t *testing.T) {
	var widgetRing []string
	for _, id := range []string{Skin, SkinClassic, SkinSilver} {
		for _, scale := range scales {
			a, p := open(t, id, scale)
			s := p.strip
			if id != Skin {
				for c, slot := range map[widget.Component]string{
					s.readout: "display", s.seek: "seek", s.volume: "volume", s.play: "play",
					s.shuffle: "shuffle", s.skinBtn: "skin",
				} {
					want, ok := style.SkinSlotRect(s.Look(), layoutStrip, slot, s.LocalBounds())
					if !ok {
						t.Fatalf("%s@%gx: no %s slot", id, scale, slot)
					}
					got := c.Bounds()
					if d := maxDiff(got, want); d > 0.5 {
						t.Errorf("%s@%gx: %s at %v, its slot is %v", id, scale, slot, got, want)
					}
				}
				for i, f := range p.eqPane.bands {
					want, _ := style.SkinSlotRect(f.Look(), layoutEqualiser, "band."+string(rune('0'+i)), p.eqPane.LocalBounds())
					if d := maxDiff(f.Bounds(), want); d > 0.5 {
						t.Errorf("%s@%gx: band %d at %v, its slot is %v", id, scale, i, f.Bounds(), want)
					}
				}
			}
			if scale != 1 {
				continue
			}
			ring := playertest.RingNames(p.Main, playertest.FocusRing(t, a, p.Main))
			if id == Skin {
				widgetRing = ring
				continue
			}
			// The panels have pause, eject and balance where the widget
			// face has mute; otherwise the order is the same order.
			if got, want := without(ring, "Pause", "Eject: stop and go back to the first track", "Balance"),
				without(widgetRing, "Mute", "Unmute"); !sameOrder(skinKeyless(got), skinKeyless(want)) {
				t.Errorf("%s: the tab order is\n %v\nwhere the widget face's is\n %v", id, got, want)
			}
		}
	}
}

// skinKeyless is a ring's names with the skin key's, which names the skin
// it is on, made one name.
func skinKeyless(xs []string) []string {
	out := append([]string(nil), xs...)
	for i, x := range out {
		if len(x) > 5 && x[:5] == "Skin:" {
			out[i] = "Skin"
		}
	}
	return out
}

func maxDiff(a, b paintengine2d.Rect) float32 {
	d := float32(0)
	for _, v := range []float32{a.Min.X - b.Min.X, a.Min.Y - b.Min.Y, a.Max.X - b.Max.X, a.Max.Y - b.Max.Y} {
		if v < 0 {
			v = -v
		}
		d = max(d, v)
	}
	return d
}

func without(xs []string, drop ...string) []string {
	var out []string
next:
	for _, x := range xs {
		for _, d := range drop {
			if x == d {
				continue next
			}
		}
		out = append(out, x)
	}
	return out
}

func sameOrder(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	// A ring has no start: compare it rotated to the first name.
	for off := range b {
		ok := true
		for i := range a {
			if a[i] != b[(i+off)%len(b)] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// Minim Silver's equaliser has a tab for a header and no title band: it is
// the only window the skin gives its own caption, by the role the player
// gives it, and the other two keep the navy band.
func TestTheSilverEqualiserWearsItsOwnCaption(t *testing.T) {
	for _, scale := range scales {
		_, p := open(t, SkinSilver, scale)
		if p.Eq.FrameRole() != layoutEqualiser {
			t.Fatalf("the equaliser's role is %q", p.Eq.FrameRole())
		}
		st := style.DecorationState{Active: true}
		main := style.DecorationOf(p.Main.Look(), st)
		st.Role = p.Eq.FrameRole()
		eq := style.DecorationOf(p.Eq.Look(), st)
		whole := func(v float32) float32 { return float32(int(v*scale + 0.5)) }
		if main.Caption != whole(float32(panel.Silver.Caption)) || eq.Caption != whole(float32(panel.Silver.EqCaption)) {
			t.Errorf("@%gx: captions %g (strip) and %g (equaliser), want %g and %g",
				scale, main.Caption, eq.Caption, whole(15), whole(13))
		}
		// Above the display, to the right of any title: the strip's band
		// is navy, the equaliser's is the pale chrome of its face.
		x, y := int(200*scale), int(5*scale)
		navy := func(w *app.Window) bool {
			r, g, b, _ := w.Capture().PremulAt(x, y)
			return b > r+20 && r < 90 && g < 110
		}
		if !navy(p.Main) {
			t.Errorf("@%gx: the strip's caption is not the navy band", scale)
		}
		if navy(p.Eq) {
			t.Errorf("@%gx: the equaliser still has the navy band", scale)
		}
	}
	// In the classic panel every window keeps the one band.
	_, p := open(t, SkinClassic, 1)
	st := style.DecorationState{Active: true, Role: p.Eq.FrameRole()}
	if got := style.DecorationOf(p.Eq.Look(), st).Caption; got != float32(panel.Classic.Caption) {
		t.Errorf("classic equaliser caption %g, want the band's %d", got, panel.Classic.Caption)
	}
}

// A round key takes the pointer on its ink: the middle of Silver's play key
// is the key, the corners of its box are the face behind it. Classic's keys
// fill their boxes, and take the whole of them.
func TestTheRoundKeysRefuseTheirCorners(t *testing.T) {
	for _, scale := range scales {
		_, p := open(t, SkinSilver, scale)
		key := p.strip.play
		b := key.Bounds()
		at := func(dx, dy float32) widget.Component {
			return p.strip.HitTest(paintengine2d.Pt(b.Min.X+dx, b.Min.Y+dy))
		}
		if at(b.Dx()/2, b.Dy()/2) != widget.Component(key) {
			t.Fatalf("silver@%gx: the middle of the play key is not the key", scale)
		}
		for _, c := range [][2]float32{{1, 1}, {b.Dx() - 2, 1}, {1, b.Dy() - 2}, {b.Dx() - 2, b.Dy() - 2}} {
			if at(c[0], c[1]) == widget.Component(key) {
				t.Errorf("silver@%gx: the corner %v of the play key's box is the key", scale, c)
			}
		}
	}
	_, p := open(t, SkinClassic, 1)
	key := p.strip.play
	b := key.Bounds()
	if p.strip.HitTest(paintengine2d.Pt(b.Min.X+1, b.Min.Y+1)) != widget.Component(key) {
		t.Error("classic: a key drawn to its corners refuses its corner")
	}
	// And the keyboard never looks at any of it.
	p.strip.play.AccessibleAction(0, a11y.ActionDefault)
	if p.Transport.State != players.Playing {
		t.Error("the play key does not play from the accessibility tree")
	}
}

// Silver's keys brighten under the pointer; Classic's were never drawn
// with a hover face, and look the same with the pointer on them.
func TestPanelKeysHaveAHoverFaceWhereTheArtHasOne(t *testing.T) {
	for _, c := range []struct {
		id    string
		hover bool
	}{{SkinSilver, true}, {SkinClassic, false}} {
		_, p := open(t, c.id, 1)
		lk := p.strip.Look()
		paint := func(st style.ControlState) *paintengine2d.Image {
			img := paintengine2d.NewImage(40, 40)
			ctx := paintengine2d.NewContext(img)
			ctx.Clear(paintengine2d.Color{})
			style.DrawSkinSlot(lk, ctx, paintengine2d.XYWH(0, 0, 21, 21), layoutStrip, "play", st)
			return img
		}
		rest, hover := paint(style.StateNone), paint(style.StateHovered)
		differ := false
		for i := range rest.Pix {
			if rest.Pix[i] != hover.Pix[i] {
				differ = true
				break
			}
		}
		if differ != c.hover {
			t.Errorf("%s: the hover face differs from the resting one = %v, want %v", c.id, differ, c.hover)
		}
		// The key itself paints its hover face when the pointer is on it.
		p.strip.play.MouseEnter()
		if st := p.strip.play.PaintState(); !st.Hovered() {
			t.Errorf("%s: a key under the pointer is not hovered", c.id)
		}
	}
}

// The windows keep the player's titles in every face. The panels print them
// in capitals, and that is each skin's choice about its caption, not a
// change to what the window is called.
func TestTheTitlesAreThePlayersInEveryFace(t *testing.T) {
	want := map[string]string{"strip": "Minim — a visual demo", "equaliser": "Minim equaliser", "playlist": "Minim playlist"}
	for _, id := range []string{Skin, SkinClassic, SkinSilver, Themed} {
		_, p := open(t, id, 1)
		for name, w := range map[string]*app.Window{"strip": p.Main, "equaliser": p.Eq, "playlist": p.List} {
			if w.Title() != want[name] {
				t.Errorf("%s: the %s is titled %q, want %q", id, name, w.Title(), want[name])
			}
		}
	}
	for _, c := range []struct {
		id    string
		upper bool
	}{{SkinClassic, true}, {SkinSilver, true}, {Skin, false}} {
		sk, _ := style.LoadSkin(c.id)
		if got := sk.Text["caption"] != nil && sk.Text["caption"].Upper; got != c.upper {
			t.Errorf("%s: caption in capitals = %v, want %v", c.id, got, c.upper)
		}
	}
}

// At 1.25, 1.5 and 1.75 Minim Classic's transport row is whole pixels of its
// own art: every colour there is one the row is drawn in at 1x, so nothing
// was blended, and each piece continues the face under it.
//
// The palette is read off the player's own window at 1x rather than out of
// the skin's sprite sheet: the art lives in uitoolkit, embedded in the pack,
// and what this player can say about it is what it paints.
func TestTheClassicPanelIsWholePixelsAtFractionalScales(t *testing.T) {
	// row is the transport row, from the first key to the last, in the
	// strip's own coordinates.
	rowOf := func(p *Player) (paintengine2d.Rect, paintengine2d.Point) {
		first, _ := style.SkinSlotRect(p.strip.Look(), layoutStrip, "prev", p.strip.LocalBounds())
		last, _ := style.SkinSlotRect(p.strip.Look(), layoutStrip, "eject", p.strip.LocalBounds())
		first.Max.X = last.Max.X
		return first, widget.DeviceOrigin(p.strip)
	}
	shot := func(scale float32) (*paintengine2d.Image, paintengine2d.Rect, paintengine2d.Point) {
		_, p := open(t, SkinClassic, scale)
		p.Transport.State = players.Stopped
		p.refresh()
		row, o := rowOf(p)
		return p.Main.Capture(), row, o
	}
	// The palette is what the row is painted in at the two whole scales:
	// the 1x sheet's colours and the 2x sheet's, which is every colour the
	// art has to offer. A fractional scale draws from one of those two
	// sheets at whole multiples, so it may use no other.
	palette := map[[3]uint8]bool{}
	for _, whole := range []float32{1, 2} {
		img, row, o := shot(whole)
		for y := int(o.Y + row.Min.Y + 1); y < int(o.Y+row.Max.Y-1); y++ {
			for x := int(o.X + row.Min.X + 1); x < int(o.X+row.Max.X-1); x++ {
				r, g, b, _ := img.PremulAt(x, y)
				palette[[3]uint8{r, g, b}] = true
			}
		}
	}
	if len(palette) < 4 {
		t.Fatalf("the transport row is %d colours at 1x: nothing was painted", len(palette))
	}
	for _, scale := range []float32{1.25, 1.5, 1.75} {
		img, row, o := shot(scale)
		bad := 0
		for y := int(o.Y + row.Min.Y + 1); y < int(o.Y+row.Max.Y-1); y++ {
			for x := int(o.X + row.Min.X + 1); x < int(o.X+row.Max.X-1); x++ {
				r, g, b, _ := img.PremulAt(x, y)
				if !palette[[3]uint8{r, g, b}] {
					bad++
				}
			}
		}
		if bad > 0 {
			t.Errorf("%gx: %d pixels of the transport row are not colours of the art: blended, not whole pixels", scale, bad)
		}
	}
}
