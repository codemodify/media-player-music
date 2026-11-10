package winamp

import (
	"reflect"
	"testing"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/playertest"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

func TestPlaylistCaptionDropsPreserveCaptionAndPlayerActions(t *testing.T) {
	for _, skin := range []string{Skin, SkinSilver} {
		t.Run(skin, func(t *testing.T) {
			a, p := open(t, skin, 1.75)
			var got []string
			calls := 0
			p.Host.DropFiles = func(paths []string) { got = paths; calls++ }
			header := p.List.Caption()
			id := header.ID()
			children := append([]widget.Component(nil), header.Children()...)
			for _, appearance := range []string{skin, otherPanelSkin(skin)} {
				p.Host.ApplySkin(appearance)
				a.PumpOnce()
				p.Advance(0)
				a.PumpOnce()
				if p.List.Caption() != header || header.ID() != id || !reflect.DeepEqual(header.Children(), children) {
					t.Fatal("binding a caption drop policy rebuilt toolkit controls or identities")
				}
				caption := widget.DeviceBounds(header).Center()
				if region, _ := p.List.NonClientHit(caption); region != ui.RegionCaption {
					t.Fatalf("the original caption lost its hit/drag identity: %s", region)
				}
				tree := playertest.Audit(t, "playlist caption", p.List)
				close := captionCloseNode(t, tree)
				left, right := header.Controls()
				for _, button := range append(left.Shown(), right.Shown()...) {
					if button == platform.CaptionMinimize || button == platform.CaptionMaximize {
						t.Fatal("the playlist's hidden caption buttons were restored")
					}
				}
				for _, point := range []paint2d.Point{caption, close.Bounds.Center(), widget.DeviceBounds(left).Center()} {
					before := calls
					p.List.Surface().(*platform.Offscreen).SimulateDropAction(point, map[string][]byte{
						"text/uri-list": []byte("file:///home/user/Music/net\r\n"),
					}, platform.DragCopy|platform.DragMove, platform.DragMove)
					a.PumpOnce()
					if taken := p.List.Surface().(*platform.Offscreen).DropTaken(); taken == nil || !*taken || calls != before+1 ||
						!reflect.DeepEqual(got, []string{"/home/user/Music/net"}) {
						t.Fatalf("caption drop at %v did not append once: paths=%q calls=%d taken=%v", point, got, calls, taken)
					}
					if !p.List.Visible() || p.List.Closed() {
						t.Fatal("dropping on Close invoked the caption control")
					}
				}
				off := p.List.Surface().(*platform.Offscreen)
				moves := off.FrameCalls().Moves
				p.List.Inject(platform.Event{Kind: platform.EventMouseDown, Button: platform.ButtonLeft, Pos: caption})
				p.List.Inject(platform.Event{Kind: platform.EventMouseMove, Pos: caption.Add(paint2d.Pt(35, 0))})
				p.List.Inject(platform.Event{Kind: platform.EventMouseUp, Button: platform.ButtonLeft, Pos: caption.Add(paint2d.Pt(35, 0))})
				a.PumpOnce()
				if off.FrameCalls().Moves != moves+1 {
					t.Fatal("caption dragging no longer starts one toolkit window move")
				}
				p.List.RequestFocus(p.listPane.list)
				p.Transport.Stop()
				p.List.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeySpace})
				a.PumpOnce()
				if p.Transport.State != players.Playing || p.List.Focus() != p.listPane.list {
					t.Fatal("binding the caption lost playlist focus or transport keyboard access")
				}
				close = captionCloseNode(t, p.List.AccessibleTree())
				p.List.Inject(platform.Event{Kind: platform.EventMouseMove, Pos: close.Bounds.Center()})
				p.List.Inject(platform.Event{Kind: platform.EventMouseDown, Button: platform.ButtonLeft, Pos: close.Bounds.Center()})
				a.PumpOnce()
				p.List.Inject(platform.Event{Kind: platform.EventMouseUp, Button: platform.ButtonLeft, Pos: close.Bounds.Center()})
				a.PumpOnce()
				// Window.RequestClose queues its close request for the next turn.
				a.PumpOnce()
				if p.List.Visible() || p.List.Closed() || p.Host.Model.PlaylistVisible {
					t.Fatalf("caption Close lost its original hide action: visible=%t closed=%t model=%t", p.List.Visible(), p.List.Closed(), p.Host.Model.PlaylistVisible)
				}
				p.ShowPlaylist(true)
				a.PumpOnce()
				close = captionCloseNode(t, p.List.AccessibleTree())
				if !p.List.AccessibleAction(close.ID, a11y.ActionDefault) {
					t.Fatal("accessible caption Close was refused")
				}
				a.PumpOnce()
				if p.List.Visible() || p.List.Closed() {
					t.Fatal("accessible caption Close lost its original action")
				}
				p.ShowPlaylist(true)
				a.PumpOnce()
			}
			replacement := widgets.NewHeaderBar(nil, nil, nil)
			replacement.ShowTitle = true
			p.List.SetTitleBar(replacement)
			a.PumpOnce()
			p.Advance(0)
			a.PumpOnce()
			if p.List.Caption() != replacement || replacement.Parent() != p.playlistCaption {
				t.Fatal("a new toolkit caption did not acquire the playlist drop policy")
			}
			off := p.List.Surface().(*platform.Offscreen)
			before := calls
			off.SimulateDrop(widget.DeviceBounds(replacement).Center(), map[string][]byte{"text/uri-list": []byte("file:///home/user/Music/net\r\n")})
			a.PumpOnce()
			if taken := off.DropTaken(); taken == nil || !*taken || calls != before+1 {
				t.Fatal("the replacement caption refused a folder drop")
			}
		})
	}
}

func captionCloseNode(t *testing.T, tree *a11y.Node) *a11y.Node {
	t.Helper()
	var close *a11y.Node
	tree.Walk(func(node *a11y.Node) bool {
		if node.Role == a11y.RoleButton && node.Name == "Close" {
			close = node
		}
		return true
	})
	if close == nil || close.Bounds.Empty() {
		t.Fatal("the toolkit caption lost its accessible Close button")
	}
	return close
}
