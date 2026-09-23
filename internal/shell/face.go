// Package shell is the application: one model, three faces, and the switch
// between them.
//
// A face is a way of showing a player. Minim is a compact strip with an
// equaliser and a playlist that snap to its edges; Marquee is a cabinet that
// folds down into a small shaped window; Lantern is a window cut to its
// skin's outline with a playlist anchored beside it. They share no interface
// at all — that is the point of having three — and they share everything
// underneath: the transport, the queue, the equaliser and the analyser live
// here, in the application, and are handed to whichever face is being worn.
//
// So changing face is not a restart. The windows of the old face are closed
// and the windows of the new one are opened around the same model, and the
// track goes on playing from exactly where it was, at the same volume, with
// the same queue in the same order and the same curve on the equaliser.
package shell

import (
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/lantern"
	"github.com/codemodify/media-player-music/internal/players/marquee"
	"github.com/codemodify/media-player-music/internal/players/minim"
	"github.com/codemodify/uitoolkit/app"
)

// The three faces, by id. They are what -face takes, what the face menu
// lists and what is written down for the next run.
const (
	FaceMinim   = "minim"
	FaceMarquee = "marquee"
	FaceLantern = "lantern"
)

// Faces are the faces in the order the face key steps through them.
var Faces = []string{FaceMinim, FaceMarquee, FaceLantern}

// Face is one face of the application.
//
// Everything it is asked to do it does to windows: open them around the
// model it is handed, put the model back into its controls, say where its
// clock should wake, and close them again. Nothing about the model is a
// face's to own, which is what lets one be taken off and another put on
// without the player stopping.
type Face interface {
	// ID is the face's id, and Label what it is called in a menu.
	ID() string
	// Label is the face's name in the face menu.
	Label() string
	// Skins are the packs it was drawn for, in the order its skin key
	// steps through them; the first is the one it opens in.
	Skins() []string
	// Open builds the face's windows around the host's model.
	Open(h *players.Host) error
	// Close takes them away again. The model is untouched.
	Close()
	// MainWindow is the window the application's clock wakes on, and the
	// one a face's own windows hang from.
	MainWindow() *app.Window
	// Windows are all of them, named, for a caller taking a picture of
	// each.
	Windows() []players.NamedWindow
	// Advance is whatever the face does on a tick beyond the model, which
	// the application has already moved: for the two faces with more than
	// one window, looking at where the desktop has put them.
	Advance(dt time.Duration)
	// Refresh puts the model back into the controls.
	Refresh()
	// Pose puts the model at a stated position without a clock, so a
	// screenshot is the same picture every time.
	Pose(pos time.Duration)
}

// ---- the three of them ---------------------------------------------------------

// newFace builds the face with an id, unopened.
func newFace(id string, opts Options) Face {
	switch id {
	case FaceMarquee:
		return &marqueeFace{opts: opts}
	case FaceLantern:
		return &lanternFace{opts: opts}
	default:
		return &minimFace{opts: opts}
	}
}

// minimFace is the compact strip.
type minimFace struct {
	opts Options
	p    *minim.Player
}

func (f *minimFace) ID() string    { return FaceMinim }
func (f *minimFace) Label() string { return "Minim — the compact strip" }
func (f *minimFace) Skins() []string {
	return []string{minim.Skin, minim.SkinClassic, minim.SkinSilver}
}

func (f *minimFace) Open(h *players.Host) error {
	p, err := minim.New(h, minim.Options{
		Headless: f.opts.Headless,
		Scale:    f.opts.Scale,
		NoEq:     f.opts.NoEq,
		NoList:   f.opts.NoList,
	})
	if err != nil {
		return err
	}
	f.p = p
	return nil
}

func (f *minimFace) Close()                         { f.p.Close() }
func (f *minimFace) MainWindow() *app.Window        { return f.p.MainWindow() }
func (f *minimFace) Windows() []players.NamedWindow { return f.p.Windows() }
func (f *minimFace) Advance(dt time.Duration)       { f.p.Advance(dt) }
func (f *minimFace) Refresh()                       { f.p.Refresh() }
func (f *minimFace) Pose(pos time.Duration)         { f.p.Pose(pos) }
func (f *minimFace) Player() *minim.Player          { return f.p }

// marqueeFace is the cabinet that folds.
type marqueeFace struct {
	opts Options
	p    *marquee.Player
}

func (f *marqueeFace) ID() string      { return FaceMarquee }
func (f *marqueeFace) Label() string   { return "Marquee — the cabinet that folds" }
func (f *marqueeFace) Skins() []string { return []string{marquee.Skin} }

func (f *marqueeFace) Open(h *players.Host) error {
	p, err := marquee.New(h, marquee.Options{
		Headless: f.opts.Headless,
		Scale:    f.opts.Scale,
		Compact:  f.opts.Compact,
	})
	if err != nil {
		return err
	}
	f.p = p
	return nil
}

func (f *marqueeFace) Close()                         { f.p.Close() }
func (f *marqueeFace) MainWindow() *app.Window        { return f.p.MainWindow() }
func (f *marqueeFace) Windows() []players.NamedWindow { return f.p.Windows() }
func (f *marqueeFace) Advance(dt time.Duration)       { f.p.Advance(dt) }
func (f *marqueeFace) Refresh()                       { f.p.Refresh() }
func (f *marqueeFace) Pose(pos time.Duration)         { f.p.Pose(pos) }
func (f *marqueeFace) Player() *marquee.Player        { return f.p }

// lanternFace is the shaped window with the playlist beside it.
type lanternFace struct {
	opts Options
	p    *lantern.Player
}

func (f *lanternFace) ID() string      { return FaceLantern }
func (f *lanternFace) Label() string   { return "Lantern — the shaped window" }
func (f *lanternFace) Skins() []string { return []string{lantern.Skin} }

func (f *lanternFace) Open(h *players.Host) error {
	p, err := lantern.New(h, lantern.Options{
		Headless: f.opts.Headless,
		Scale:    f.opts.Scale,
		NoList:   f.opts.NoList,
		Themed:   h.Worn() != lantern.Skin,
	})
	if err != nil {
		return err
	}
	f.p = p
	return nil
}

func (f *lanternFace) Close()                         { f.p.Close() }
func (f *lanternFace) MainWindow() *app.Window        { return f.p.MainWindow() }
func (f *lanternFace) Windows() []players.NamedWindow { return f.p.Windows() }
func (f *lanternFace) Advance(dt time.Duration)       { f.p.Advance(dt) }
func (f *lanternFace) Refresh()                       { f.p.Refresh() }
func (f *lanternFace) Pose(pos time.Duration)         { f.p.Pose(pos) }
func (f *lanternFace) Player() *lantern.Player        { return f.p }
