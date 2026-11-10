//go:build ignore

// Raw public UIToolkit widget regressions, introduced at v0.23.5 and rechecked
// at v0.23.6, without Music Player guards.
// Run explicitly: tools/testenv.sh go test -race ./tools/probes/widget_regressions_0235_test.go
package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	app "goframework.io/desktop-ui"
	"goframework.io/desktop-ui-skingen"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/layout"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
	"image/color"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
)

func window(t *testing.T, scale float32, w, h int) (*app.Application, *app.Window) {
	t.Helper()
	a := app.New(app.Options{Headless: true, Scale: scale, DisableLookWatch: true})
	win, e := a.NewWindow(platform.WindowOptions{Headless: true, Width: w, Height: h, Decorations: platform.DecorationsNone})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		for _, w := range a.Windows() {
			w.Close()
		}
	})
	return a, win
}
func skin(col paint2d.Color, label string) fstest.MapFS {
	im := paint2d.NewImage(8, 8)
	paint2d.NewContext(im).Clear(col)
	var b bytes.Buffer
	im.WritePNG(&b)
	doc := fmt.Sprintf(`{"skin":1,"label":%q,"base":"dark","sheets":{"art":{"1x":"art.png"}},"sprites":{"test":{"at":[0,0,8,8]}}}`, label)
	return fstest.MapFS{"skin.json": {Data: []byte(doc)}, "art.png": {Data: b.Bytes()}}
}
func pixel(t *testing.T, lk style.LookAndFeel, scale float32) color.NRGBA {
	im := paint2d.NewImage(24, 24)
	if !skins.DrawSkinSprite(style.WithScale(lk, scale), paint2d.NewContext(im), paint2d.XYWH(0, 0, 20, 20), "test", paint2d.Color{}) {
		t.Fatal("sprite missing")
	}
	return im.NRGBAAt(10, 10)
}
func TestRegisteredSkinReplacementOverrideAndEnumeration(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	const id = "raw-widget-audit-skin"
	reg := func(c paint2d.Color, label string) {
		if e := skins.RegisterSkinFS(skin(c, label), id); e != nil {
			t.Fatal(e)
		}
	}
	reg(paint2d.RGB(1, 0, 0), "First")
	pack, _ := style.LoadTheme(id)
	old := pack.Look()
	for _, s := range []float32{1, 1.75} {
		if got := pixel(t, old, s); got != (color.NRGBA{255, 0, 0, 255}) {
			t.Fatal(got)
		}
	}
	reg(paint2d.RGB(0, 0, 1), "Replacement")
	fresh, _ := style.LoadTheme(id)
	for _, s := range []float32{1, 1.75} {
		for _, lk := range []style.LookAndFeel{old, fresh.Look()} {
			if got := pixel(t, lk, s); got != (color.NRGBA{0, 0, 255, 255}) {
				t.Fatal("stale", got)
			}
		}
	}
	count := 0
	for _, p := range skins.ListSkins() {
		if p.Name == id {
			count++
			if p.Label != "Replacement" {
				t.Fatal("enumeration stale")
			}
		}
	}
	if count != 1 {
		t.Fatal("registered enumeration", count)
	}
	dir := skins.UserSkinDir(id)
	if e := os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	for name, file := range skin(paint2d.RGB(0, 1, 0), "User override") {
		if e := os.WriteFile(filepath.Join(dir, name), file.Data, 0600); e != nil {
			t.Fatal(e)
		}
	}
	skins.InvalidateSkinCache()
	for _, s := range []float32{1, 1.75} {
		if got := pixel(t, old, s); got != (color.NRGBA{0, 255, 0, 255}) {
			t.Fatal("override", got)
		}
	}
	var labels []string
	for _, p := range skins.ListSkins() {
		if p.Name == id {
			labels = append(labels, p.Label)
		}
	}
	if !reflect.DeepEqual(labels, []string{"Replacement", "User override"}) {
		t.Fatal("override policy", labels)
	}
	os.RemoveAll(dir)
	skins.InvalidateSkinCache()
	for _, s := range []float32{1, 1.75} {
		if got := pixel(t, old, s); got != (color.NRGBA{0, 0, 255, 255}) {
			t.Fatal("override removal", got)
		}
	}
	t.Log("1/20/21 registration, replacement through used/fresh looks, user precedence and ordered duplicate policy pass")
}
func TestSliderNormalization(t *testing.T) {
	for _, where := range []string{"OnChange", "OnInput"} {
		s := widgets.NewSlider(0, 100, 50, nil)
		s.Step = 1
		var changes, inputs []float32
		s.OnChange = func(v float32) {
			changes = append(changes, v)
			if where == "OnChange" && v == 51 {
				s.SetValue(60)
			}
		}
		s.OnInput = func(v float32) {
			inputs = append(inputs, v)
			if where == "OnInput" && v == 51 {
				s.SetValue(60)
			}
		}
		s.KeyPress(widget.KeyEvent{Key: platform.KeyRight})
		want := []float32{60}
		if where == "OnInput" {
			want = []float32{51}
		}
		if s.Value != 60 || !reflect.DeepEqual(inputs, want) || !reflect.DeepEqual(changes, []float32{51, 60}) {
			t.Fatal(where, s.Value, changes, inputs)
		}
		t.Log(where, "changes", changes, "inputs", inputs, "final", s.Value)
	}
}

type wrappedList struct{ *widgets.ListView }

func TestListWrapperFocusAndAccessibleGeometry(t *testing.T) {
	a, w := window(t, 1, 200, 100)
	l := &wrappedList{widgets.NewListView(8, nil, nil)}
	l.Init(l)
	l.Selected = 0
	l.RowGeo = func(style.LookAndFeel) *widgets.RowGeometry {
		return &widgets.RowGeometry{Rows: paint2d.XYWH(10, 20, 120, 60), Bar: paint2d.XYWH(140, 20, 10, 60), RowHeight: 20}
	}
	focused := false
	l.RowPaint = func(_ *paint2d.Context, _ paint2d.Rect, i int, st style.ControlState) bool {
		if i == 0 {
			focused = st.Focused()
		}
		return true
	}
	w.SetContent(l)
	a.PumpOnce()
	w.RequestFocus(l)
	a.PumpOnce()
	if w.Focus() != l || !focused {
		t.Fatal("wrapper focus missing", w.Focus(), focused)
	}
	first := l.AccessibleItems()[0]
	if first.Bounds != (paint2d.XYWH(10, 20, 120, 20)) {
		t.Fatal("row bounds", first.Bounds)
	}
	l.ScrollTo(20)
	first = l.AccessibleItems()[0]
	if first.Bounds != (paint2d.XYWH(10, 0, 120, 20)) || !first.State.Has(a11y.StateOffscreen) {
		t.Fatal("scrolled row", first.Bounds, first.State)
	}
	t.Log("3 focused outer row; first=(10,20,120,20), scrolled=(10,0,120,20) offscreen")
}
func TestPopupAccessibleActions(t *testing.T) {
	for _, native := range []bool{false, true} {
		a, w := window(t, 1, 275, 116)
		if native {
			w.Surface().(*platform.Offscreen).SimulatePopups(platform.FrameRect{W: 1280, H: 760})
		}
		b := widgets.NewButton("Open", nil)
		w.SetContent(b)
		a.PumpOnce()
		picked := 0
		disabled := widgets.Item("Disabled", func() { t.Error("disabled action") })
		disabled.Disabled = true
		pop := widgets.ShowContextMenu(b, paint2d.Pt(8, 8), disabled, widgets.Item("Enabled", func() { picked++ }))
		a.PumpOnce()
		if pop.AccessibleItems()[0].Actions.Has(a11y.ActionDefault) || w.AccessibleAction(widget.ItemID(pop, 0), a11y.ActionDefault) {
			t.Fatal("disabled advertised/executed")
		}
		if !w.AccessibleAction(widget.ItemID(pop, 1), a11y.ActionDefault) {
			t.Fatal("enabled action rejected")
		}
		a.PumpOnce()
		if picked != 1 || w.Popup() != nil {
			t.Fatal("action count/close", picked, w.Popup())
		}
		t.Log("4 native-simulation", native, "enabled executed once and closed; disabled neither advertised nor executed")
	}
}

type leaf struct{ widget.Base }

func newLeaf() *leaf { l := &leaf{}; l.Init(l); return l }
func (l *leaf) Measure(c layout.Constraints) paint2d.Point {
	return c.Constrain(paint2d.Pt(40, 40))
}
func (l *leaf) Arrange(r paint2d.Rect) { l.SetBounds(r) }
func TestHiddenGridSpan(t *testing.T) {
	for _, vertical := range []bool{false, true} {
		g := widgets.NewGrid()
		g.SetLook(style.DarkLook())
		g.RowGap, g.ColGap = 8, 8
		first, middle, last, span := newLeaf(), newLeaf(), newLeaf(), newLeaf()
		if !vertical {
			g.Cols = []widgets.Track{widgets.Auto(), widgets.Auto(), widgets.Share(1)}
			g.Place(first, 0, 0)
			g.Place(middle, 0, 1)
			g.Place(last, 0, 2)
			g.PlaceSpan(span, 1, 0, 1, 3)
		} else {
			g.Rows = []widgets.Track{widgets.Auto(), widgets.Auto(), widgets.Share(1)}
			g.Place(first, 0, 0).VAlign = layout.AlignStretch
			g.Place(middle, 1, 0).VAlign = layout.AlignStretch
			g.Place(last, 2, 0).VAlign = layout.AlignStretch
			g.PlaceSpan(span, 0, 1, 3, 1).VAlign = layout.AlignStretch
		}
		middle.SetVisible(false)
		g.Arrange(paint2d.XYWH(0, 0, 400, 400))
		end, wide := last.Bounds().Max.X, span.Bounds().Max.X
		if vertical {
			end, wide = last.Bounds().Max.Y, span.Bounds().Max.Y
		}
		if end != 400 || wide != 400 {
			t.Fatal(vertical, end, wide)
		}
		t.Log("5 vertical", vertical, "trailing/span end", end, wide)
	}
}
func TestCompactChooser(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		a, w := window(t, scale, 275, 116)
		b := widgets.NewButton("Choose", nil)
		w.SetContent(b)
		a.PumpOnce()
		w.RequestFocus(b)
		cancel := 0
		fd := widgets.ShowFileDialog(b, widgets.FileDialogOptions{Path: t.TempDir(), Entries: []widgets.FileInfo{{Name: "one.mp3"}, {Name: "two.mp3"}}, OnCancel: func() { cancel++ }})
		a.PumpOnce()
		if fd == nil || len(a.Windows()) != 2 {
			t.Fatal("no automatic chooser host")
		}
		var dw *app.Window
		for _, candidate := range a.Windows() {
			if candidate != w {
				dw = candidate
			}
		}
		off := dw.Surface().(*platform.Offscreen)
		if off.Owner() != w.Surface() || off.WindowRole() != platform.RoleDialog {
			t.Fatal("dialog owner/role")
		}
		names := map[string]bool{}
		widget.Walk(dw.Content(), func(c widget.Component) {
			switch v := c.(type) {
			case *widgets.TextField:
				names["path"] = true
			case *widgets.TableView:
				names["table"] = true
				if len(v.AccessibleItems()) < 2 {
					t.Error("no table rows")
				}
			case *widgets.Button:
				names[v.Text] = true
			}
			r := widget.DeviceBounds(c)
			if c.Visible() && (r.Empty() || r.Min.X < -.5 || r.Min.Y < -.5 || r.Max.X > dw.WindowRect().Dx()+.5 || r.Max.Y > dw.WindowRect().Dy()+.5) {
				t.Errorf("clipped %T %v in%v", c, r, dw.WindowRect())
			}
		})
		for _, name := range []string{"path", "table", "Open", "Cancel"} {
			if !names[name] {
				t.Error("missing control", name)
			}
		}
		dw.RequestClose()
		for i := 0; i < 4; i++ {
			a.PumpOnce()
		}
		fd.Cancel()
		if cancel != 1 || !dw.Closed() || w.Closed() || w.Focus() != b {
			t.Fatal("cancel/lifecycle/focus", cancel, dw.Closed(), w.Focus())
		}
		ww, hh := dw.Size()
		t.Logf("11 scale=%g automatic=%dx%d role=dialog owner=requester rows+buttons visible cancel=1 requester focus retained", scale, ww, hh)
	}
}
func TestGeneratorAnchorsAndShapePins(t *testing.T) {
	dir := t.TempDir()
	p := &skingen.Plan{Name: "raw-authoring", Label: "Raw authoring", Base: "dark", Layouts: []skingen.LayoutSpec{{Name: "panel", W: 100, H: 80, Slots: []skingen.SlotSpec{{Name: "stretch", At: [4]int{5, 7, 9, 11}, StretchX: true, StretchY: true}, {Name: "edge", At: [4]int{10, 12, 8, 6}, FromRight: true, FromBottom: true}}}}, Window: &skingen.WindowSpec{Layout: ":", Shape: []skingen.ShapeRect{{At: [4]int{10, 12, 8, 6}, FromRight: true, FromBottom: true}}}}
	if e := skingen.Write(dir, p); e != nil {
		t.Fatal(e)
	}
	fsys := os.DirFS(filepath.Join(dir, p.Name))
	raw, e := os.ReadFile(filepath.Join(dir, p.Name, "skin.json"))
	if e != nil {
		t.Fatal(e)
	}
	var doc map[string]any
	json.Unmarshal(raw, &doc)
	shape := doc["window"].(map[string]any)["shape"].([]any)[0].(map[string]any)
	if shape["fromRight"] != true || shape["fromBottom"] != true {
		t.Fatal("shape pins dropped", shape)
	}
	sk, e := skins.LoadSkinFS(fsys, p.Name)
	if e != nil {
		t.Fatal(e)
	}
	if !sk.Window.Shape[0].FromRight || !sk.Window.Shape[0].FromBottom {
		t.Fatal("shape pins not loaded")
	}
	if e := skins.RegisterSkinFS(fsys, p.Name); e != nil {
		t.Fatal(e)
	}
	pack, _ := style.LoadTheme(p.Name)
	for _, scale := range []float32{1, 1.75} {
		lk := style.WithScale(pack.Look(), scale)
		box := paint2d.XYWH(0, 0, 200*scale, 160*scale)
		r, _ := skins.SkinSlotRect(lk, "panel", "stretch", box)
		want := paint2d.XYWH(5*scale, 7*scale, 186*scale, 142*scale)
		if r != want {
			t.Fatal("stretch", r, want)
		}
		edge, _ := skins.SkinSlotRect(lk, "panel", "edge", box)
		if edge != paint2d.XYWH(182*scale, 142*scale, 8*scale, 6*scale) {
			t.Fatal("edge", edge)
		}
		t.Log("17 anchors roundtrip", scale, r, edge)
	}
}

func TestGeneratorFontPreferencesReachRenderedLook(t *testing.T) {
	t.Setenv(style.SystemFontsEnv, "0")
	for _, tc := range []struct {
		name     string
		fonts    *style.FontPrefs
		ui, mono string
	}{
		{"nil", nil, style.FamilyUI, style.FamilyMono},
		{"empty", &style.FontPrefs{}, style.FamilyUI, style.FamilyMono},
		{"ui only", &style.FontPrefs{UI: []string{"ui"}}, style.FamilyUI, style.FamilyMono},
		{"mono only", &style.FontPrefs{Mono: []string{"mono"}}, style.FamilyUI, style.FamilyMono},
		{"case aliases", &style.FontPrefs{UI: []string{"TiTiLlIuM WeB"}, Mono: []string{"JETBRAINS MONO"}}, style.FamilyUI, style.FamilyMono},
		{"ordered role choices", &style.FontPrefs{UI: []string{"raw-audit-missing-ui", "MoNo", "ui"}, Mono: []string{"raw-audit-missing-mono", "UI", "mono"}}, style.FamilyMono, style.FamilyUI},
		{"missing and blank fallback", &style.FontPrefs{UI: []string{"", "raw-audit-missing-ui"}, Mono: []string{" ", "raw-audit-missing-mono"}}, style.FamilyUI, style.FamilyMono},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &skingen.Plan{Name: "raw-fonts-" + strings.ReplaceAll(tc.name, " ", "-"), Base: "dark", Fonts: tc.fonts}
			dir := t.TempDir()
			if err := skingen.Write(dir, p); err != nil {
				t.Fatal(err)
			}
			fsys := os.DirFS(filepath.Join(dir, p.Name))
			raw, err := os.ReadFile(filepath.Join(dir, p.Name, skins.SkinFile))
			if err != nil {
				t.Fatal(err)
			}
			var doc map[string]json.RawMessage
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatal(err)
			}
			if tc.fonts == nil || tc.fonts.Empty() {
				if _, present := doc["fonts"]; present {
					t.Fatal("empty preferences emitted a fonts block")
				}
			} else {
				var got style.FontPrefs
				if err := json.Unmarshal(doc["fonts"], &got); err != nil || !reflect.DeepEqual(got, *tc.fonts) {
					t.Fatal("manifest lost exact preference order/case", got, err)
				}
			}
			sk, err := skins.LoadSkinFS(fsys, p.Name)
			if err != nil {
				t.Fatal(err)
			}
			if tc.fonts != nil {
				want := *tc.fonts
				if tc.name == "missing and blank fallback" {
					// The public loader discards blank family names. Missing
					// names stay as preferences and resolve through fallback.
					want = style.FontPrefs{UI: []string{"raw-audit-missing-ui"}, Mono: []string{"raw-audit-missing-mono"}}
				}
				if !reflect.DeepEqual(sk.Tokens.Fonts, want) {
					t.Fatal("decoded preferences changed", sk.Tokens.Fonts, want)
				}
			}
			if err := skins.RegisterSkin(sk); err != nil {
				t.Fatal(err)
			}
			pack, ok := style.LoadTheme(p.Name)
			if !ok {
				t.Fatal("registered theme not found")
			}
			lk := pack.Look()
			ui, mono := style.FontFor(lk, style.RoleUI), style.FontFor(lk, style.RoleMono)
			if ui.Family != tc.ui || mono.Family != tc.mono {
				t.Fatalf("resolved UI/Mono=%q/%q, want %q/%q", ui.Family, mono.Family, tc.ui, tc.mono)
			}
			t.Logf("17 fonts=%+v resolved UI=%q Mono=%q", tc.fonts, ui.Family, mono.Family)
		})
	}
	for _, raw := range []string{`{"skin":1,"base":"dark","fonts":{"ui":"ui"}}`, `{"skin":1,"base":"dark","fonts":{"serif":["ui"]}}`} {
		if _, err := skins.LoadSkinFS(fstest.MapFS{skins.SkinFile: {Data: []byte(raw)}}, "raw-invalid-fonts"); err == nil {
			t.Fatal("invalid font schema accepted", raw)
		}
	}
}

func TestListPublicActivationDisabledInputAndViewportPress(t *testing.T) {
	l := widgets.NewListView(8, nil, nil)
	l.RowGeo = func(style.LookAndFeel) *widgets.RowGeometry {
		return &widgets.RowGeometry{Rows: paint2d.XYWH(10, 20, 120, 60), Bar: paint2d.XYWH(140, 20, 10, 60), RowHeight: 20}
	}
	l.Arrange(paint2d.XYWH(0, 0, 200, 100))
	l.Selected = 2
	selections, activations, contexts := 0, 0, 0
	l.OnSelect = func(int) { selections++ }
	l.OnActivate = func(int) { activations++ }
	l.OnContext = func(int, paint2d.Point) { contexts++ }
	l.Activate(2)
	l.Activate(2)
	if activations != 2 || selections != 0 {
		t.Fatal("programmatic commits", activations, selections)
	}
	l.SetEnabled(false)
	for _, button := range []platform.MouseButton{platform.ButtonLeft, platform.ButtonRight} {
		if !l.MousePress(widget.MouseEvent{Pos: paint2d.Pt(20, 30), Button: button}) {
			t.Fatal("disabled press not consumed")
		}
	}
	if l.KeyPress(widget.KeyEvent{Key: platform.KeyDown}) || l.KeyPress(widget.KeyEvent{Key: platform.KeyReturn}) || l.AccessibleAction(1, a11y.ActionDefault) {
		t.Fatal("disabled keyboard/accessibility accepted")
	}
	if l.Selected != 2 || activations != 2 || selections != 0 || contexts != 0 {
		t.Fatal("disabled input changed list", l.Selected, activations, selections, contexts)
	}
	l.SetEnabled(true)
	for _, p := range []paint2d.Point{paint2d.Pt(0, 30), paint2d.Pt(135, 30), paint2d.Pt(151, 30), paint2d.Pt(20, 19), paint2d.Pt(20, 80)} {
		if got := l.RowAt(p); got != -1 {
			t.Fatal("outside row hit", p, got)
		}
		l.MousePress(widget.MouseEvent{Pos: p, Button: platform.ButtonLeft})
		if l.Selected != 2 || selections != 0 || activations != 2 {
			t.Fatal("outside row press selected/activated", p, l.Selected, selections, activations)
		}
	}
	t.Log("6 Activate twice commits twice; 22 disabled presses consumed, key/a11y rejected, counts unchanged; 28 outside RowAt and pointer presses agree")
}

func TestFallbackChooserSupportsMultipleGlobs(t *testing.T) {
	entries := []widgets.FileInfo{{Name: "folder", Dir: true}, {Name: "clip.MP3"}, {Name: "song.flac"}, {Name: "song.m4a"}, {Name: "frame-1.PNG"}, {Name: "frame-12.png"}, {Name: "notes.txt"}}
	for _, tc := range []struct {
		filter string
		want   []string
	}{
		{"*.mp3 *.FLAC;*.m4a", []string{"clip.MP3", "folder", "song.flac", "song.m4a"}},
		{"frame-?.png,*.flac", []string{"folder", "frame-1.PNG", "song.flac"}},
		{"[broken,*.mp3", []string{"clip.MP3", "folder"}},
		{"[", []string{"clip.MP3", "folder", "frame-1.PNG", "frame-12.png", "notes.txt", "song.flac", "song.m4a"}},
	} {
		fd := widgets.NewFileDialog(widgets.FileDialogOptions{Path: "/raw-chooser-probe", Filter: tc.filter, OnNavigate: func(string) []widgets.FileInfo { return append([]widgets.FileInfo(nil), entries...) }})
		var got []string
		for _, entry := range fd.Entries() {
			got = append(got, entry.Name)
		}
		sort.Strings(got)
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("filter %q gave %v, want %v", tc.filter, got, tc.want)
		}
		fd.Cancel()
	}
	if got := widgets.FilterPatterns("*.mp3;*.flac,*.m4a *.wav"); !reflect.DeepEqual(got, []string{"*.mp3", "*.flac", "*.m4a", "*.wav"}) {
		t.Fatal("native/fallback shared filter tokens", got)
	}
}
