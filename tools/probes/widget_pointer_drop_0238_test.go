//go:build ignore

// Stock scrollbar/drag and external file-drop routing, without app adapters.
// tools/testenv.sh go test -race ./tools/probes/widget_pointer_drop_0238_test.go
package pointeraudit

import (
	"fmt"
	"reflect"
	"testing"

	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

func TestScrollStripDoesNotStartRowDrag(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprint(scale), func(t *testing.T) {
			lk := style.WithScale(style.DarkLook(), scale)
			l := widgets.NewListView(40, func(int) string { return "row" }, nil)
			l.OnDrag = func([]int) *widget.Drag { return &widget.Drag{Types: []string{"text/plain"}} }
			var roots []*widgets.TreeNode
			for i := 0; i < 40; i++ {
				roots = append(roots, widgets.NewTreeNode("node"))
			}
			tr := widgets.NewTreeView(roots...)
			tr.OnDrag = func(*widgets.TreeNode) *widget.Drag { return &widget.Drag{Types: []string{"text/plain"}} }
			tb := widgets.NewTableView([]widgets.TableColumn{{Title: "A", Width: 80}, {Title: "B", Width: 80}}, 40, func(int, int) string { return "cell" }, nil)
			tb.OnDrag = func([]int) *widget.Drag { return &widget.Drag{Types: []string{"text/plain"}} }
			for _, c := range []interface {
				widget.Component
				DragAt(paint2d.Point) *widget.Drag
			}{l, tr, tb} {
				c.SetLook(lk)
				c.Arrange(paint2d.XYWH(0, 0, 220*scale, 140*scale))
				row := widget.MouseEvent{Button: platform.ButtonLeft, Pos: paint2d.Pt(20*scale, 70*scale)}
				c.MousePress(row)
				c.MouseRelease(row)
				// An item drag is the positive control. The rightmost strip is
				// visibly reserved for scrollbar/gutter in all three stock views.
				if c.DragAt(paint2d.Pt(20*scale, 70*scale)) == nil {
					t.Fatalf("%T refuses ordinary row drag", c)
				}
				for x := float32(210) * scale; x < 220*scale; x += .5 * scale {
					if c.DragAt(paint2d.Pt(x, 70*scale)) != nil {
						t.Errorf("%T starts row drag from right strip x=%g", c, x)
					}
				}
			}
		})
	}
}

func TestMenuScrollbarDoesNotRunAnItem(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		calls := 0
		var items []*widgets.MenuItem
		for i := 0; i < 40; i++ {
			items = append(items, widgets.Item(fmt.Sprint(i), func() { calls++ }))
		}
		p := widgets.NewPopupMenu(items...)
		p.SetLook(style.WithScale(style.DarkLook(), scale))
		p.Arrange(paint2d.XYWH(0, 0, 240*scale, 108*scale))
		if p.MaxOffset() <= 0 {
			t.Fatal("menu did not overflow")
		}
		click := func(x float32) {
			e := widget.MouseEvent{Button: platform.ButtonLeft, Pos: paint2d.Pt(x, 54*scale)}
			p.MousePress(e)
			p.MouseRelease(e)
		}
		for x := float32(230) * scale; x < 240*scale; x += .5 * scale {
			click(x)
			if calls != 0 {
				t.Fatalf("scale%g scrollbar x%g ran item", scale, x)
			}
		}
		click(20 * scale)
		if calls != 1 {
			t.Fatalf("scale%g item positivecontrol calls=%d", scale, calls)
		}
	}
}

func TestFileDropOverTextFieldGoesToFileAncestor(t *testing.T) {
	a := ui.New(ui.Options{Headless: true, DisableLookWatch: true})
	w, e := a.NewWindow(platform.WindowOptions{Headless: true, Width: 400, Height: 200, Decorations: platform.DecorationsNone})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(w.Close)
	field := widgets.NewTextField("original", "", nil)
	var files []string
	zone := widgets.NewDropZone(field, func(p []string) { files = append(files, p...) })
	w.SetContent(widgets.NewColumn(zone))
	a.PumpOnce()
	o := w.Surface().(*platform.Offscreen)
	at := widget.DeviceBounds(field).Center()
	o.SimulateDropAction(at, map[string][]byte{"text/uri-list": []byte("file:///tmp/song.mp3\r\n"), "text/plain": []byte("file:///tmp/song.mp3")}, platform.DragCopy, platform.DragCopy)
	a.PumpOnce()
	if !reflect.DeepEqual(files, []string{"/tmp/song.mp3"}) || field.Text != "original" || o.DropTaken() == nil || !*o.DropTaken() {
		t.Fatalf("files=%q field=%q taken=%v", files, field.Text, o.DropTaken())
	}
	// A text-only offer remains useful to the child editor.
	o.SimulateDropAction(at, map[string][]byte{"text/plain": []byte("added")}, platform.DragCopy, platform.DragCopy)
	a.PumpOnce()
	if field.Text == "original" || len(files) != 1 || o.DropTaken() == nil || !*o.DropTaken() {
		t.Fatalf("text control files=%q field=%q taken=%v", files, field.Text, o.DropTaken())
	}
	// Mixed URI list preserves remote authority instead of delivering it
	// as a local path through the stock file-oriented ancestor.
	o.SimulateDropAction(at, map[string][]byte{"text/uri-list": []byte("file://remote.invalid/tmp/remote.mp3\r\n"), "text/plain": []byte("remote")}, platform.DragCopy, platform.DragCopy)
	a.PumpOnce()
	if len(files) != 1 || o.DropTaken() == nil || *o.DropTaken() {
		t.Fatalf("remote-only delivered locally: files=%q taken=%v", files, o.DropTaken())
	}
}
