package amarok

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/layout"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

type body struct {
	widget.Base
	p            *Player
	slots        *skins.Slots
	buttons      map[string]*players.GlyphButton
	play         *players.GlyphButton
	seek, volume *sourceSlider
	search       *widgets.TextField
	list         *players.QueueView
	headers      [4]*players.GlyphButton
	eq           *eqPane
	analyser     *players.AnalyserView
	grip         *resizeGrip
	query        string
	sortColumn   int
	descending   bool
	queueRect    paint2d.Rect
	searchLook   style.LookAndFeel
}

func newBody(p *Player) *body {
	b := &body{p: p, slots: skins.NewSlots(LayoutMain), buttons: make(map[string]*players.GlyphButton), sortColumn: -1}
	b.Init(b)
	b.SetManagesChildren(true)
	b.SetAccessibleName("Amarok 1.4 music player")
	button := func(name, label string, glyph players.Glyph, action func()) *players.GlyphButton {
		c := iconButton(p, name, label, glyph, action)
		b.buttons[name] = c
		b.slots.Bind(name, c)
		b.Add(c)
		return c
	}
	b.play = button("play", "Play", players.GlyphPlay, func() {
		if p.Transport.State == players.Stopped {
			p.Host.Play()
		} else {
			p.Host.Command(players.CmdPlayPause)
		}
	})
	button("prev", "Previous track", players.GlyphPrev, func() { p.Host.Command(players.CmdPrev) })
	button("next", "Next track", players.GlyphNext, func() { p.Host.Command(players.CmdNext) })
	button("stop", "Stop", players.GlyphStop, func() { p.Host.Command(players.CmdStop) })
	button("open", "Open music files", players.GlyphEject, func() { p.Host.OpenFiles(b.buttons["open"], false) })
	button("add", "Add music files", players.GlyphEject, func() { p.Host.OpenFiles(b.buttons["add"], true) })
	button("remove", "Remove the playing track", players.GlyphNone, p.Host.RemoveTrack)
	button("eq", "Show or hide equalizer", players.GlyphSliders, func() { p.ShowEqualiser(!p.Host.Model.EqualizerVisible) }).Toggle = true
	button("playlist", "Show or hide playlist", players.GlyphList, func() { p.ShowPlaylist(!p.Host.Model.PlaylistVisible) }).Toggle = true
	button("appearance", "Appearance", players.GlyphSkin, func() { p.Host.ContextMenu(b.buttons["appearance"], menuAt(b.buttons["appearance"])) })
	button("settings", "Playback settings", players.GlyphNone, p.Host.OpenSettings)
	button("shuffle", "Shuffle", players.GlyphShuffle, func() { p.Host.Command(players.CmdShuffle) }).Toggle = true
	button("repeat", "Repeat", players.GlyphRepeat, func() { p.Host.Command(players.CmdRepeat) }).Toggle = true
	for _, caption := range []struct {
		name, label string
		kind        style.CaptionButton
		action      func()
	}{{"close", "Close to tray", style.CaptionClose, p.Main.RequestClose}, {"minimize", "Minimize Music Player", style.CaptionMinimize, p.Main.Minimize}} {
		c := button(caption.name, caption.label, players.GlyphNone, caption.action)
		kind := caption.kind
		c.Art = func(ctx *paint2d.Context, r paint2d.Rect, st style.ControlState) bool {
			bevel(ctx, r, st.Pressed(), style.Dip(c.Look(), 1))
			style.DrawCaptionGlyph(ctx, r.Inset(style.Dip(c.Look(), 4)), kind, false, ink, style.Dip(c.Look(), 9), style.Dip(c.Look(), 1))
			return true
		}
	}
	b.seek = slider(false, 0, 1, 0, "Seek", p.Host.Seek)
	b.seek.Step, b.seek.Page = .02, .1
	b.seek.Format = func(v float32) string {
		return players.Clock(time.Duration(float64(p.Transport.Length()) * float64(v)))
	}
	b.volume = slider(false, 0, 100, p.Transport.Volume*100, "Volume", func(v float32) { p.Host.SetVolume(v / 100) })
	b.volume.Step = 2
	b.volume.Page = 10
	b.volume.Format = func(v float32) string { return fmt.Sprintf("%d%%", int(v+.5)) }
	for _, f := range []struct {
		name string
		c    widget.Component
	}{{"seek", b.seek}, {"volume", b.volume}} {
		b.slots.Bind(f.name, f.c)
		b.Add(f.c)
	}
	b.search = widgets.NewTextField("", "Playlist Search", nil)
	b.search.Clearable = true
	b.search.SetAccessibleName("Search playlist")
	b.search.OnInput = func(query string) { b.query = query; b.list.InterruptActivation(); b.list.OffsetY = 0; b.syncRows() }
	b.search.OnEscape = func() { b.search.SetText(""); b.query = ""; b.syncRows(); p.Main.RequestFocus(b.list) }
	b.search.OnSubmit = func(string) { p.ShowPlaylist(true); p.Main.RequestFocus(b.list) }
	b.slots.Bind("search", b.search)
	b.Add(b.search)
	b.list = players.NewQueueView(p.Host)
	b.list.SetAccessibleName("Amarok playlist")
	b.list.RowGeo = func(lk style.LookAndFeel) *widgets.RowGeometry {
		r := b.list.LocalBounds()
		d := style.Dip(lk, 1)
		return &widgets.RowGeometry{Rows: paint2d.XYWH(d, d, max(r.Dx()-16*d, 0), max(r.Dy()-2*d, 0)), Bar: paint2d.XYWH(r.Dx()-14*d, d, 13*d, max(r.Dy()-2*d, 0)), RowHeight: 19 * d}
	}
	b.list.RowPaint = b.paintRow
	b.list.ScrollPaint = b.paintScroll
	b.list.OnContext = func(_ int, _ paint2d.Point) { b.contextMenu(b.list) }
	b.Add(b.list)
	for i, title := range []string{"Title", "Artist", "Album", "Length"} {
		column, label := i, title
		c := textButton(p, label, func() {
			b.list.InterruptActivation()
			if b.sortColumn == column {
				b.descending = !b.descending
			} else {
				b.sortColumn = column
				b.descending = false
			}
			b.syncRows()
			b.Invalidate()
		})
		b.headers[i] = c
		b.Add(c)
	}
	b.eq = newEqPane(p)
	b.Add(b.eq)
	b.analyser = players.NewAnalyserView(p.Spectrum)
	b.analyser.Tint = paint2d.RGB(.24, .25, .32)
	b.analyser.Peaks = false
	b.slots.Bind("analyser", b.analyser)
	b.Add(b.analyser)
	for _, tab := range []struct {
		name, label string
		action      func()
	}{{"context", "Context", func() { p.ShowEqualiser(false) }}, {"equalizer", "Equalizer", func() { p.ShowEqualiser(true) }}} {
		name, label := tab.name, tab.label
		c := iconButton(p, name, label, players.GlyphNone, tab.action)
		c.Toggle = true
		c.Art = func(ctx *paint2d.Context, r paint2d.Rect, st style.ControlState) bool {
			col := ink
			if st.Checked() {
				ctx.DrawRect(r, paint2d.Fill(blue))
				col = paint2d.White
			} else {
				bevel(ctx, r, st.Pressed(), style.Dip(c.Look(), 1))
			}
			ctx.Save()
			ctx.Translate(r.Min.X, r.Max.Y)
			ctx.Rotate(-math.Pi / 2)
			players.DrawTextIn(ctx, font(c.Look(), 11), paint2d.XYWH(0, 0, r.Dy(), r.Dx()), label, col, style.AlignCenter)
			ctx.Restore()
			return true
		}
		b.buttons["tab."+name] = c
		b.Add(c)
	}
	b.grip = &resizeGrip{p: p}
	b.grip.Init(b.grip)
	b.slots.Bind("resize", b.grip)
	b.Add(b.grip)
	return b
}
func (b *body) sync() {
	t := b.p.Transport
	lk := b.Look()
	if b.searchLook != lk {
		b.search.SetLook(&fieldLook{lk})
		b.searchLook = lk
	}
	if !b.seek.Dragging() {
		b.seek.Set(t.Fraction())
	}
	b.seek.SetEnabled(t.Length() > 0)
	if !b.volume.Dragging() {
		b.volume.Set(t.Volume * 100)
	}
	b.play.SetGlyph(players.GlyphPlay, "Play")
	if t.State == players.Playing {
		b.play.SetGlyph(players.GlyphPause, "Pause")
	}
	b.buttons["shuffle"].SetChecked(t.Random)
	b.buttons["repeat"].SetChecked(t.Repeat != players.RepeatOff)
	b.buttons["eq"].SetChecked(b.p.Host.Model.EqualizerVisible)
	b.buttons["playlist"].SetChecked(b.p.Host.Model.PlaylistVisible)
	b.buttons["tab.context"].SetChecked(!b.p.Host.Model.EqualizerVisible)
	b.buttons["tab.equalizer"].SetChecked(b.p.Host.Model.EqualizerVisible)
	b.buttons["settings"].SetEnabled(b.p.Host.ShowSettings != nil)
	b.eq.SetVisible(b.p.Host.Model.EqualizerVisible)
	b.list.SetVisible(b.p.Host.Model.PlaylistVisible)
	for _, h := range b.headers {
		h.SetVisible(b.p.Host.Model.PlaylistVisible)
	}
	b.eq.sync()
	b.syncRows()
	b.analyser.Invalidate()
	b.Invalidate()
}
func (b *body) syncRows() {
	if b.list == nil {
		return
	}
	q := b.p.Transport.List
	ids := q.Find(b.query)
	if ids == nil {
		ids = make([]int, q.Len())
		for i := range ids {
			ids[i] = i
		}
	}
	if b.sortColumn >= 0 {
		sort.SliceStable(ids, func(i, j int) bool {
			a, c := q.At(ids[i]), q.At(ids[j])
			less := false
			if b.sortColumn == 3 {
				less = a.Length < c.Length
				if b.descending {
					return a.Length > c.Length
				}
				return less
			}
			av, cv := columnValues(a), columnValues(c)
			if b.descending {
				return strings.ToLower(av[b.sortColumn]) > strings.ToLower(cv[b.sortColumn])
			}
			return strings.ToLower(av[b.sortColumn]) < strings.ToLower(cv[b.sortColumn])
		})
	}
	b.list.SyncRows(ids)
}
func (b *body) Measure(c layout.Constraints) paint2d.Point {
	sz, _ := b.slots.Measure(b.Look(), c)
	return sz
}
func (b *body) Arrange(r paint2d.Rect) {
	b.SetBounds(r)
	lk, box := b.Look(), b.LocalBounds()
	b.slots.Arrange(lk, box)
	d := func(n float32) float32 { return style.Dip(lk, n) }
	q := slot(lk, "queue", box)
	eq := slot(lk, "equalizer", box)
	if b.eq.Visible() {
		b.eq.Arrange(eq)
		q.Max.Y = eq.Min.Y - d(6)
	} else {
		b.eq.Arrange(paint2d.Rect{})
	}
	b.queueRect = q
	rows := q
	rows.Min.Y += d(24)
	b.list.Arrange(rows)
	head := paint2d.XYWH(q.Min.X, q.Min.Y, q.Dx()-d(15), d(24))
	for i, cell := range columnRects(head, lk) {
		b.headers[i].Arrange(cell)
	}
	rail := slot(lk, "rail", box)
	b.buttons["tab.context"].Arrange(paint2d.XYWH(rail.Min.X, rail.Min.Y, rail.Dx(), d(94)))
	b.buttons["tab.equalizer"].Arrange(paint2d.XYWH(rail.Min.X, rail.Min.Y+d(98), rail.Dx(), d(106)))
}
func (b *body) Paint(ctx *paint2d.Context) {
	lk, box := b.Look(), b.LocalBounds()
	d := func(n float32) float32 { return style.Dip(lk, n) }
	ctx.DrawRect(box, paint2d.Fill(chrome))
	ctx.DrawRect(box.Inset(d(.5)), paint2d.StrokePaint(border, d(1)))
	caption := slot(lk, "caption", box)
	skins.DrawSkinSlot(lk, ctx, caption, LayoutMain, "caption", 0)
	skins.DrawSkinSprite(lk, ctx, paint2d.XYWH(d(8), d(5), d(16), d(16)), "app.icon", paint2d.Color{})
	players.DrawTextIn(ctx, font(lk, 12), slot(lk, "title", box), "Music Player", paint2d.White, style.AlignStart)
	players.DrawTextIn(ctx, font(lk, 11), paint2d.XYWH(d(247), d(34), d(57), d(24)), "Search:", ink, style.AlignEnd)
	ctx.DrawRect(paint2d.XYWH(d(5), d(64), box.Dx()-d(10), d(1)), paint2d.Fill(border))
	b.paintContext(ctx, slot(lk, "context", box))
	ctx.DrawRect(b.queueRect, paint2d.Fill(paint2d.White))
	ctx.DrawRect(b.queueRect.Inset(d(.5)), paint2d.StrokePaint(border, d(1)))
	if !b.list.Visible() {
		players.DrawTextIn(ctx, font(lk, 12), b.queueRect, "Playlist hidden · Ctrl+L to show", muted, style.AlignCenter)
	}
	footer := paint2d.XYWH(d(6), box.Dy()-d(96), box.Dx()-d(12), d(1))
	ctx.DrawRect(footer, paint2d.Fill(border))
	volume := slot(lk, "volume", box)
	players.DrawTextIn(ctx, font(lk, 10), paint2d.XYWH(volume.Min.X, volume.Min.Y-d(6), volume.Dx(), d(12)), fmt.Sprintf("Volume %d%%", int(b.p.Transport.Volume*100+.5)), muted, style.AlignEnd)
	seek := slot(lk, "seek", box)
	players.DrawTextIn(ctx, font(lk, 11), paint2d.XYWH(d(7), seek.Min.Y, d(38), seek.Dy()), players.Clock(b.p.Transport.Pos), ink, style.AlignStart)
	players.DrawTextIn(ctx, font(lk, 11), paint2d.XYWH(seek.Max.X+d(6), seek.Min.Y, d(108), seek.Dy()), "−"+players.Clock(max(b.p.Transport.Length()-b.p.Transport.Pos, 0)), ink, style.AlignEnd)
	status := slot(lk, "status", box)
	text := b.p.Host.StatusText()
	if b.p.Host.Model.Error == "" && b.p.Host.Model.Live {
		text = fmt.Sprintf("%s: %s · %d tracks (%s)", b.p.Transport.State, b.p.Transport.Track().Label(), b.p.Transport.List.Len(), players.Clock(b.p.Transport.List.Total()))
	}
	players.DrawTextIn(ctx, font(lk, 10), status, text, ink, style.AlignStart)
}
func (b *body) paintContext(ctx *paint2d.Context, r paint2d.Rect) {
	lk := b.Look()
	d := func(n float32) float32 { return style.Dip(lk, n) }
	t := b.p.Transport.Track()
	ctx.Save()
	ctx.ClipRect(r)
	defer ctx.Restore()
	ctx.DrawRect(r, paint2d.Fill(paint2d.White))
	ctx.DrawRect(r.Inset(d(.5)), paint2d.StrokePaint(border, d(1)))
	heading := paint2d.XYWH(r.Min.X+d(2), r.Min.Y+d(2), r.Dx()-d(4), d(23))
	ctx.DrawRect(heading, paint2d.Fill(paint2d.RGB(.86, .88, .96)))
	players.DrawTextIn(ctx, font(lk, 11), heading.Inset(d(4)), "Current track", ink, style.AlignStart)
	x, y, w := r.Min.X+d(9), r.Min.Y+d(33), r.Dx()-d(18)
	for _, line := range []string{t.Title, t.Artist, t.Album} {
		players.DrawTextIn(ctx, font(lk, 11), paint2d.XYWH(x, y, w, d(20)), line, ink, style.AlignStart)
		y += d(20)
	}
	coverW := d(133)
	skins.DrawSkinSprite(lk, ctx, paint2d.XYWH(x+(w-coverW)/2, y+d(6), coverW, d(120)), "cover.empty", paint2d.Color{})
	y += d(132)
	players.DrawTextIn(ctx, font(lk, 10), paint2d.XYWH(x, y, w, d(18)), "No cover artwork", muted, style.AlignCenter)
	y += d(26)
	lines := []string{fmt.Sprintf("%s · %s", b.p.Transport.State, players.Clock(t.Length))}
	if t.Rate > 0 {
		lines = append(lines, fmt.Sprintf("%d kHz · %d channels", t.Rate, t.Channels))
	}
	if t.Bitrate > 0 {
		lines = append(lines, fmt.Sprintf("%d kbps", t.Bitrate/1000))
	}
	for _, line := range lines {
		if line == "" {
			continue
		}
		players.DrawTextIn(ctx, font(lk, 11), paint2d.XYWH(x, y, w, d(20)), line, muted, style.AlignStart)
		y += d(20)
	}
}
func (b *body) paintRow(ctx *paint2d.Context, row paint2d.Rect, i int, st style.ControlState) bool {
	lk := b.list.Look()
	col, bg := ink, paint2d.White
	if i%2 == 1 {
		bg = paint2d.RGB(.86, .89, .98)
	}
	if i == b.list.Selected {
		col, bg = paint2d.White, paint2d.RGB(.54, .54, .72)
	}
	ctx.DrawRect(row, paint2d.Fill(bg))
	if i == b.list.Selected && st.Focused() {
		lk.DrawFocusRing(ctx, row)
	}
	values := columnValues(b.p.Transport.List.At(b.list.QueueIndex(i)))
	for column, cell := range columnRects(row, lk) {
		align := style.AlignStart
		if column == 3 {
			align = style.AlignEnd
		}
		if column == 0 && b.list.QueueIndex(i) == b.p.Transport.List.Index() {
			players.DrawGlyph(ctx, paint2d.XYWH(cell.Min.X, cell.Min.Y, style.Dip(lk, 15), cell.Dy()), players.GlyphPlay, col, style.Dip(lk, 1))
			cell.Min.X += style.Dip(lk, 15)
		}
		players.DrawTextIn(ctx, font(lk, 11), cell.Inset(style.Dip(lk, 3)), values[column], col, align)
	}
	return true
}
func (b *body) paintScroll(ctx *paint2d.Context, track, thumb paint2d.Rect, st style.ControlState) bool {
	ctx.DrawRect(track, paint2d.Fill(paint2d.RGB(.88, .88, .93)))
	bevel(ctx, thumb, st.Pressed(), style.Dip(b.Look(), 1))
	return true
}
func columnValues(t players.Track) [4]string {
	clock := "--:--"
	if t.Length > 0 {
		clock = players.Clock(t.Length)
	}
	return [4]string{t.Title, t.Artist, t.Album, clock}
}
func columnRects(row paint2d.Rect, lk style.LookAndFeel) [4]paint2d.Rect {
	timeW := style.Dip(lk, 55)
	remaining := max(row.Dx()-timeW, 0)
	widths := [4]float32{remaining * .45, remaining * .27, remaining * .28, timeW}
	var out [4]paint2d.Rect
	x := row.Min.X
	for i, w := range widths {
		out[i] = paint2d.XYWH(x, row.Min.Y, w, row.Dy())
		x += w
	}
	return out
}
func (b *body) contextMenu(from widget.Component) {
	if from == nil {
		from = b
	}
	widgets.ShowContextMenu(from, menuAt(from), widgets.Item("Open music files…", func() { b.p.Host.OpenFiles(from, false) }), widgets.Item("Add music files…", func() { b.p.Host.OpenFiles(from, true) }), widgets.Item("Remove the playing track", b.p.Host.RemoveTrack), widgets.Sep(), widgets.Item("Appearance…", func() { b.p.Host.ContextMenu(from, menuAt(from)) }), widgets.Item("Audio backend settings…", b.p.Host.OpenSettings))
}
func (b *body) KeyPress(e widget.KeyEvent) bool { return b.p.keys(e) }
func (b *body) MousePress(e widget.MouseEvent) bool {
	if e.Button == platform.ButtonRight {
		b.list.InterruptActivation()
		b.contextMenu(b)
		return true
	}
	if e.Button == platform.ButtonLeft && e.Pos.Y < style.Dip(b.Look(), 26) {
		b.p.restore = nil
		return b.p.Main.StartMove()
	}
	return false
}
func (b *body) DropTypes() []string { return []string{"text/uri-list"} }
func (b *body) DropActionFor(offered platform.DragAction) platform.DragAction {
	return offered & platform.DragCopy
}
func (b *body) Drop(e widget.DropEvent) bool { return b.p.Host.AcceptFileDrop(e) }
func (b *body) Describe(n *a11y.Node) {
	n.Role, n.Name, n.Description = a11y.RolePane, "Amarok 1.4 music player", b.p.Host.StatusText()
}

type resizeGrip struct {
	widget.Base
	p *Player
}

func (g *resizeGrip) Measure(c layout.Constraints) paint2d.Point {
	return c.Constrain(paint2d.Pt(style.Dip(g.Look(), 17), style.Dip(g.Look(), 17)))
}
func (g *resizeGrip) Arrange(r paint2d.Rect) { g.SetBounds(r) }
func (g *resizeGrip) Paint(ctx *paint2d.Context) {
	r := g.LocalBounds()
	d := style.Dip(g.Look(), 1)
	for i := float32(3); i < 14; i += 4 {
		ctx.DrawLine(paint2d.Pt(r.Max.X-i*d, r.Max.Y-2*d), paint2d.Pt(r.Max.X-2*d, r.Max.Y-i*d), paint2d.StrokePaint(border, d))
	}
}
func (g *resizeGrip) MousePress(e widget.MouseEvent) bool {
	if e.Button != platform.ButtonLeft {
		return false
	}
	g.p.restore = nil
	return g.p.Main.StartResize(platform.EdgeBottom | platform.EdgeRight)
}
