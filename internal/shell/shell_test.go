package shell

import (
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/players/playertest"
	"github.com/codemodify/uitoolkit"
	"github.com/codemodify/uitoolkit/platform"
	"github.com/codemodify/uitoolkit/style"
)

// open is the application, headless, in a face.
func open(t *testing.T, face string) (*uitoolkit.Application, *Shell) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	opts := Options{Face: face, Headless: true, Remember: true}
	id := FaceFor(opts)
	t.Setenv(style.ThemeEnv, SkinFor(id, opts))
	a := uitoolkit.New(uitoolkit.Options{Headless: true, DisableLookWatch: true})
	s, err := New(a, opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if s.face != nil {
			s.face.Close()
		}
	})
	a.PumpOnce()
	return a, s
}

// Every face opens, in the pack it was drawn for, with windows that hold up
// to a11y.Check.
func TestEveryFaceOpens(t *testing.T) {
	for _, id := range Faces {
		a, s := open(t, id)
		if s.FaceID() != id {
			t.Fatalf("asked for the %s face and got %s", id, s.FaceID())
		}
		if got, want := s.Host.Worn(), s.face.Skins()[0]; got != want {
			t.Errorf("%s opened wearing %q, want %q", id, got, want)
		}
		if len(s.Windows()) == 0 {
			t.Fatalf("%s opened no windows", id)
		}
		for _, w := range s.Windows() {
			playertest.Audit(t, id+" "+w.Name, w.Window)
		}
		a.PumpOnce()
	}
}

// The point of the whole thing: changing face does not restart the player.
// The transport, the queue and the equaliser are the application's, so
// everything they hold is the same on the other side of the switch.
func TestChangingFaceKeepsThePlayerExactlyAsItWas(t *testing.T) {
	a, s := open(t, FaceMinim)
	m := s.Host.Model
	// Somewhere in the middle of the third track, half muted, shuffled,
	// with the equaliser bent.
	m.Transport.SelectTrack(2)
	m.Transport.Play()
	m.Transport.Pos = 41 * time.Second
	m.Transport.SetVolume(0.31)
	m.Transport.SetRandom(true)
	m.Equaliser.Set(0, 6)
	before := struct {
		track  int
		pos    time.Duration
		vol    float32
		state  players.State
		random bool
		gain   float32
		len    int
	}{m.Transport.List.Index(), m.Transport.Pos, m.Transport.Volume, m.Transport.State,
		m.Transport.Random, m.Equaliser.Gains[0], m.Transport.List.Len()}

	for _, id := range []string{FaceMarquee, FaceLantern, FaceMinim} {
		if err := s.SetFace(id); err != nil {
			t.Fatalf("switching to %s: %v", id, err)
		}
		a.PumpOnce()
		if s.Host.Model != m {
			t.Fatal("the model was rebuilt by a change of face")
		}
		switch {
		case m.Transport.List.Index() != before.track:
			t.Errorf("%s: playing track %d, was %d", id, m.Transport.List.Index(), before.track)
		case m.Transport.Pos != before.pos:
			t.Errorf("%s: the head is at %v, was %v", id, m.Transport.Pos, before.pos)
		case m.Transport.Volume != before.vol:
			t.Errorf("%s: the volume is %v, was %v", id, m.Transport.Volume, before.vol)
		case m.Transport.State != before.state:
			t.Errorf("%s: the transport is %v, was %v", id, m.Transport.State, before.state)
		case !m.Transport.Random:
			t.Errorf("%s: shuffle was turned off by the switch", id)
		case m.Equaliser.Gains[0] != before.gain:
			t.Errorf("%s: the first band is at %v, was %v", id, m.Equaliser.Gains[0], before.gain)
		case m.Transport.List.Len() != before.len:
			t.Errorf("%s: the queue is %d tracks, was %d", id, m.Transport.List.Len(), before.len)
		}
	}
}

// The old face's windows go and the new face's are there, and the one the
// clock wakes on is one of them.
func TestChangingFaceSwapsTheWindows(t *testing.T) {
	a, s := open(t, FaceMinim)
	old := s.Windows()
	if err := s.SetFace(FaceLantern); err != nil {
		t.Fatal(err)
	}
	a.PumpOnce()
	for _, w := range old {
		if !w.Window.Closed() {
			t.Errorf("the strip's %s window is still open after the switch", w.Name)
		}
	}
	now := s.Windows()
	if len(now) == 0 {
		t.Fatal("the lantern face opened no windows")
	}
	for _, w := range now {
		if w.Window.Closed() {
			t.Errorf("the lantern's %s window is closed", w.Name)
		}
	}
	if s.Face().MainWindow() != now[0].Window {
		t.Error("the clock would wake on a window that is not the face's own")
	}
}

// The clock goes on running across a switch: a player that stopped counting
// when its face changed would have stopped playing, which is the one thing
// the switch must not do.
func TestTheClockRunsOnAcrossASwitch(t *testing.T) {
	a, s := open(t, FaceMarquee)
	s.Start()
	if !s.Pulse().Running() {
		t.Fatal("the clock is not running")
	}
	at := s.Host.Model.Transport.Pos
	if err := s.SetFace(FaceMinim); err != nil {
		t.Fatal(err)
	}
	a.PumpOnce()
	if !s.Pulse().Running() {
		t.Error("the clock stopped when the face changed")
	}
	s.Pulse().Tick()
	if s.Host.Model.Transport.Pos <= at {
		t.Errorf("the head did not move after the switch: %v", s.Host.Model.Transport.Pos)
	}
}

// Ctrl+F changes face from the keyboard, in whatever window has it, and
// Ctrl+Shift+F goes back the other way.
func TestTheFaceKeyWorksInEveryWindow(t *testing.T) {
	a, s := open(t, FaceMinim)
	// A window at a time, by place rather than by pointer: the switch back
	// builds the face again, so the windows of the second round are not
	// the windows of the first.
	for i := range s.Windows() {
		want := Faces[1]
		w := s.Windows()[i]
		w.Window.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyF, Mods: platform.ModCtrl})
		a.PumpOnce()
		if s.FaceID() != want {
			t.Fatalf("Ctrl+F in the %s window left the face at %s, want %s", w.Name, s.FaceID(), want)
		}
		// And back, so the next window starts where this one did.
		s.Face().MainWindow().Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyF, Mods: platform.ModCtrl | platform.ModShift})
		a.PumpOnce()
		if s.FaceID() != Faces[0] {
			t.Fatalf("Ctrl+Shift+F left the face at %s, want %s", s.FaceID(), Faces[0])
		}
	}
}

// The transport keys are the same in every face, because they are the
// application's and not a face's.
func TestTheTransportKeysAreTheSameInEveryFace(t *testing.T) {
	for _, id := range Faces {
		a, s := open(t, id)
		tr := s.Host.Model.Transport
		tr.Play()
		s.Face().MainWindow().Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeySpace})
		a.PumpOnce()
		if tr.State != players.Paused {
			t.Errorf("%s: space did not pause the player (%v)", id, tr.State)
		}
		before := tr.List.Index()
		s.Face().MainWindow().Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyB})
		a.PumpOnce()
		if tr.List.Index() == before {
			t.Errorf("%s: B did not change track", id)
		}
	}
}

// The face and the skin are written down, and the next run opens in them.
func TestTheFaceAndSkinAreRemembered(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	opts := Options{Face: FaceMinim, Headless: true, Remember: true}
	t.Setenv(style.ThemeEnv, SkinFor(FaceFor(opts), opts))
	a := uitoolkit.New(uitoolkit.Options{Headless: true, DisableLookWatch: true})
	s, err := New(a, opts)
	if err != nil {
		t.Fatal(err)
	}
	s.Host.NextSkin()
	a.PumpOnce()
	if err := s.SetFace(FaceLantern); err != nil {
		t.Fatal(err)
	}
	a.PumpOnce()
	s.Face().Close()

	// What the next run would read before it builds anything.
	if got := FaceFor(Options{}); got != FaceLantern {
		t.Errorf("the next run would open the %s face, want %s", got, FaceLantern)
	}
	if got := SkinFor(FaceMinim, Options{}); got != "minim-classic" {
		t.Errorf("the next run would dress the strip in %q, want minim-classic", got)
	}
	// A face that was never switched keeps its own skin.
	if got := SkinFor(FaceMarquee, Options{}); got != "marquee" {
		t.Errorf("the marquee face would open in %q, want marquee", got)
	}
}

// -face and -skin beat what was remembered.
func TestTheCommandLineBeatsWhatWasRemembered(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := (players.Settings{Face: FaceLantern}).WithSkin(FaceMinim, "minim-silver").Save(); err != nil {
		t.Fatal(err)
	}
	if got := FaceFor(Options{Face: FaceMarquee}); got != FaceMarquee {
		t.Errorf("-face was ignored: %q", got)
	}
	if got := SkinFor(FaceMinim, Options{Skin: "breeze-night"}); got != "breeze-night" {
		t.Errorf("-skin was ignored: %q", got)
	}
	if got := FaceFor(Options{}); got != FaceLantern {
		t.Errorf("without -face the remembered one is %q", got)
	}
}
