package lantern

import (
	"strings"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/playertest"
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit"
	"github.com/codemodify/uitoolkit/a11y"
	"github.com/codemodify/uitoolkit/app"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/rack"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
)

func open(t *testing.T, pack string, scale float32) (*uitoolkit.Application, *Player) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(style.ThemeEnv, pack)
	a := uitoolkit.New(uitoolkit.Options{Headless: true, Scale: scale, DisableLookWatch: true})
	h := players.NewHost(a)
	h.Packs = func() []string { return []string{Skin} }
	p, err := New(h, Options{Headless: true, Scale: scale, Themed: pack != Skin})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, w := range []*app.Window{p.Main, p.List} {
			if w != nil {
				w.Close()
			}
		}
	})
	a.PumpOnce()
	return a, p
}

func TestTheSkinIsAPartialOverride(t *testing.T) {
	sk, ok := style.LoadSkin(Skin)
	if !ok {
		t.Fatalf("%q did not load", Skin)
	}
	if sk.Window == nil || len(sk.Window.Shape) == 0 {
		t.Fatal("the skin declares no window shape")
	}
	// It is meant to be partial: some parts are its own and some are its
	// base pack's, which is the case the demo is showing.
	bound, loose := 0, 0
	for _, part := range style.SkinPartNames() {
		if _, ok := sk.Parts[part]; ok {
			bound++
		} else {
			loose++
		}
	}
	if bound == 0 || loose == 0 {
		t.Errorf("the skin binds %d parts and leaves %d; it is meant to do both", bound, loose)
	}
	for _, sh := range sk.Sheets {
		if !sh.Covers(2) {
			t.Errorf("sheet %s has no asset at 2x", sh.Name)
		}
	}
}

func TestBothWindowsAreAccessible(t *testing.T) {
	for _, pack := range []string{Skin, Themed} {
		t.Run(pack, func(t *testing.T) {
			a, p := open(t, pack, 1)
			_ = a
			main := playertest.Audit(t, "main", p.Main)
			playertest.Audit(t, "playlist", p.List)
			if playertest.Count(main, a11y.RoleMenuBar) == 0 {
				t.Error("no menu bar in the tree")
			}
			if playertest.Count(main, a11y.RoleStatusBar) == 0 {
				t.Error("no status bar in the tree")
			}
			if playertest.Count(p.List.AccessibleTree(), a11y.RoleTable) == 0 {
				t.Error("the playlist is not a table in the tree")
			}
			for _, want := range []string{"Play", "Stop", "Seek", "Volume", "Skin", "Playlist window"} {
				if !named(main, want) {
					t.Errorf("no node named %q", want)
				}
			}
		})
	}
}

func TestEveryControlIsOnTheKeyboard(t *testing.T) {
	a, p := open(t, Skin, 1)
	ring := playertest.FocusRing(t, a, p.Main)
	for _, c := range []widget.Component{
		p.body.prev, p.body.play, p.body.stop, p.body.next,
		p.body.mute, p.body.seek, p.body.volume,
		p.body.shuffle, p.body.repeat, p.body.listBtn, p.body.skinBtn,
	} {
		if !playertest.Reaches(ring, c) {
			t.Errorf("Tab never reaches %T", c)
		}
	}
	list := playertest.FocusRing(t, a, p.List)
	for _, c := range []widget.Component{p.queue.search, p.queue.table, p.queue.anchor} {
		if !playertest.Reaches(list, c) {
			t.Errorf("Tab never reaches %T in the playlist", c)
		}
	}
}

// The switch is the argument: the same app, another pack, and the outline
// goes with the skin because it never belonged to the app.
func TestDroppingTheSkinKeepsTheApp(t *testing.T) {
	a, p := open(t, Skin, 1)
	if !p.Main.ShapeActive() {
		t.Fatal("the skinned window is not shaped")
	}
	ringBefore := playertest.RingNames(p.Main, playertest.FocusRing(t, a, p.Main))
	treeBefore := names(p.Main.AccessibleTree())
	statusBefore := p.Status()

	p.SetSkinned(false)
	a.PumpOnce()

	if p.Main.ShapeActive() {
		t.Error("the window kept a silhouette with the skin dropped")
	}
	if got := p.App.Look().Name(); got == "" {
		t.Error("the look has no name")
	}
	// The ring is a cycle and Tab starts it wherever the focus happens to
	// be, so it is compared as a cycle: rotated to the same first stop.
	ringAfter := rotateTo(playertest.RingNames(p.Main, playertest.FocusRing(t, a, p.Main)), ringBefore[0])
	if len(ringAfter) != len(ringBefore) {
		t.Errorf("the tab order changed with the skin: %d stops then %d\n  %v\n  %v",
			len(ringBefore), len(ringAfter), ringBefore, ringAfter)
	} else {
		for i := range ringBefore {
			if ringBefore[i] != ringAfter[i] {
				t.Errorf("tab stop %d was %q and is now %q", i, ringBefore[i], ringAfter[i])
			}
		}
	}
	if got := names(p.Main.AccessibleTree()); !sameStrings(treeBefore, got) {
		t.Errorf("the accessibility tree changed with the skin\n  before %v\n  after  %v", treeBefore, got)
	}
	// The one thing that is meant to differ, and does.
	if got := p.Status(); got == statusBefore {
		t.Errorf("the status line still reads %q with the skin dropped", got)
	}
	img := p.Main.Capture()
	if img == nil || flat(img) {
		t.Fatal("the themed window painted nothing")
	}
	playertest.Audit(t, "themed main", p.Main)

	// And back again.
	p.SetSkinned(true)
	a.PumpOnce()
	if !p.Main.ShapeActive() {
		t.Error("the outline did not come back with the skin")
	}
}

// Ctrl+K is the same switch from the keyboard, and Ctrl+L opens and closes
// the playlist window.
func TestTheSwitchIsOnTheKeyboard(t *testing.T) {
	a, p := open(t, Skin, 1)
	press := func(k platform.Key) {
		p.Main.Inject(platform.Event{Kind: platform.EventKeyDown, Key: k, Mods: platform.ModCtrl})
		a.PumpOnce()
	}
	press(platform.KeyK)
	if p.Skinned() {
		t.Error("Ctrl+K did not drop the skin")
	}
	press(platform.KeyK)
	if !p.Skinned() {
		t.Error("Ctrl+K did not put it back")
	}
	press(platform.KeyL)
	if p.ListShown() {
		t.Error("Ctrl+L did not close the playlist")
	}
	press(platform.KeyL)
	if !p.ListShown() {
		t.Error("Ctrl+L did not open it again")
	}
}

// The skirt is asymmetric, and it is the same skirt at every scale.
func TestTheSkirtIsTheSameAtEveryScale(t *testing.T) {
	const n = 64
	var base []float64
	for _, scale := range []float32{1, 1.25, 1.5, 1.75, 2} {
		a, p := open(t, Skin, scale)
		_ = a
		if !p.Main.ShapeActive() {
			t.Fatalf("%.2fx: not shaped", scale)
		}
		out := playertest.Outline(p.Main, n)
		if len(out) != n {
			t.Fatalf("%.2fx: no outline", scale)
		}
		if base == nil {
			base = out
			if out[n/2] < 0.999 {
				t.Errorf("the middle of the window covers %.3f of the width", out[n/2])
			}
			if last := out[n-1]; last >= 1 {
				t.Errorf("the last row covers %.3f — the skirt is not sweeping in at all", last)
			}
			// The skirt is asymmetric, which is the whole of this outline:
			// one bottom corner is swept round several times as far as the
			// other. Coverage alone cannot see that; the two insets can.
			l, r := playertest.Insets(p.Main, 0.99)
			if l < r*3 {
				t.Errorf("the skirt is %d in on the left and %d on the right — that is not asymmetric", l, r)
			}
			continue
		}
		if d := playertest.OutlineDiff(base, out); d > 0.02 {
			t.Errorf("%.2fx: the outline differs from 1x by %.3f of the window's width\n  1x:    %.3v\n  %.2fx: %.3v",
				scale, d, base, scale, out)
		}
	}
}

// The playlist window is anchored to the main window's edge and travels
// with it, and the control that says where it sits moves it.
func TestThePlaylistIsAnchoredAndFollows(t *testing.T) {
	a, p := open(t, Skin, 1)
	if !p.Desk.Places() {
		t.Skip("this backend does not place windows")
	}
	pane := p.Desk.Rack.Pane(p.iList)
	if pane.To != p.iMain || pane.Bond.Side != rack.SideRight {
		t.Fatalf("the playlist is bonded to %d on the %v", pane.To, pane.Bond.Side)
	}
	p.Main.Move(120, 90)
	a.PumpOnce()
	p.Desk.Follow()
	main := p.Desk.Rack.Pane(p.iMain).Box
	if pane.Box.X != main.Right() || pane.Box.Y != main.Y {
		t.Errorf("the playlist is at %+v, want flush right of %+v", pane.Box, main)
	}

	// The control: put it under the player instead.
	p.Desk.Attach(p.iList, p.iMain, rack.SideBottom)
	a.PumpOnce()
	if pane.Box.Y != main.Bottom() || pane.Box.X != main.X {
		t.Errorf("re-anchored to %+v, want flush under %+v", pane.Box, main)
	}
}

// The one text field in all three players: typing in it does not reach the
// transport, which is what makes bare-letter shortcuts safe everywhere else.
func TestTypingInTheFilterDoesNotDriveTheTransport(t *testing.T) {
	a, p := open(t, Skin, 1)
	p.List.RequestFocus(p.queue.search)
	a.PumpOnce()
	before := p.Transport.List.Index()
	p.List.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyB})
	p.List.Inject(platform.Event{Kind: platform.EventText, Rune: 'B'})
	a.PumpOnce()
	if p.Transport.List.Index() != before {
		t.Error("typing B in the filter moved to the next track")
	}
	if got := p.queue.search.Text; got != "B" {
		t.Errorf("the filter holds %q, want %q", got, "B")
	}
	// And it filters: "beacon" is one of the invented tracks.
	p.queue.search.SetText("beacon")
	p.queue.filter("beacon")
	a.PumpOnce()
	if p.queue.table.RowCount != 1 {
		t.Errorf("the filter left %d rows, want 1", p.queue.table.RowCount)
	}
	p.queue.filter("")
	if p.queue.table.RowCount != p.Transport.List.Len() {
		t.Errorf("clearing the filter left %d rows, want %d", p.queue.table.RowCount, p.Transport.List.Len())
	}

	// With the focus back on the player, the same key does move.
	p.Main.RequestFocus(p.body.play)
	a.PumpOnce()
	p.Main.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyB})
	a.PumpOnce()
	if p.Transport.List.Index() == before {
		t.Error("B did not move to the next track when the focus was on the player")
	}
}

func TestTheStatusLineSaysWhatThisIs(t *testing.T) {
	a, p := open(t, Skin, 1)
	p.Pose(97 * time.Second)
	a.PumpOnce()
	if !namedContaining(p.Main.AccessibleTree(), players.Disclaimer) {
		t.Error("nothing in the tree says this is a demo")
	}
	if got := p.Status(); !strings.Contains(got, Skin) {
		t.Errorf("the status line is %q and does not name the skin", got)
	}
	p.SetSkinned(false)
	if got := p.Status(); !strings.Contains(got, Themed) {
		t.Errorf("the status line is %q and does not name the pack", got)
	}
}

// ---- helpers ------------------------------------------------------------------

func named(tree *a11y.Node, name string) bool {
	found := false
	tree.Walk(func(n *a11y.Node) bool {
		if n.Name == name {
			found = true
		}
		return !found
	})
	return found
}

func namedContaining(tree *a11y.Node, part string) bool {
	found := false
	tree.Walk(func(n *a11y.Node) bool {
		if strings.Contains(n.Name, part) || strings.Contains(n.Description, part) {
			found = true
		}
		return !found
	})
	return found
}

// names is the shape of the tree a screen reader walks: every node's role,
// in order, and the name of every node that is a *control*.
//
// The names of the nodes that are not controls are deliberately left out.
// One of them is the status line, whose entire job is to say which pack is
// on, so it must differ across the switch; asserting it did not would be
// asserting the app tells the same lie in both looks.
func names(tree *a11y.Node) []string {
	var out []string
	tree.Walk(func(n *a11y.Node) bool {
		if n.Role.Interactive() {
			out = append(out, n.Role.String()+" "+n.Name)
		} else {
			out = append(out, n.Role.String())
		}
		return true
	})
	return out
}

// rotateTo turns a cycle round until it starts at first, or leaves it alone
// when first is not in it.
func rotateTo(ring []string, first string) []string {
	for i, s := range ring {
		if s == first {
			return append(append([]string(nil), ring[i:]...), ring[:i]...)
		}
	}
	return ring
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func flat(img *paintengine2d.Image) bool {
	r0, g0, b0, _ := img.PremulAt(0, 0)
	for y := 0; y < img.Height; y += 3 {
		for x := 0; x < img.Width; x += 3 {
			r, g, b, _ := img.PremulAt(x, y)
			if r != r0 || g != g0 || b != b0 {
				return false
			}
		}
	}
	return true
}
