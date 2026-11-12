package shell

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/library"
	"github.com/codemodify/media-player-music/internal/playback"
	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/internal/service"
	"github.com/codemodify/media-player-music/internal/tags"
	"github.com/codemodify/media-player-music/musicapi"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/a11y"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
)

// librarySong is a file of the test's library, and what its tags say.
type librarySong struct {
	artist, album, genre string
	year, number         int
	title                string
}

var librarySongs = []librarySong{
	{"ABBA", "Arrival", "Pop", 1976, 1, "When I Kissed the Teacher"},
	{"ABBA", "Arrival", "Pop", 1976, 2, "Dancing Queen"},
	{"ABBA", "Voulez-Vous", "Pop", 1979, 1, "As Good as New"},
	{"Radiohead", "OK Computer", "Rock", 1997, 1, "Airbag"},
	{"Radiohead", "OK Computer", "Rock", 1997, 2, "Paranoid Android"},
	{"", "", "", 0, 0, "Untitled Demo"},
}

// libraryShell is a live shell on the daemon's service in the test's
// process, on a fake engine, with a library of librarySongs (their tags
// the test's reader's; the files hold nothing to decode).
func libraryShell(t *testing.T, skin string) (*ui.Application, *Shell, *fakeEngine) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(style.ThemeEnv, LookFor(skin))
	root := t.TempDir()
	byPath := map[string]librarySong{}
	for i, each := range librarySongs {
		folder := filepath.Join(root, each.artist, each.album)
		if err := os.MkdirAll(folder, 0o755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(folder, string(rune('a'+i))+" "+each.title+".mp3")
		if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
		byPath[path] = each
	}
	read := func(path string) (tags.Info, bool) {
		each := byPath[path]
		return tags.Info{Title: each.title, Artist: each.artist, Album: each.album, Genre: each.genre, Year: each.year, Number: each.number, Length: 3 * time.Minute}, true
	}
	a := ui.New(ui.Options{Headless: true, DisableLookWatch: true})
	e := newFakeEngine()
	var lock sync.Mutex
	var d *daemon
	music, err := library.Open(library.Options{Read: read, Changed: func() {
		lock.Lock()
		defer lock.Unlock()
		if d != nil {
			d.service.LibraryChanged()
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { music.Close() })
	lock.Lock()
	d = &daemon{app: a}
	d.service = service.New(service.Options{Engine: e, Library: music, Publish: func(event string, value any) {
		a.Post(func() {
			if d.shell == nil {
				return
			}
			if event == musicapi.LibraryEvent {
				d.shell.ShowLibrary(value.(musicapi.Library))
				return
			}
			d.shell.Show(event, value)
		})
	}})
	lock.Unlock()
	t.Cleanup(func() { d.service.Close() })
	s, err := New(a, Options{Skin: skin, Headless: true, Player: d.service})
	if err != nil {
		t.Fatal(err)
	}
	d.shell = s
	daemonsLock.Lock()
	daemons[s] = d
	daemonsLock.Unlock()
	t.Cleanup(s.Close)
	s.Pulse().Stop()
	if _, err := d.service.AddFolder(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := music.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	d.service.Publish()
	for deadline := time.Now().Add(20 * time.Second); s.library.Tracks != len(librarySongs) || s.library.Scanning; {
		if time.Now().After(deadline) {
			t.Fatalf("the library: %+v", s.library)
		}
		time.Sleep(5 * time.Millisecond)
		a.DrainPosted()
	}
	a.PumpOnce()
	return a, s, e
}

// pump runs the posted work and a frame.
func pump(a *ui.Application) {
	a.DrainPosted()
	a.PumpOnce()
}

// rows are a list's rows as it shows them.
func rows(l *widgets.ListView) []string {
	var out []string
	for i := range l.Count {
		out = append(out, l.ItemText(i))
	}
	return out
}

func same(got []string, want ...string) bool {
	return strings.Join(got, "|") == strings.Join(want, "|")
}

// The library window (Ctrl+B): the library by artist (an artist's albums
// and tracks), by album, genre or year, and searched, all by keys; Enter
// plays the tracks listed from the one chosen, Insert adds to the queue;
// Escape clears the search, then closes. Its parts are named for
// assistive technology.
func TestTheLibraryWindowBrowsesSearchesAndPlaysByKeys(t *testing.T) {
	a, s, e := libraryShell(t, players.DefaultSkin)
	s.MainWindow().Press(platform.KeyB, platform.ModCtrl)
	pump(a)
	w := s.LibraryWindow()
	if w == nil {
		t.Fatal("Ctrl+B opened no library window")
	}
	v := s.libraryView
	tree := w.AccessibleTree()
	for _, want := range []string{"Music library", "Browse by", "Search the library", "Artists", "Albums", "Tracks", "Play", "Add to queue", "6 tracks · "} {
		if !holds(tree, want) {
			t.Fatalf("the window's tree lacks %q", want)
		}
	}
	if !same(rows(v.groups), "ABBA", "Radiohead", "(unknown)") || w.Focus() != widget.Component(v.groups) {
		t.Fatalf("artists %q, focus %T", rows(v.groups), w.Focus())
	}
	// the first artist chosen: its albums and all its tracks
	if !same(rows(v.albums), "Arrival", "Voulez-Vous") || len(v.trackList) != 3 {
		t.Fatalf("albums %q, tracks %q", rows(v.albums), rows(v.tracks))
	}

	w.Press(platform.KeyDown)
	if !same(rows(v.tracks), "1. Airbag — Radiohead", "2. Paranoid Android — Radiohead") || v.tracks.ItemDetail(0) != "3:00" {
		t.Fatalf("Radiohead's tracks %q", rows(v.tracks))
	}
	w.Press(platform.KeyReturn) // on to the albums
	w.Press(platform.KeyReturn) // on to the album's tracks
	if w.Focus() != widget.Component(v.tracks) {
		t.Fatalf("the focus is on %T", w.Focus())
	}
	w.Press(platform.KeyDown)
	w.Press(platform.KeyReturn) // plays OK Computer from Paranoid Android
	pump(a)
	state, err := s.player.State(context.Background())
	if err != nil || state.Queue.Length != 2 || state.Index != 1 || !strings.Contains(state.Track.Path, "Paranoid Android") {
		t.Fatalf("%+v %v", state, err)
	}
	if load, ok := e.last(playback.OpLoad); !ok || !strings.Contains(load.Path, "Paranoid Android") {
		t.Fatalf("the engine loaded %+v", load)
	}
	w.Press(platform.KeyInsert) // the track chosen to the queue's end
	pump(a)
	if state, _ := s.player.State(context.Background()); state.Queue.Length != 3 {
		t.Fatalf("Insert: %+v", state.Queue)
	}

	// by album, genre, year: Ctrl+2..4
	w.Press(platform.Key2, platform.ModCtrl)
	if !same(rows(v.groups), "Arrival — ABBA", "OK Computer — Radiohead", "Voulez-Vous — ABBA", "(unknown)") || v.albumsColumn.Visible() {
		t.Fatalf("albums %q", rows(v.groups))
	}
	w.Press(platform.Key3, platform.ModCtrl)
	if !same(rows(v.groups), "Pop", "Rock", "(unknown)") || !same(rows(v.albums), "Arrival — ABBA", "Voulez-Vous — ABBA") {
		t.Fatalf("genres %q, albums %q", rows(v.groups), rows(v.albums))
	}
	w.Press(platform.Key4, platform.ModCtrl)
	if !same(rows(v.groups), "1997", "1979", "1976", "(no year)") {
		t.Fatalf("years %q", rows(v.groups))
	}

	// the search: Ctrl+F, typed; Enter goes to what was found
	w.Press(platform.KeyF, platform.ModCtrl)
	if w.Focus() != widget.Component(v.search) {
		t.Fatalf("Ctrl+F: the focus is on %T", w.Focus())
	}
	w.Type("queen")
	pump(a)
	if !same(rows(v.tracks), "2. Dancing Queen — ABBA") || len(v.groupList) != 0 || !holds(w.AccessibleTree(), "Tracks found") {
		t.Fatalf("found %q and %q", rows(v.groups), rows(v.tracks))
	}
	w.Press(platform.KeyReturn)
	if w.Focus() != widget.Component(v.tracks) {
		t.Fatalf("Enter in the search: the focus is on %T", w.Focus())
	}
	v.window.FocusOn(v.search)
	w.Press(platform.KeyEscape) // clears the search
	pump(a)
	if v.search.Text != "" || s.LibraryWindow() == nil || !same(rows(v.groups), "1997", "1979", "1976", "(no year)") {
		t.Fatalf("Escape in the search: %q, %q", v.search.Text, rows(v.groups))
	}
	w.Press(platform.KeyEscape) // closes
	pump(a)
	if s.LibraryWindow() != nil {
		t.Fatal("Escape left the window open")
	}
}

// The buttons: Play plays the tracks listed (a group's), Add to queue adds
// them; a changed library is listed again.
func TestTheLibraryWindowsButtonsAndChanges(t *testing.T) {
	a, s, _ := libraryShell(t, players.DefaultSkin)
	s.openLibrary()
	pump(a)
	v := s.libraryView
	if v == nil {
		t.Fatal("no library window")
	}
	if !v.window.ClickComponent(v.queueButton) {
		t.Fatal("Add to queue did not click")
	}
	pump(a)
	if state, _ := s.player.State(context.Background()); state.Queue.Length != 3 || !strings.Contains(v.status.Text, "3 tracks added to the queue") {
		t.Fatalf("%+v, %q", state.Queue, v.status.Text)
	}
	v.window.FocusOn(v.groups)
	v.window.Press(platform.KeyDown) // Radiohead
	if !v.window.ClickComponent(v.playButton) {
		t.Fatal("Play did not click")
	}
	pump(a)
	if state, _ := s.player.State(context.Background()); state.Queue.Length != 2 || state.Index != 0 || !strings.HasSuffix(state.Track.Path, "Airbag.mp3") {
		t.Fatalf("%+v", state)
	}
	// a library told anew (another version) is listed again, the artist
	// chosen kept
	library := s.library
	library.Version++
	s.ShowLibrary(library)
	if v.groups.Selected != 1 || v.version != library.Version {
		t.Fatalf("chosen %d, version %d", v.groups.Selected, v.version)
	}
}

// Every face's menu (the menu key's) offers "Library…", which opens the
// window; the window follows the player's look-independent default UI.
func TestEveryFacesMenuOpensTheLibrary(t *testing.T) {
	for _, skin := range Appearances {
		t.Run(skin, func(t *testing.T) {
			a, s, _ := liveShell(t, skin)
			w := s.MainWindow()
			w.Inject(platform.Event{Kind: platform.EventKeyDown, Key: platform.KeyMenu})
			pump(a)
			pop, ok := w.Popup().(*widgets.PopupMenu)
			if !ok {
				t.Fatalf("the menu key opened %T", w.Popup())
			}
			index := -1
			for i, item := range pop.Items {
				if item.Text == "Library…" {
					index = i
				}
			}
			if index < 0 {
				t.Fatal("the menu has no Library…")
			}
			if !w.AccessibleAction(widget.ItemID(pop, index), a11y.ActionDefault) {
				t.Fatal("Library… refused its action")
			}
			pump(a)
			pump(a)
			if s.LibraryWindow() == nil {
				t.Fatal("Library… opened no window")
			}
		})
	}
}
