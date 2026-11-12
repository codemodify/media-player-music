package tui

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/library"
	"github.com/codemodify/media-player-music/internal/playback"
	"github.com/codemodify/media-player-music/internal/service"
	"github.com/codemodify/media-player-music/internal/tags"
	"github.com/codemodify/media-player-music/internal/tags/tagstest"
	"github.com/codemodify/media-player-music/musicapi"
	ui "goframework.io/terminal-ui"
	"goframework.io/terminal-ui/uitest"
)

// engine plays nothing: it keeps the service's commands, and a test plays
// the engine's part through the service's Inject.
type engine struct {
	lock     sync.Mutex
	commands []playback.Command
	events   chan playback.Update
	closed   bool
}

func newEngine() *engine { return &engine{events: make(chan playback.Update)} }

func (thisRef *engine) Apply(command playback.Command) error {
	thisRef.lock.Lock()
	defer thisRef.lock.Unlock()
	if thisRef.closed {
		return playback.ErrClosed
	}
	thisRef.commands = append(thisRef.commands, command)
	return nil
}

func (thisRef *engine) Events() <-chan playback.Update { return thisRef.events }

func (thisRef *engine) Close() error {
	thisRef.lock.Lock()
	defer thisRef.lock.Unlock()
	if !thisRef.closed {
		thisRef.closed = true
		close(thisRef.events)
	}
	return nil
}

// loaded is the path of the last track the engine was told to load.
func (thisRef *engine) loaded() string {
	thisRef.lock.Lock()
	defer thisRef.lock.Unlock()
	for index := len(thisRef.commands) - 1; index >= 0; index-- {
		if thisRef.commands[index].Op == playback.OpLoad {
			return thisRef.commands[index].Path
		}
	}
	return ""
}

// song is a track of the test's library, its tags as the test's reader
// tells them.
type song struct {
	artist, album, genre string
	year, number         int
	title                string
}

var songs = []song{
	{"ABBA", "Arrival", "Pop", 1976, 1, "When I Kissed the Teacher"},
	{"ABBA", "Arrival", "Pop", 1976, 2, "Dancing Queen"},
	{"ABBA", "Voulez-Vous", "Pop", 1979, 1, "As Good as New"},
	{"Radiohead", "OK Computer", "Rock", 1997, 1, "Airbag"},
	{"Radiohead", "OK Computer", "Rock", 1997, 2, "Paranoid Android"},
	{"坂本龍一", "戦場のメリークリスマス", "Soundtrack", 1983, 1, "メリークリスマス・ミスターローレンス・オリジナル・サウンドトラック"},
	{"", "", "", 0, 0, "Untitled Demo"}, // tags that say only its title
}

// collection writes the songs' files: a short MP3 each, with an ID3v2.4
// tag the service reads as it adds them to the queue. The library reads
// them by the test's reader, which says each is three minutes long.
func collection(t *testing.T) (string, func(path string) (tags.Info, bool)) {
	t.Helper()
	sound, err := os.ReadFile(filepath.Join("..", "playback", "testdata", "lame-mono22.mp3"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	byPath := map[string]song{}
	for _, each := range songs {
		folder := filepath.Join(root, each.artist, each.album)
		if err := os.MkdirAll(folder, 0o755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(folder, twoDigits(int64(each.number))+" "+each.title+".mp3")
		frames := []tagstest.Frame{{ID: "TIT2", Data: tagstest.Text(4, each.title)}}
		if each.artist != "" {
			frames = append(frames,
				tagstest.Frame{ID: "TPE1", Data: tagstest.Text(4, each.artist)},
				tagstest.Frame{ID: "TALB", Data: tagstest.Text(4, each.album)},
				tagstest.Frame{ID: "TCON", Data: tagstest.Text(4, each.genre)},
				tagstest.Frame{ID: "TRCK", Data: tagstest.Text(4, strconv.Itoa(each.number))},
				tagstest.Frame{ID: "TDRC", Data: tagstest.Text(4, strconv.Itoa(each.year))},
			)
		}
		tag := tagstest.V2(4, frames...)
		if err := os.WriteFile(path, append(tag, sound...), 0o600); err != nil {
			t.Fatal(err)
		}
		byPath[path] = each
	}
	return root, func(path string) (tags.Info, bool) {
		each, ok := byPath[path]
		if !ok {
			return tags.Info{}, true
		}
		return tags.Info{Title: each.title, Artist: each.artist, Album: each.album, Genre: each.genre, Year: each.year, Number: each.number, Length: 3 * time.Minute}, true
	}
}

// harness is the screen on a terminal in memory, connected to the daemon's
// service in the test's process.
type harness struct {
	t       *testing.T
	driver  *uitest.Driver
	view    *View
	service *service.Service
	engine  *engine
	music   string // the library's folder; "" without one
}

// start runs the screen at columns × rows against a service, its library
// holding the songs when withMusic.
func start(t *testing.T, columns, rows int, withMusic bool) *harness {
	t.Helper()
	h := &harness{t: t, engine: newEngine()}
	h.driver = uitest.NewDriver(t, uitest.Size(columns, rows), uitest.WithOptions(ui.Options{Mouse: ui.MouseClicks}))
	h.view = New(h.driver.App)
	var read func(string) (tags.Info, bool)
	h.music, read = collection(t)
	var s *service.Service
	var changed sync.Mutex
	music, err := library.Open(library.Options{Read: read, Changed: func() {
		changed.Lock()
		defer changed.Unlock()
		if s != nil {
			s.LibraryChanged()
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { music.Close() })
	app := h.driver.App
	changed.Lock()
	s = service.New(service.Options{Engine: h.engine, Library: music, Publish: func(event string, value any) {
		app.Do(func() { tell(h.view, event, value) })
	}})
	changed.Unlock()
	t.Cleanup(func() { s.Close() })
	h.service = s
	h.view.SetPlayer(s)
	h.driver.Start(h.view.Build())
	app.Do(func() { h.view.ShowLink("", false) }) // as link tells it once connected
	s.Publish()
	s.Connected(+1)
	h.driver.Wait(func() bool { return h.view.state.Clients == 1 && h.view.libraryKnown })
	if withMusic {
		if _, err := s.AddFolder(context.Background(), h.music); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := music.Wait(ctx); err != nil {
			t.Fatal(err)
		}
		h.wait("the library", func() bool {
			return h.view.library.Tracks == len(songs) && !h.view.library.Scanning && h.view.library_.rows() == 4
		})
	}
	return h
}

// tell gives the screen a state as link would.
func tell(view *View, event string, value any) {
	switch event {
	case musicapi.StateEvent:
		view.ShowState(value.(musicapi.State))
	case musicapi.QueueEvent:
		view.ShowQueue(value.(musicapi.Queue))
	case musicapi.NowEvent:
		view.ShowNow(value.(musicapi.Now))
	case musicapi.LibraryEvent:
		view.ShowLibrary(value.(musicapi.Library))
	}
}

func (thisRef *harness) wait(what string, condition func() bool) {
	thisRef.t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			thisRef.t.Fatalf("%s did not come:\n%s", what, thisRef.driver.Screen())
		}
		time.Sleep(time.Millisecond)
		thisRef.driver.Settle()
	}
	thisRef.driver.Feed()
}

// shows waits for the screen to show each text.
func (thisRef *harness) shows(texts ...string) {
	thisRef.t.Helper()
	for _, text := range texts {
		thisRef.wait("«"+text+"»", func() bool { return strings.Contains(thisRef.driver.Screen(), text) })
	}
}

// matches waits for the screen to match a regular expression.
func (thisRef *harness) matches(pattern string) {
	thisRef.t.Helper()
	expression := regexp.MustCompile(pattern)
	thisRef.wait("/"+pattern+"/", func() bool { return expression.MatchString(thisRef.driver.Screen()) })
}

// hides fails where the screen shows a text.
func (thisRef *harness) hides(texts ...string) {
	thisRef.t.Helper()
	thisRef.driver.Settle()
	for _, text := range texts {
		if strings.Contains(thisRef.driver.Screen(), text) {
			thisRef.t.Fatalf("«%s» is shown:\n%s", text, thisRef.driver.Screen())
		}
	}
}

// choose moves a list's choice to the row whose key holds text, by keys.
func (thisRef *harness) choose(rows func() int, label func(int) string, text string) {
	thisRef.t.Helper()
	for index := range rows() {
		if strings.Contains(label(index), text) {
			thisRef.driver.Press("home")
			for range index {
				thisRef.driver.Press("down")
			}
			return
		}
	}
	thisRef.t.Fatalf("no row holds %q:\n%s", text, thisRef.driver.Screen())
}

// chooseLibrary chooses the library's row naming text.
func (thisRef *harness) chooseLibrary(text string) {
	thisRef.t.Helper()
	library := thisRef.view.library_
	thisRef.choose(library.rows, library.key, text)
	if !strings.Contains(library.key(library.list.Selected()), text) {
		thisRef.t.Fatalf("%q is not chosen", text)
	}
}

// add adds the songs whose titles hold the texts to the queue, through
// the service.
func (thisRef *harness) add(titles ...string) {
	thisRef.t.Helper()
	var paths []string
	for _, title := range titles {
		matches, _ := filepath.Glob(filepath.Join(thisRef.music, "*", "*", "*"+title+"*.mp3"))
		paths = append(paths, matches...)
	}
	if _, err := thisRef.service.Add(context.Background(), musicapi.Add{Paths: paths}); err != nil {
		thisRef.t.Fatal(err)
	}
	thisRef.wait("the queue", func() bool { return len(thisRef.view.queue.Tracks) == thisRef.view.state.Queue.Length })
}

// queued are the queue's titles, as the screen has them.
func (thisRef *harness) queued() []string {
	var titles []string
	for _, each := range thisRef.view.queue.Tracks {
		titles = append(titles, each.Title)
	}
	return titles
}

func screenshot(t *testing.T, driver *uitest.Driver, name string) {
	if folder := os.Getenv("SCREENSHOTS"); folder != "" {
		if err := os.WriteFile(filepath.Join(folder, name), []byte(driver.Screen()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestNothingOfItsOwnBeforeTheDaemon: until the daemon answers, the
// screen says it is connecting and shows nothing of a player: no track, no
// volume, no library; keys do nothing and break nothing.
func TestNothingOfItsOwnBeforeTheDaemon(t *testing.T) {
	driver := uitest.NewDriver(t, uitest.Size(80, 24))
	view := New(driver.App)
	driver.Start(view.Build())
	driver.Wait(func() bool { return strings.Contains(driver.Screen(), "connecting to the music daemon…") })
	for _, word := range []string{"vol ", "rep ", "Equaliser", "ReplayGain", "nothing to play", "empty"} {
		if strings.Contains(driver.Screen(), word) {
			t.Fatalf("%q before the daemon answered:\n%s", word, driver.Screen())
		}
	}
	for _, key := range []string{"c", "x", "v", "b", "z", "left", "right", "+", "-", "s", "r", "m", "enter", "a", "E", "tab", "backspace",
		"2", "enter", "D", "p", "P", "C", "3", "/", "x", "enter", "escape", "4", "left", "right", "enter", "space", "D", "5", "down", "1", "<", ">"} {
		driver.Press(key)
	}
	driver.App.Do(func() { view.ShowLink("can't reach the music daemon (media-player-musicd), retrying in 2s", true) })
	driver.Wait(func() bool { return strings.Contains(driver.Screen(), "can't reach the music daemon") })
	if view.stated || len(view.settings.rows) != 0 {
		t.Fatal("a state of its own")
	}
}

// TestLibrary goes down the library, artist, album, track, plays a track,
// adds an album and puts another in the queue's place, then lists genres
// and years.
func TestLibrary(t *testing.T) {
	h := start(t, 80, 24, true)
	h.shows("Artists", "ABBA", "Radiohead", "坂本龍一", "3 · 9:00")
	h.chooseLibrary("ABBA")
	h.driver.Press("enter")
	h.shows("Artists › ABBA", "Arrival (1976)", "Voulez-Vous (1979)")
	h.chooseLibrary("Arrival")
	h.driver.Press("enter")
	h.shows("Artists › ABBA › Arrival", "01. When I Kissed the Teacher", "02. Dancing Queen", "3:00")
	screenshot(t, h.driver, "tui-library.txt")

	// Enter plays a track: added to the queue, and played
	h.driver.Press("j", "enter")
	h.wait("Dancing Queen playing", func() bool {
		return h.view.state.Status == musicapi.StatusPlaying && h.view.state.Track.Title == "Dancing Queen"
	})
	if !strings.HasSuffix(h.engine.loaded(), "Dancing Queen.mp3") {
		t.Fatalf("the engine loaded %q", h.engine.loaded())
	}
	h.shows("▶ ABBA - Dancing Queen")

	// back up, the album chosen still; a adds an album, E replaces the queue
	h.driver.Press("backspace")
	h.shows("Voulez-Vous (1979)")
	if !strings.HasPrefix(h.view.library_.key(h.view.library_.list.Selected()), "Arrival") {
		t.Fatalf("the album gone back to is not chosen: %q", h.view.library_.key(h.view.library_.list.Selected()))
	}
	h.chooseLibrary("Voulez-Vous")
	h.driver.Press("a")
	h.wait("the album added", func() bool { return len(h.view.queue.Tracks) == 2 })
	h.chooseLibrary("Arrival")
	h.driver.Press("E")
	h.wait("the queue replaced", func() bool {
		titles := h.queued()
		return len(titles) == 2 && titles[0] == "When I Kissed the Teacher" && titles[1] == "Dancing Queen"
	})
	h.driver.Press("escape")
	h.shows("Radiohead")
	h.chooseLibrary("Radiohead")
	h.driver.Press("e") // an artist's every track
	h.wait("the artist added", func() bool { return len(h.view.queue.Tracks) == 4 })

	// Tab: genres, then years, then artists again
	h.driver.Press("tab")
	h.shows("Genres", "Pop", "Rock", "Soundtrack")
	h.chooseLibrary("Rock")
	h.driver.Press("enter")
	h.shows("Genres › Rock", "OK Computer (1997) — Radiohead")
	h.driver.Press("enter")
	h.shows("Genres › Rock › OK Computer", "01. Airbag", "02. Paranoid Android")
	h.driver.Press("backspace", "backspace", "tab")
	h.shows("Years", "1997", "1983", "1979", "1976")
	h.chooseLibrary("1979")
	h.driver.Press("enter")
	h.shows("Years › 1979", "Voulez-Vous")
	h.driver.Press("escape", "tab")
	h.shows("Artists", "ABBA")
}

// TestQueue plays, moves, removes and clears the queue's tracks by keys,
// the track playing marked.
func TestQueue(t *testing.T) {
	h := start(t, 80, 24, true)
	h.add("Teacher", "Dancing", "Airbag")
	h.driver.Press("2")
	h.shows("Queue", "1. ABBA - When I Kissed the Teacher", "2. ABBA - Dancing Queen", "3. Radiohead - Airbag", "3 · 0:")
	if !strings.Contains(h.driver.Lines()[2], "▶") {
		t.Fatalf("the track playing is not marked:\n%s", h.driver.Screen())
	}
	h.driver.Press("G", "enter")
	h.wait("the third playing", func() bool { return h.view.state.Index == 2 })
	if !strings.Contains(h.driver.Lines()[4], "▶") {
		t.Fatalf("the track playing is not marked:\n%s", h.driver.Screen())
	}
	screenshot(t, h.driver, "tui-queue.txt")

	h.driver.Press("g", "p") // Teacher down
	h.wait("a track moved down", func() bool { return strings.Join(h.queued(), ",") == "Dancing Queen,When I Kissed the Teacher,Airbag" })
	if h.view.queue_.list.Selected() != 1 {
		t.Fatalf("the choice did not follow the track: %d", h.view.queue_.list.Selected())
	}
	h.driver.Press("P") // and up again
	h.wait("a track moved up", func() bool { return strings.Join(h.queued(), ",") == "When I Kissed the Teacher,Dancing Queen,Airbag" })
	h.driver.Press("j", "D")
	h.wait("a track removed", func() bool { return strings.Join(h.queued(), ",") == "When I Kissed the Teacher,Airbag" })
	h.driver.Press("delete")
	h.wait("a track removed by Delete", func() bool { return strings.Join(h.queued(), ",") == "When I Kissed the Teacher" })

	// C asks first: Cancel keeps the queue, Clear empties it
	h.driver.Press("C")
	h.shows("Clear the queue")
	h.driver.Press("escape")
	h.hides("Take every track out")
	if len(h.view.queue.Tracks) != 1 {
		t.Fatal("cancelled, and cleared")
	}
	h.driver.Press("C", "enter")
	h.wait("the queue cleared", func() bool { return len(h.view.queue.Tracks) == 0 && h.view.state.Status == musicapi.StatusStopped })
	h.shows("empty", "nothing to play")
	h.driver.Press("enter", "D", "p", "P", "C", "delete") // nothing to do, nothing broken
	h.hides("Clear the queue")
}

// TestTransport: cmus's keys, sent to the daemon, and the status lines as
// it tells them.
func TestTransport(t *testing.T) {
	h := start(t, 80, 24, true)
	h.add("Teacher", "Dancing")
	h.service.Inject(playback.Update{Kind: playback.UpdateLoaded, Generation: h.service.Generation()})
	h.service.Inject(playback.Update{Kind: playback.UpdateDuration, Generation: h.service.Generation(), Duration: 3 * time.Minute})
	h.service.Inject(playback.Update{Kind: playback.UpdatePosition, Generation: h.service.Generation(), Position: 90 * time.Second})
	h.shows("▶ ABBA - When I Kissed the Teacher", "1:30/3:00", "50%", "rep off")
	h.matches(`vol \d+% rep off`)
	volume := h.view.state.Volume
	screenshot(t, h.driver, "tui-transport.txt")

	steps := []struct {
		keys []string
		what string
		done func(musicapi.State) bool
	}{
		{[]string{"c"}, "paused", func(state musicapi.State) bool { return state.Status == musicapi.StatusPaused }},
		{[]string{"c"}, "playing", func(state musicapi.State) bool { return state.Status == musicapi.StatusPlaying }},
		{[]string{"v"}, "stopped", func(state musicapi.State) bool { return state.Status == musicapi.StatusStopped }},
		{[]string{"x"}, "played", func(state musicapi.State) bool { return state.Status == musicapi.StatusPlaying }},
		{[]string{"b"}, "the next", func(state musicapi.State) bool { return state.Index == 1 }},
		{[]string{"z"}, "the one before", func(state musicapi.State) bool { return state.Index == 0 }},
		{[]string{"right"}, "5 s on", func(state musicapi.State) bool { return state.PositionMS == 5000 }},
		{[]string{"l", "."}, "15 s on", func(state musicapi.State) bool { return state.PositionMS == 15000 }},
		{[]string{"left"}, "5 s back", func(state musicapi.State) bool { return state.PositionMS == 10000 }},
		{[]string{"h", ","}, "10 s back", func(state musicapi.State) bool { return state.PositionMS == 0 }},
		{[]string{">"}, "a minute on", func(state musicapi.State) bool { return state.PositionMS == 60000 }},
		{[]string{"<"}, "a minute back", func(state musicapi.State) bool { return state.PositionMS == 0 }},
		{[]string{"-"}, "the volume down", func(state musicapi.State) bool { return state.Volume < volume }},
		{[]string{"+"}, "the volume up", func(state musicapi.State) bool { return state.Volume == volume }},
		{[]string{"-", "="}, "the volume up by =", func(state musicapi.State) bool { return state.Volume == volume }},
		{[]string{"m"}, "muted", func(state musicapi.State) bool { return state.Volume == 0 }},
		{[]string{"m"}, "the volume back", func(state musicapi.State) bool { return state.Volume == volume }},
		{[]string{"s"}, "shuffled", func(state musicapi.State) bool { return state.Shuffle }},
		{[]string{"r"}, "repeat all", func(state musicapi.State) bool { return state.Repeat == musicapi.RepeatAll }},
	}
	for _, step := range steps {
		h.driver.Press(step.keys...)
		h.wait(step.what, func() bool { return step.done(h.view.state) })
	}
	h.shows("rep all shuf")
	h.driver.Press("m")
	h.shows("muted rep all shuf")
}

// TestSearch: / from anywhere, results as typed, Enter plays a track or
// shows an artist's or an album's in the library, a and E add.
func TestSearch(t *testing.T) {
	h := start(t, 80, 24, true)
	h.driver.Press("2", "/")
	if h.view.Shown() != ViewSearch || !h.view.search.input.Focused() {
		t.Fatal("/ did not start a search")
	}
	h.driver.Type("dancing")
	h.shows("track", "ABBA - Dancing Queen", "1")
	screenshot(t, h.driver, "tui-search.txt")
	h.driver.Press("enter") // to the results
	if !h.view.search.list.Focused() {
		t.Fatal("Enter did not go to the results")
	}
	h.driver.Press("enter")
	h.wait("the track found playing", func() bool {
		return h.view.state.Track.Title == "Dancing Queen" && h.view.state.Status == musicapi.StatusPlaying
	})

	// an artist: shown in the library
	h.driver.Press("/")
	h.driver.Type("abba")
	h.shows("artist ABBA")
	h.driver.Press("enter", "enter")
	h.wait("the artist in the library", func() bool { return h.view.Shown() == ViewLibrary })
	h.shows("Artists › ABBA", "Voulez-Vous (1979)")
	h.driver.Press("/")
	h.driver.Type("arrival")
	h.shows("album  Arrival — ABBA")
	h.driver.Press("enter", "enter")
	h.shows("Artists › ABBA › Arrival", "02. Dancing Queen")

	// an album: its tracks in the library; a adds them, E replaces the queue
	h.driver.Press("/")
	h.driver.Type("computer")
	h.shows("OK Computer — Radiohead")
	h.driver.Press("escape", "a")
	h.wait("the album added", func() bool { return len(h.view.queue.Tracks) == 3 })
	h.driver.Press("E")
	h.wait("the queue replaced", func() bool { return strings.Join(h.queued(), ",") == "Airbag,Paranoid Android" })
	h.driver.Press("enter")
	h.wait("the album in the library", func() bool { return h.view.Shown() == ViewLibrary })
	h.shows("Artists › Radiohead › OK Computer", "01. Airbag")
	h.driver.Press("backspace")
	h.shows("OK Computer (1997)")

	// nothing found; an empty search
	h.driver.Press("/")
	h.driver.Type("zzzz")
	h.shows("nothing found")
	h.driver.Press("enter", "escape", "enter", "a", "E", "/", "backspace") // the results, empty; the search again, emptied
	h.hides("nothing found")
	if h.view.search.input.Text() != "" {
		t.Fatalf("the search holds %q", h.view.search.input.Text())
	}
}

// TestSettings: the equaliser, ReplayGain and the library's folders, by
// keys, as the daemon then has them.
func TestSettings(t *testing.T) {
	h := start(t, 80, 30, true)
	h.driver.Press("4")
	h.shows("Equaliser", "off", "Preset", "Flat", "31 Hz", "16k Hz", "ReplayGain", "Library", h.music, "Add a folder…")
	screenshot(t, h.driver, "tui-settings.txt")
	state := func() musicapi.State { s, _ := h.service.State(context.Background()); return s }

	h.driver.Press("enter")
	h.wait("the equaliser on", func() bool { return state().Equalizer.On })
	h.driver.Press("j", "right")
	h.wait("the next preset", func() bool { return state().Equalizer.Preset == musicapi.Presets[1] })
	h.driver.Press("space")
	h.wait("the preset after", func() bool { return state().Equalizer.Preset == musicapi.Presets[2] })
	h.driver.Press("h", "h", "left")
	h.wait("the last preset", func() bool { return state().Equalizer.Preset == musicapi.Presets[len(musicapi.Presets)-1] })
	h.driver.Press("j", "right", "right", "right")
	h.wait("the preamp up", func() bool { return state().Equalizer.Preamp == 3 })
	before := state().Equalizer.Gains[0]
	h.driver.Press("j", "l", "l")
	h.wait("the lowest band up", func() bool { g := state().Equalizer; return g.Gains[0] == before+2 && g.Preset == "Custom" })
	h.shows("Custom")

	h.choose(func() int { return len(h.view.settings.rows) }, func(index int) string { return h.view.settings.rows[index].label }, "ReplayGain")
	h.shows("ReplayGain        ‹ track ›") // the daemon's at first
	h.driver.Press("enter")
	h.wait("ReplayGain by album", func() bool { return state().ReplayGain.Mode == musicapi.ReplayGainAlbum })
	h.driver.Press("right")
	h.wait("ReplayGain off", func() bool { return state().ReplayGain.Mode == musicapi.ReplayGainOff })
	h.driver.Press("left", "left")
	h.wait("ReplayGain by track", func() bool { return state().ReplayGain.Mode == musicapi.ReplayGainTrack })
	h.driver.Press("j", "left", "left")
	h.wait("ReplayGain's preamp down", func() bool { return state().ReplayGain.Preamp == -2 })

	// a folder taken out, once asked, and added again by its path
	h.choose(func() int { return len(h.view.settings.rows) }, func(index int) string { return h.view.settings.rows[index].label }, h.music)
	h.driver.Press("D")
	h.shows("Remove a folder")
	h.driver.Press("enter")
	h.wait("the folder gone", func() bool { return len(h.view.library.Folders) == 0 && h.view.library.Tracks == 0 })
	h.choose(func() int { return len(h.view.settings.rows) }, func(index int) string { return h.view.settings.rows[index].label }, "Add a folder")
	h.driver.Press("enter")
	h.shows(":lib-add")
	h.driver.Type(h.music)
	h.driver.Press("enter")
	h.wait("the folder back", func() bool { return len(h.view.library.Folders) == 1 && h.view.library.Tracks == len(songs) })
	h.choose(func() int { return len(h.view.settings.rows) }, func(index int) string { return h.view.settings.rows[index].label }, "Read the folders again")
	h.driver.Press("enter")
	h.wait("read again", func() bool { return !h.view.library.Scanning && h.view.library.Tracks == len(songs) })
	h.driver.Press("1")
	h.shows("ABBA")
}

// TestCommands: o, Ctrl+S and : open the command line, each command does
// what it says, Escape leaves it, a wrong one says so.
func TestCommands(t *testing.T) {
	h := start(t, 80, 24, true)
	h.driver.Press("o")
	h.shows(":add")
	h.driver.Type(filepath.Join(h.music, "Radiohead"))
	h.driver.Press("enter")
	h.wait("a folder added", func() bool { return len(h.view.queue.Tracks) == 2 })

	playlist := filepath.Join(t.TempDir(), "queue.m3u8")
	h.driver.Press("ctrl+s")
	h.shows(":save")
	h.driver.Type(playlist)
	h.driver.Press("enter")
	h.wait("the playlist saved", func() bool { _, err := os.Stat(playlist); return err == nil })
	h.shows("Saved the queue as")

	version := h.view.queue.Version
	h.driver.Press(":")
	h.driver.Type("replace " + playlist)
	h.driver.Press("enter")
	h.wait("the playlist in the queue's place", func() bool { return h.view.queue.Version > version && len(h.view.queue.Tracks) == 2 })
	h.service.Inject(playback.Update{Kind: playback.UpdateDuration, Generation: h.service.Generation(), Duration: 3 * time.Minute})
	h.shows("/3:00")

	run := func(text string, done func(musicapi.State) bool) {
		t.Helper()
		h.driver.Press(":")
		h.driver.Type(text)
		h.driver.Press("enter")
		h.wait(text, func() bool { return done(h.view.state) })
	}
	run("vol 50", func(state musicapi.State) bool { return state.Volume == 0.5 })
	run("seek 1:10", func(state musicapi.State) bool { return state.PositionMS == 70000 })
	run("seek -10", func(state musicapi.State) bool { return state.PositionMS == 60000 })
	run("clear", func(state musicapi.State) bool { return state.Queue.Length == 0 })
	run("add "+filepath.Join(h.music, "nowhere.mp3"), func(state musicapi.State) bool { return state.Error != "" })
	h.shows("nowhere.mp3")
	run("lib-remove "+h.music, func(musicapi.State) bool { return len(h.view.library.Folders) == 0 })
	run("lib-add "+h.music, func(musicapi.State) bool { return len(h.view.library.Folders) == 1 })
	run("rescan", func(musicapi.State) bool { return true })

	h.driver.Press(":")
	h.driver.Type("nope")
	h.driver.Press("enter")
	h.shows("no command :nope")
	h.driver.Press(":")
	h.driver.Type("vol loud")
	h.driver.Press("enter")
	h.shows(":vol needs a volume")

	// Escape leaves the line, the focus back in the view
	h.driver.Press("o")
	h.driver.Type("x")
	h.driver.Press("escape")
	if !h.view.prompt.Hidden() || !h.view.library_.list.Focused() {
		t.Fatal("Escape did not leave the command line")
	}
	if len(h.view.queue.Tracks) != 0 {
		t.Fatal("x reached the player through the command line")
	}
}

// TestQuit: q asks, Escape keeps the UI, Enter quits; the daemon plays on.
func TestQuit(t *testing.T) {
	h := start(t, 80, 24, false)
	h.driver.Press("q")
	h.shows("Quit the terminal UI?")
	h.driver.Press("escape")
	h.hides("Quit the terminal UI?")
	select {
	case <-h.driver.App.Done():
		t.Fatal("Escape quit")
	default:
	}
	h.driver.Press("q", "enter")
	select {
	case <-h.driver.App.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("Quit did not quit")
	}
}

// TestEmpty: an empty library and queue say so, and every key in every
// view does nothing wrong.
func TestEmpty(t *testing.T) {
	h := start(t, 80, 24, false)
	h.shows("Artists", "empty: 4 adds a folder", "■ nothing to play")
	h.matches(`vol \d+%`)
	for _, view := range []string{"1", "2", "3", "4", "5"} {
		h.driver.Press(view)
		for _, key := range []string{"enter", "a", "e", "E", "D", "delete", "p", "P", "tab", "backspace", "escape", "space", "j", "k", "g", "G", "c", "b", "z", "right", "left"} {
			h.driver.Press(key)
		}
		h.driver.Press("escape", "escape")
	}
	h.driver.Settle()
	if len(h.view.queue.Tracks) != 0 {
		t.Fatal("tracks of nowhere")
	}
}

// TestNarrow: at 40 × 12 every view fits, long and wide titles cut by the
// cells they take, the status lines whole.
func TestNarrow(t *testing.T) {
	h := start(t, 40, 12, true)
	h.add("メリークリスマス")
	h.service.Inject(playback.Update{Kind: playback.UpdateDuration, Generation: h.service.Generation(), Duration: 3 * time.Minute})
	h.shows("▶ 坂本龍一 - メリー", "…", "0:00/3:00")
	h.matches(`vol \d+%`)
	for _, view := range []string{"1", "2", "3", "4", "5"} {
		h.driver.Press(view, "escape") // Escape: out of the search's text
		h.driver.Settle()
		lines := h.driver.Lines()
		if len(lines) > 12 {
			t.Fatalf("view %s: %d lines", view, len(lines))
		}
		screenshot(t, h.driver, "tui-narrow-"+view+".txt")
	}
	h.driver.Press("2")
	h.shows("1. 坂本龍一 - メリー")
	h.driver.Press("1")
	h.chooseLibrary("坂本龍一")
	h.driver.Press("enter")
	h.shows("戦場のメリークリスマス (1983)")
	h.driver.Press("enter")
	h.shows("01. メリー")
	t.Logf("\n%s", h.driver.Screen())

	// smaller still: nothing breaks
	h.driver.Resize(20, 6)
	for _, view := range []string{"1", "2", "3", "4", "5"} {
		h.driver.Press(view, "escape")
	}
	h.driver.Settle()
}

// TestUnknown: the tracks whose tags do not say an artist, a genre or a
// year are a group of their own, "(unknown)", which opens to them alone.
func TestUnknown(t *testing.T) {
	h := start(t, 80, 24, true)
	open := func(heading string) {
		t.Helper()
		h.driver.Press("G", "enter") // the unknown last
		h.shows(heading, "(unknown)")
		h.hides("Arrival", "OK Computer")
		h.driver.Press("enter")
		h.shows(heading+" › (unknown)", "Untitled Demo")
		h.hides("Airbag", "Dancing Queen")
		if h.view.library_.rows() != 1 {
			t.Fatalf("%d tracks of the unknown:\n%s", h.view.library_.rows(), h.driver.Screen())
		}
	}
	h.shows("(unknown)")
	open("Artists › (unknown)")
	h.driver.Press("backspace", "backspace")
	h.shows("ABBA")
	h.driver.Press("a") // the unknown artist's tracks, chosen again on the way back
	h.wait("the unknown's tracks added", func() bool { return strings.Join(h.queued(), ",") == "Untitled Demo" })
	h.driver.Press("tab")
	h.shows("Genres", "Soundtrack")
	open("Genres › (unknown)")
	h.driver.Press("backspace", "backspace", "tab")
	h.shows("Years", "1976", "(no year)")
	open("Years › (no year)")
}

// TestDialogsTakeTheKeys: while a dialog asks, the player's keys do
// nothing, and q opens no second dialog.
func TestDialogsTakeTheKeys(t *testing.T) {
	h := start(t, 80, 24, true)
	h.add("Teacher")
	h.wait("playing", func() bool { return h.view.state.Status == musicapi.StatusPlaying })
	status := h.view.state.Status
	h.driver.Press("q")
	h.shows("Quit the terminal UI?")
	h.driver.Press("c", "v", "2", "q", "s", "/")
	h.driver.Settle()
	if layers := len(h.driver.App.Layers()); layers != 1 {
		t.Fatalf("%d layers", layers)
	}
	if h.view.state.Status != status || h.view.state.Shuffle || h.view.Shown() != ViewLibrary {
		t.Fatalf("keys reached the player under the dialog: %+v, view %d", h.view.state, h.view.Shown())
	}
	h.driver.Press("escape")
	h.hides("Quit the terminal UI?")
	h.driver.Press("2")
	if h.view.Shown() != ViewQueue {
		t.Fatal("the keys stayed taken after the dialog")
	}
}

// Starting media-player-musicd (R115, link's Daemon): the line under the
// views says "Starting…", then why it could not be with Ctrl+R to retry,
// which starts it again; nothing once it runs.
func TestTheScreenSaysTheDaemonIsStartingOrWhyNot(t *testing.T) {
	h := start(t, 100, 30, false)
	app := h.driver.App
	app.Do(func() { h.view.ShowDaemon("Starting media-player-musicd…", false, nil) })
	h.shows("Starting media-player-musicd…")
	var retried atomic.Int32
	app.Do(func() {
		h.view.ShowDaemon("media-player-musicd not found next to this program or on PATH", true, func() { retried.Add(1) })
	})
	h.shows("media-player-musicd not found", "Ctrl+R retry")
	h.driver.Press("ctrl+r")
	h.wait("the retry", func() bool { return retried.Load() == 1 })
	h.driver.Press("ctrl+r") // once: the next comes with the next failure
	h.driver.Settle()
	if retried.Load() != 1 {
		t.Fatalf("retried %d times", retried.Load())
	}
	app.Do(func() { h.view.ShowDaemon("", false, nil) })
	h.shows("5 help")
	h.hides("media-player-musicd not found")
}
