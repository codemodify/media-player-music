package players

import (
	"time"

	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/app"
	"github.com/codemodify/uitoolkit/style"
	"github.com/codemodify/uitoolkit/widget"
	"github.com/codemodify/uitoolkit/widgets"
)

// The one model, and what a face is given to work against.
//
// The application has three faces and one of everything else. A face is a
// way of *showing* a player — a compact strip, a cabinet that folds, a
// window cut to its outline — and none of them owns the thing being shown:
// the transport, the queue, the equaliser and the analyser are the
// application's, and they are handed to whichever face is being worn. That
// is what makes changing face a change of face rather than a restart:
// nothing is rebuilt but the windows, and the track goes on playing from
// where it was.

// Model is the model every face shares.
type Model struct {
	Transport *Transport
	Spectrum  *Spectrum
	Equaliser *Equalizer
}

// NewModel is the application's model at rest: the invented library, a
// stopped transport at the head of it, an analyser and a flat equaliser.
//
// The analyser has as many bands as the widest face draws. A face with a
// narrower window shows fewer of them and averages the ones it drops
// ([AnalyserView]), so one spectrum serves all three and its peak markers
// do not restart when the face changes.
func NewModel() *Model {
	s := &Model{
		Transport: NewTransport(NewLibrary()),
		Spectrum:  NewSpectrum(48),
		Equaliser: NewEqualizer(),
	}
	return s
}

// Themed is the pack a skin is dropped for: an ordinary theme with no art
// in it at all, which is the point — the same app, the same widget tree,
// the same keyboard, in a look anybody's desktop already has.
const Themed = "breeze-night"

// Host is what a face is given: the application, the one model, and the few
// things only the application can do — run a transport command with the
// clock behind it, answer the keys every face shares, change the skin, and
// change the face.
//
// Every hook is optional. A face handed a bare host (NewHost) is a face
// running on its own, which is what its own tests do to it.
type Host struct {
	App   *app.Application
	Model *Model

	// Run is the application's transport command: the model, and the clock
	// started again behind it. Nil runs the command on the model alone.
	Run func(Command)
	// Shared is the keyboard every face answers to — the transport table,
	// the face switch, Escape — tried after the face's own keys.
	Shared func(w *app.Window, e widget.KeyEvent) bool
	// Packs is the list of packs the face being worn was drawn for, in the
	// order a skin key steps through them; the first is its own.
	Packs func() []string
	// Wrote is told about a skin the user chose, so the application can
	// write it down for the next run. Nil remembers nothing.
	Wrote func(pack string)
	// Faces, Face and SetFace are the face switch: what there is, what is
	// being worn, and putting on another one.
	Faces   func() []string
	Face    func() string
	SetFace func(id string)
	// FaceLabel is what a face is called in a menu.
	FaceLabel func(id string) string

	// lastSkin is the skin DropSkin puts back.
	lastSkin string
}

// NewHost is a host with a model of its own and no application behind it:
// a face on its own, as a test opens one.
func NewHost(a *app.Application) *Host { return &Host{App: a, Model: NewModel()} }

// Command runs a transport command through the application, so the clock is
// running behind whatever it started.
func (h *Host) Command(c Command) {
	if h.Run != nil {
		h.Run(c)
		return
	}
	h.Model.Transport.Do(c)
}

// Keys is the shared keyboard, for a face that has tried its own keys first.
func (h *Host) Keys(w *app.Window, e widget.KeyEvent) bool {
	if h.Shared != nil {
		return h.Shared(w, e)
	}
	if Typing(w.Focus()) {
		return false
	}
	if c := CommandFor(e); c != CmdNone {
		h.Command(c)
		return true
	}
	return false
}

// Pose puts the model at a stated position with the analyser advanced to
// match, and touches no clock at all: the same position is the same picture
// every time, which is what makes one screenshot comparable with the last.
func (h *Host) Pose(pos time.Duration) {
	t := h.Model.Transport
	t.State = Playing
	t.Pos = pos
	h.Model.Spectrum.Reset()
	h.Model.Spectrum.Advance(pos, true, 250*time.Millisecond)
}

// ---- the skin ----------------------------------------------------------------

// Skins are the packs the face being worn was drawn for, in the order the
// skin key steps through them.
func (h *Host) Skins() []string {
	if h.Packs == nil {
		return nil
	}
	return h.Packs()
}

// Worn is the pack the application is wearing now.
func (h *Host) Worn() string { return style.LookAppearance(h.App.Look()).Name }

// Skinned reports whether what is worn is one of the face's own skins
// rather than an ordinary theme.
func (h *Host) Skinned() bool { return h.IsSkin(h.Worn()) }

// IsSkin reports whether a pack is one of the face's own.
func (h *Host) IsSkin(id string) bool {
	for _, s := range h.Skins() {
		if s == id {
			return true
		}
	}
	return false
}

// SetSkin puts the application in a pack — one of the face's skins, or any
// other — and remembers the choice for the next run.
//
// It is the call Settings makes for any theme: a skin is a pack, so
// choosing one is choosing the appearance's pack id and rebuilding the
// look. Every window follows, because that is what Application.SetLook
// does, and nothing in any widget tree is rebuilt.
func (h *Host) SetSkin(id string) {
	if id == "" || h.App == nil {
		return
	}
	if h.IsSkin(id) {
		h.lastSkin = id
	}
	ap := style.LookAppearance(h.App.Look())
	ap.FollowDesktop = false
	ap.Name = id
	h.App.SetLook(style.WithAppearance(h.App.Look(), ap))
	if h.Wrote != nil {
		h.Wrote(id)
	}
}

// NextSkin steps to the next of the face's skins, and from anything else
// back to the first.
func (h *Host) NextSkin() {
	skins := h.Skins()
	if len(skins) == 0 {
		return
	}
	cur := h.Worn()
	for i, id := range skins {
		if id == cur {
			h.SetSkin(skins[(i+1)%len(skins)])
			return
		}
	}
	h.SetSkin(skins[0])
}

// DropSkin drops the skin for the themed fallback, or puts back the skin
// that was dropped.
func (h *Host) DropSkin() {
	if h.Skinned() {
		h.SetSkin(Themed)
		return
	}
	last := h.lastSkin
	if last == "" {
		if skins := h.Skins(); len(skins) > 0 {
			last = skins[0]
		}
	}
	h.SetSkin(last)
}

// ---- the face ------------------------------------------------------------------

// FaceIDs are the faces the application has, and FaceID the one being worn.
func (h *Host) FaceIDs() []string {
	if h.Faces == nil {
		return nil
	}
	return h.Faces()
}

// FaceID is the face being worn, or "" for a face running on its own.
func (h *Host) FaceID() string {
	if h.Face == nil {
		return ""
	}
	return h.Face()
}

// WearFace puts on another face. The model is untouched, so the queue, the
// position and the equaliser's curve are the same on the other side of it.
func (h *Host) WearFace(id string) {
	if h.SetFace != nil {
		h.SetFace(id)
	}
}

// LabelOfFace is what a face is called in a menu.
func (h *Host) LabelOfFace(id string) string {
	if h.FaceLabel != nil {
		return h.FaceLabel(id)
	}
	return id
}

// NamedWindow is one of a face's windows with the name a screenshot is
// written under.
type NamedWindow struct {
	Name   string
	Window *app.Window
}

// ---- the menus the faces open ----------------------------------------------------

// PackLabel is what a pack is called in a menu: the label the pack itself
// states, and its id where it states none.
func (h *Host) PackLabel(id string) string {
	if id == Themed {
		return "No skin"
	}
	if sk, ok := style.LoadSkin(id); ok && sk.Label != "" {
		return sk.Label
	}
	if th, ok := style.LoadTheme(id); ok && th.Label != "" {
		return th.Label
	}
	return id
}

// FaceItems are the application's faces as menu rows, the one being worn
// ticked. Any face that opens a menu can offer them, and every face offers
// them somewhere: a switch only the keyboard can reach is not finished.
func (h *Host) FaceItems() []*widgets.MenuItem {
	cur := h.FaceID()
	var out []*widgets.MenuItem
	for _, id := range h.FaceIDs() {
		id := id
		out = append(out, widgets.RadioItem(h.LabelOfFace(id), "face", id == cur, func() { h.WearFace(id) }))
	}
	return out
}

// SkinItems are the packs the face being worn was drawn for, and the themed
// fallback under them, as menu rows with the one being worn ticked.
func (h *Host) SkinItems() []*widgets.MenuItem {
	cur := h.Worn()
	var out []*widgets.MenuItem
	for _, id := range append(append([]string(nil), h.Skins()...), Themed) {
		id := id
		out = append(out, widgets.RadioItem(h.PackLabel(id), "skin", id == cur, func() { h.SetSkin(id) }))
	}
	return out
}

// ContextMenu opens the faces over the skins at a point in a window: what a
// right-click on the face of a player offers, wherever there is room for it.
func (h *Host) ContextMenu(from widget.Component, at paintengine2d.Point) {
	items := h.FaceItems()
	if skins := h.SkinItems(); len(skins) > 0 {
		if len(items) > 0 {
			items = append(items, widgets.Sep())
		}
		items = append(items, skins...)
	}
	if len(items) == 0 {
		return
	}
	widgets.ShowContextMenu(from, at, items...)
}
