package shell

import (
	"fmt"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/uitoolkit/app"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/widget"
)

// Options is what the command line passes in.
type Options struct {
	// Face is the face to open in, by id. Empty takes the one remembered
	// from the last run, and failing that the first.
	Face string
	// Skin is the pack to wear, whatever the face would have chosen.
	// Empty takes the one remembered for that face, and failing that the
	// face's own.
	Skin string
	// Remember writes the face and the skin down for the next run.
	Remember bool

	Headless bool
	Scale    float32

	// NoEq and NoList close the panels the two multi-window faces have;
	// Compact opens Marquee folded down.
	NoEq, NoList bool
	Compact      bool
}

// Shell is the application: the model, the faces, and the one clock.
type Shell struct {
	App  *app.Application
	Host *players.Host

	opts     Options
	settings players.Settings
	faces    map[string]Face
	face     Face
	// wearing is the id of the face being put on, which is set before its
	// windows are built: the skin is chosen for the face that is coming,
	// not the one that is going.
	wearing string
	pulse   *players.Pulse
	// switching guards against a face switch asked for from inside a face
	// — a menu row of its own — reentering while its windows are closing.
	switching bool
}

// New builds the application and opens it in a face.
//
// The look is not set here. A pack is chosen when the application is built
// (style.ThemeEnv), which is before this, so the caller reads [SkinFor] and
// sets it; what this does is keep the two in step from then on.
func New(a *app.Application, opts Options) (*Shell, error) {
	s := &Shell{
		App:      a,
		opts:     opts,
		settings: players.LoadSettings(),
		faces:    map[string]Face{},
	}
	s.Host = &players.Host{
		App:   a,
		Model: players.NewModel(),

		Run:    s.Command,
		Shared: s.Keys,
		Packs:  s.packs,
		Wrote:  s.wrote,
		Faces:  func() []string { return Faces },
		Face:   func() string { return s.wearing },
		// A face asked for from inside a face — a row of its own menu —
		// is put on at the next turn of the loop rather than under the
		// menu that asked for it: the switch closes the window the menu
		// is standing in.
		SetFace:   func(id string) { a.Post(func() { _ = s.SetFace(id) }) },
		FaceLabel: s.faceLabel,
	}
	// One clock for the whole application, not one per face and not one
	// per window: three windows each waking on their own would advance the
	// same transport three times in a frame.
	s.pulse = players.NewPulse(70*time.Millisecond, s.tick)

	id := opts.Face
	if id == "" {
		id = s.settings.Face
	}
	if !known(id) {
		id = Faces[0]
	}
	if err := s.wear(id); err != nil {
		return nil, err
	}
	return s, nil
}

// SkinFor is the pack a face opens in: the one asked for on the command
// line, else the one remembered for that face, else the face's own first
// skin. It is read before the application is built, so the look it is built
// with is the one the face wants.
func SkinFor(id string, opts Options) string {
	if !known(id) {
		id = Faces[0]
	}
	if opts.Skin != "" {
		return opts.Skin
	}
	if pack := players.LoadSettings().SkinOf(id); pack != "" {
		return pack
	}
	return newFace(id, opts).Skins()[0]
}

// FaceFor is the face the application opens in, by the same rule: the one
// asked for, else the one remembered, else the first.
func FaceFor(opts Options) string {
	id := opts.Face
	if id == "" {
		id = players.LoadSettings().Face
	}
	if !known(id) {
		id = Faces[0]
	}
	return id
}

func known(id string) bool {
	for _, f := range Faces {
		if f == id {
			return true
		}
	}
	return false
}

// Face is the face being worn, and FaceID its id.
func (s *Shell) Face() Face { return s.face }

// FaceID is the id of the face being worn.
func (s *Shell) FaceID() string { return s.wearing }

// Start begins the clock and starts the transport.
func (s *Shell) Start() {
	s.Host.Model.Transport.Play()
	s.pulse.Start(s.face.MainWindow())
}

// Pulse is the clock, for a test that drives it by hand.
func (s *Shell) Pulse() *players.Pulse { return s.pulse }

// Pose puts the model at a stated position with no clock involved, so a
// screenshot is the same picture every time it is taken.
func (s *Shell) Pose(pos time.Duration) { s.face.Pose(pos) }

// Windows are the windows of the face being worn, named.
func (s *Shell) Windows() []players.NamedWindow { return s.face.Windows() }

// SetFace puts on another face.
//
// The new face's windows are opened before the old face's are closed, and
// for two reasons: the application ends when its last window goes, and a
// player that blinked out of existence between two faces would not be a
// player changing its face. Nothing of the model is touched on the way
// through — the transport is not stopped, the queue is not rebuilt, the
// equaliser keeps its curve — so the track goes on playing across the
// switch at the position it had reached.
func (s *Shell) SetFace(id string) error {
	if s.switching || id == s.wearing || !known(id) {
		return nil
	}
	s.switching = true
	defer func() { s.switching = false }()
	old, oldID := s.face, s.wearing
	if err := s.wear(id); err != nil {
		// The old face is still open and still working: a face that
		// cannot be built is not a reason to leave the user with nothing.
		s.face, s.wearing = old, oldID
		s.restart()
		return err
	}
	if old != nil {
		old.Close()
	}
	return nil
}

// NextFace steps to the next face, and round from the last to the first.
func (s *Shell) NextFace() { s.step(1) }

// PrevFace steps back to the one before.
func (s *Shell) PrevFace() { s.step(-1) }

func (s *Shell) step(by int) {
	n := len(Faces)
	for i, id := range Faces {
		if id == s.wearing {
			_ = s.SetFace(Faces[((i+by)%n+n)%n])
			return
		}
	}
	_ = s.SetFace(Faces[0])
}

// wear builds a face and opens its windows, in the pack that face wears.
func (s *Shell) wear(id string) error {
	f := newFace(id, s.opts)
	// The skin is chosen for the face that is coming: Packs is asked
	// while the look is being set, and it has to answer for the new one.
	s.wearing = id
	if pack := s.skinFor(id, f); pack != "" && pack != s.Host.Worn() {
		s.Host.SetSkin(pack)
	}
	if err := f.Open(s.Host); err != nil {
		return fmt.Errorf("the %s face: %w", id, err)
	}
	s.faces[id] = f
	s.face = f
	s.remember()
	s.restart()
	f.Refresh()
	return nil
}

// skinFor is the pack a face opens in. The command line's -skin applies to
// the face the application opened in and not to every face after it: asking
// for Minim Classic is asking for a skin Marquee does not have.
func (s *Shell) skinFor(id string, f Face) string {
	if s.opts.Skin != "" && s.face == nil {
		return s.opts.Skin
	}
	if pack := s.settings.SkinOf(id); pack != "" {
		return pack
	}
	if skins := f.Skins(); len(skins) > 0 {
		return skins[0]
	}
	return ""
}

// restart points the model's callbacks and the clock at the face being
// worn. The model is the same one either way; what changes is which
// controls it is put back into.
func (s *Shell) restart() {
	m := s.Host.Model
	m.Transport.Changed = s.refresh
	m.Equaliser.Changed = s.refresh
	s.pulse.Stop()
	if s.face != nil && m.Transport.State == players.Playing {
		s.pulse.Start(s.face.MainWindow())
	}
}

func (s *Shell) refresh() {
	if s.face != nil {
		s.face.Refresh()
	}
}

// tick is one beat of the application's clock: the model, and then whatever
// the face does with it.
func (s *Shell) tick(dt time.Duration) {
	m := s.Host.Model
	m.Transport.Tick(dt)
	m.Spectrum.Advance(m.Transport.Pos, m.Transport.State == players.Playing, dt)
	if s.face != nil {
		s.face.Advance(dt)
	}
}

// Command runs a transport command and keeps the clock going behind it.
func (s *Shell) Command(c players.Command) {
	s.Host.Model.Transport.Do(c)
	if s.Host.Model.Transport.State == players.Playing && s.face != nil {
		s.pulse.Start(s.face.MainWindow())
	}
}

// Keys is the keyboard every face shares, tried after the face's own.
//
// The faces change and these do not: the transport table is the same in all
// three, so someone who learns the keys in one has learnt them in the
// others, and the face switch is on the keyboard in every window of every
// face because a switch that is only in one face's menu is a switch you
// cannot use to leave that face.
func (s *Shell) Keys(w *app.Window, e widget.KeyEvent) bool {
	if e.Key == platform.KeyEscape {
		s.App.Quit()
		return true
	}
	if e.Mods.Ctrl() {
		switch e.Key {
		case platform.KeyQ:
			s.App.Quit()
			return true
		case platform.KeyF:
			// Ctrl+F steps to the next face and Ctrl+Shift+F back to the
			// one before, in every window of every face. Choosing one by
			// name is in the menus, which each face offers somewhere with
			// room for it.
			if e.Mods.Shift() {
				s.PrevFace()
			} else {
				s.NextFace()
			}
			return true
		case platform.KeyK:
			if e.Mods.Shift() {
				s.Host.DropSkin()
			} else {
				s.Host.NextSkin()
			}
			s.refresh()
			return true
		}
		return false
	}
	if players.Typing(w.Focus()) {
		// The focus is in something that takes text: its letters are its
		// own. Nothing the transport answers to is a modifier chord, so
		// there is nothing left to try.
		return false
	}
	if c := players.CommandFor(e); c != players.CmdNone {
		s.Command(c)
		return true
	}
	return false
}

// ---- what is remembered ---------------------------------------------------------

// packs are the skins of the face being put on.
func (s *Shell) packs() []string {
	if f, ok := s.faces[s.wearing]; ok {
		return f.Skins()
	}
	return newFace(s.wearing, s.opts).Skins()
}

// wrote is the host telling the application a skin was chosen.
func (s *Shell) wrote(pack string) {
	s.settings = s.settings.WithSkin(s.wearing, pack)
	s.save()
}

// remember writes the face down as the one to open in next time.
func (s *Shell) remember() {
	if s.settings.Face == s.wearing {
		return
	}
	s.settings.Face = s.wearing
	s.save()
}

func (s *Shell) save() {
	if !s.opts.Remember {
		return
	}
	_ = s.settings.Save()
}

func (s *Shell) faceLabel(id string) string {
	if f, ok := s.faces[id]; ok {
		return f.Label()
	}
	return newFace(id, s.opts).Label()
}

// Status is one line about the application, for a log or a screenshot to be
// read against.
func (s *Shell) Status() string {
	t := s.Host.Model.Transport
	return fmt.Sprintf("%s face · %s · %s · %s", s.wearing, s.Host.Worn(), t.State, players.Percent(t.Volume))
}
