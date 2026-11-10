package standard

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"goframework.io/desktop-ui/platform"
)

func TestDefaultPlacementRestoresAfterReopeningAtFractionalScale(t *testing.T) {
	for _, scale := range []float32{1, 1.75} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			a, p := open(t, scale)
			p.Main.SetSize(1020, 640)
			if !p.Main.Move(-240, 170) {
				t.Fatal("headless window did not accept a position")
			}
			a.PumpOnce()
			saved := p.CapturePlacement()
			if len(saved) != 1 || saved["main"] != (players.WindowPlacement{X: -240, Y: 170, Width: 1020, Height: 640, Positioned: true}) {
				t.Fatalf("default saved unexpected geometry: %+v", saved)
			}
			data, err := json.Marshal(saved)
			if err != nil {
				t.Fatal(err)
			}
			var decoded map[string]players.WindowPlacement
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatal(err)
			}
			p.Close()
			if got := p.CapturePlacement(); len(got) != 0 {
				t.Fatalf("closed window was persisted: %+v", got)
			}
			reopened, err := New(p.Host, Options{Headless: true, Scale: scale})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(reopened.Close)
			reopened.RestorePlacement(decoded)
			a.PumpOnce()
			if got := reopened.CapturePlacement(); !reflect.DeepEqual(got, saved) {
				t.Fatalf("restored geometry %+v, want %+v", got, saved)
			}
		})
	}
}

func TestDefaultStartupPlacementWaitsForFrameAndRetainsTheSavedTarget(t *testing.T) {
	a, p := open(t, 1.75)
	p.Close()
	reopened, err := New(p.Host, Options{Headless: true, Scale: 1.75})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(reopened.Close)
	want := players.WindowPlacement{X: 120, Y: 75, Width: 780, Height: 470, Positioned: true}
	x, y, known := reopened.Main.Position()
	reopened.RestorePlacement(map[string]players.WindowPlacement{"main": want})
	if gotX, gotY, gotKnown := reopened.Main.Position(); gotX != x || gotY != y || gotKnown != known {
		t.Fatal("startup moved before the first toolkit layout installed its frame")
	}
	if got := reopened.CapturePlacement()["main"]; got != want {
		t.Fatalf("startup would overwrite saved placement before acknowledgement: got %+v, want %+v", got, want)
	}
	a.PumpOnce()
	reopened.Advance(70 * time.Millisecond)
	if got := reopened.CapturePlacement()["main"]; got != want || reopened.restore != nil {
		t.Fatalf("first layout did not restore and acknowledge geometry: %+v pending=%+v", got, reopened.restore)
	}
	reopened.Main.Move(380, 240)
	reopened.Advance(70 * time.Millisecond)
	if got := reopened.CapturePlacement()["main"]; got.X != 380 || got.Y != 240 {
		t.Fatalf("acknowledged startup restoration overrode a later user move: %+v", got)
	}
}

func TestDefaultPendingRestoreAdoptsUserMovementAndStopsWithoutAcknowledgement(t *testing.T) {
	a, p := open(t, 1)
	p.RestorePlacement(map[string]players.WindowPlacement{"main": {X: 120, Y: 75, Width: 780, Height: 470, Positioned: true}})
	if got := p.CapturePlacement()["main"]; got.Width != 780 || got.Height != 470 {
		t.Fatalf("an already laid-out window saved a stale size while resizing: %+v", got)
	}
	p.Main.Move(380, 240)
	a.PumpOnce()
	p.Advance(70 * time.Millisecond)
	if got := p.CapturePlacement()["main"]; got.X != 380 || got.Y != 240 || p.restore != nil {
		t.Fatalf("pending startup restoration fought a user or desktop move: %+v pending=%+v", got, p.restore)
	}

	// A desktop can accept move requests without exposing their final
	// position. The startup target must not stay pending forever there.
	off := p.Main.Surface().(*platform.Offscreen)
	off.SimulateGeometry(platform.GeometryPosition)
	p.RestorePlacement(map[string]players.WindowPlacement{"main": {X: 120, Y: 75, Positioned: true}})
	for range 40 {
		p.Advance(70 * time.Millisecond)
	}
	if p.restore != nil || p.CapturePlacement()["main"].Positioned {
		t.Fatalf("unobservable startup request never expired: pending=%+v saved=%+v", p.restore, p.CapturePlacement())
	}
}

func TestDefaultPlacementClampsSizeAndKeepsUnknownPosition(t *testing.T) {
	a, p := open(t, 1.75)
	p.Main.Move(350, 210)
	p.RestorePlacement(map[string]players.WindowPlacement{"main": {Width: 50, Height: 40}})
	a.PumpOnce()
	if w, h := p.Main.Size(); w != MinW || h != MinH {
		t.Fatalf("saved small dimensions bypassed the minimum: %dx%d", w, h)
	}
	if x, y, known := p.Main.Position(); !known || x != 350 || y != 210 {
		t.Fatalf("an unknown saved position moved the window: %d,%d known=%t", x, y, known)
	}
	p.RestorePlacement(map[string]players.WindowPlacement{"main": {X: 0, Y: 0, Positioned: true}})
	a.PumpOnce()
	if x, y, known := p.Main.Position(); !known || x != 0 || y != 0 {
		t.Fatalf("a known saved origin was ignored: %d,%d known=%t", x, y, known)
	}
	if w, h := p.Main.Size(); w != MinW || h != MinH {
		t.Fatal("missing saved dimensions changed the size")
	}
}
