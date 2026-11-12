package shell

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/codemodify/media-player-music/internal/players"
	"github.com/codemodify/media-player-music/musicapi"
	"goframework.io/desktop-ui"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
)

// The library window: the daemon's library, browsed by artist, album,
// genre or year, or searched, its tracks played or added to the queue. It
// is the default look's whatever the face (as the settings window is),
// opened from every face's menu ("Library…") and by Ctrl+B. Browse and
// Search are the daemon's (musicapi.Player), asked on the UI thread with a
// bound, as the shell's other calls; the daemon's library state (Library)
// fills its status line, and a change of the library lists it again.
//
// Its keys: Ctrl+F (or /) the search, Ctrl+1 to Ctrl+4 artists, albums,
// genres, years; in the lists Up, Down, Page Up, Page Down, Home, End and
// typing a name's start choose, Enter goes on to the next list or plays a
// track (the tracks shown, from it), Insert adds the choice to the queue;
// Tab moves between the search, the lists and the buttons; Escape clears
// the search, then closes the window.

const (
	// LibraryWidth and LibraryHeight are the window's size at 1x.
	LibraryWidth  = 900
	LibraryHeight = 560
	// libraryTimeout bounds an Add from the library (files, read at once).
	libraryTimeout = 30 * time.Second
	// searchLimit is how many of each kind a search lists.
	searchLimit = 200
)

// The browse modes, in the mode switch's order.
var libraryModes = []string{musicapi.BrowseArtists, musicapi.BrowseAlbums, musicapi.BrowseGenres, musicapi.BrowseYears}

// libraryView is the library window's content and what it lists.
type libraryView struct {
	s      *Shell
	window *ui.Window
	box    *dialogBox

	modes                   *widgets.Segmented
	search                  *widgets.TextField
	groups, albums, tracks  *widgets.ListView
	groupsHead, tracksHead  *widgets.Label
	albumsHead              *widgets.Label
	albumsColumn            *widgets.FlexBox
	status                  *widgets.Label
	playButton, queueButton *widgets.Button

	mode      int              // libraryModes'
	top       musicapi.Browse  // the browse whose groups are listed
	groupList []musicapi.Group // the groups listed (a search's artists, then albums)
	artists   int              // a search's: how many of groupList are artists
	albumList []musicapi.Group // the chosen group's albums
	trackList []musicapi.Track // the tracks listed
	found     bool             // a search is shown
	last      *widgets.ListView
	version   uint64 // the library listed
}

// ShowLibrary keeps the daemon's library state: the window's status line
// when what it says changed, and the lists again when the library changed.
// The daemon tells the same state more than once (a scan's last progress,
// a renderer's first state): rewriting the status at each would replace
// what the window just said ("3 tracks added to the queue") for nothing.
func (s *Shell) ShowLibrary(library musicapi.Library) {
	if s.closed {
		return
	}
	was := s.library
	s.library = library
	if v := s.libraryView; v != nil && v.window != nil && !v.window.Closed() {
		if libraryStatusChanged(was, library) {
			v.showStatus()
		}
		if library.Version != v.version {
			v.reload()
		}
	}
}

// libraryStatusChanged says whether the library's status line would say
// something else: another version (its counts), a scan started, going on
// or ended, an error, other folders.
func libraryStatusChanged(was, now musicapi.Library) bool {
	return was.Version != now.Version || was.Scanning != now.Scanning || was.Scanned != now.Scanned ||
		was.Error != now.Error || !slices.Equal(was.Folders, now.Folders)
}

// LibraryWindow is the library window, nil when it is not open: for the
// tests.
func (s *Shell) LibraryWindow() *ui.Window {
	if s.libraryView == nil || s.libraryView.window == nil || s.libraryView.window.Closed() {
		return nil
	}
	return s.libraryView.window
}

// openLibrary opens the library window, or brings it forward.
func (s *Shell) openLibrary() {
	if s.closed {
		return
	}
	s.App.Post(func() {
		if s.closed {
			return
		}
		if w := s.LibraryWindow(); w != nil {
			w.Show()
			w.Activate()
			return
		}
		if err := s.newLibraryWindow(); err != nil {
			s.Host.Model.Error = err.Error()
			s.refresh()
		}
	})
}

func (s *Shell) newLibraryWindow() error {
	options := platform.WindowOptions{Title: "Music Library", Center: true, Headless: s.opts.Headless,
		Decorations: platform.DecorationsServer, MinWidth: 640, MinHeight: 400}
	if main := s.MainWindow(); main != nil {
		options.Owner = main.Surface()
	}
	w, err := players.OpenSized(s.App, options, LibraryWidth, LibraryHeight, false)
	if err != nil {
		return err
	}
	v := &libraryView{s: s, window: w}
	v.build()
	appearance := style.LookAppearance(s.App.Look())
	appearance.Name, appearance.FollowDesktop = players.Themed, false
	v.box.SetLook(style.WithAppearance(s.App.Look(), appearance))
	w.SetContent(v.box)
	w.SetOnCloseRequest(func() bool {
		if s.libraryView == v {
			s.libraryView = nil
		}
		return true
	})
	s.libraryView = v
	if s.library.Version == 0 && s.player != nil {
		// no library state yet (it comes as the connection is made): ask
		ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
		if library, err := s.player.Library(ctx); err == nil {
			s.library = library
		}
		cancel()
	}
	v.showStatus()
	v.reload()
	w.FocusOn(v.groups)
	w.Activate()
	return nil
}

// closeLibrary closes the library window.
func (s *Shell) closeLibrary() {
	if v := s.libraryView; v != nil {
		s.libraryView = nil
		if v.window != nil && !v.window.Closed() {
			v.window.Close()
		}
	}
}

func (v *libraryView) build() {
	v.modes = widgets.NewSegmented([]string{"Artists", "Albums", "Genres", "Years"}, 0, func(i int) { v.setMode(i) })
	v.modes.SetAccessibleName("Browse by")
	v.search = widgets.NewTextField("", "Search the library", nil)
	v.search.SetAccessibleName("Search the library")
	v.search.OnInput = func(string) { v.reload() }
	v.search.OnSubmit = func(string) {
		v.reload()
		v.window.FocusOn(v.groups)
		if len(v.groupList) == 0 && len(v.trackList) > 0 {
			v.window.FocusOn(v.tracks)
		}
	}
	v.search.OnEscape = func() {
		if v.search.Text != "" {
			v.search.SetText("")
			v.reload()
			return
		}
		v.s.closeLibrary()
	}

	list := func(name string, text func(int) string, detail func(int) string, selected func(int), activated func(int)) *widgets.ListView {
		l := widgets.NewListView(0, text, selected)
		l.ItemDetail = detail
		l.DetailTone = widgets.ToneMuted
		l.OnActivate = activated
		l.SetAccessibleName(name)
		return l
	}
	v.groups = list("Artists", v.groupText, v.groupDetail, v.chooseGroup, func(int) { v.goOn(v.groups) })
	v.albums = list("Albums", v.albumText, v.albumDetail, v.chooseAlbum, func(int) { v.goOn(v.albums) })
	v.tracks = list("Tracks", v.trackText, v.trackDetail, func(int) { v.last = v.tracks }, func(i int) { v.playFrom(i) })

	v.groupsHead = widgets.NewTitle("Artists")
	v.albumsHead = widgets.NewTitle("Albums")
	v.tracksHead = widgets.NewTitle("Tracks")
	column := func(head *widgets.Label, l *widgets.ListView) *widgets.FlexBox {
		c := widgets.NewColumn(head).WithGap(4)
		c.AddFlex(l, 1)
		return c
	}
	v.albumsColumn = column(v.albumsHead, v.albums)
	lists := widgets.NewRow().WithGap(10)
	lists.AddFlex(column(v.groupsHead, v.groups), 1)
	lists.AddFlex(v.albumsColumn, 1)
	lists.AddFlex(column(v.tracksHead, v.tracks), 2)

	top := widgets.NewRow(v.modes).WithGap(10)
	top.AddFlex(v.search, 1)

	v.status = widgets.NewLabel("")
	v.status.Wrap = true
	v.playButton = widgets.NewButton("Play", v.playChosen)
	v.playButton.SetAccessibleDescription("Play the tracks chosen in place of the queue (Enter)")
	v.queueButton = widgets.NewButton("Add to queue", v.queueChosen)
	v.queueButton.SetAccessibleDescription("Add the tracks chosen to the queue's end (Insert)")
	bottom := widgets.NewRow().WithGap(8)
	bottom.AddFlex(v.status, 1)
	bottom.Add(v.playButton)
	bottom.Add(v.queueButton)

	content := widgets.NewColumn(top).WithGap(10).WithPadding(14, 12, 14, 12)
	content.AddFlex(lists, 1)
	content.Add(bottom)
	v.box = newDialogBox("Music library", content, v.s.closeLibrary)
	v.box.keys = v.keys
}

// keys are the window's own: those the focused control left.
func (v *libraryView) keys(e widget.KeyEvent) bool {
	switch {
	case e.Mods.Ctrl() && (e.Key == platform.KeyF):
		v.window.FocusOn(v.search)
		return true
	case !e.Mods.Ctrl() && e.Char == '/' && !players.Typing(v.window.Focus()):
		v.window.FocusOn(v.search)
		return true
	case e.Mods.Ctrl() && e.Key >= platform.Key1 && e.Key <= platform.Key4:
		v.setMode(int(e.Key - platform.Key1))
		return true
	case e.Key == platform.KeyInsert && !players.Typing(v.window.Focus()):
		v.queueChosen()
		return true
	}
	return false
}

// setMode lists the library by another field, and leaves a search.
func (v *libraryView) setMode(mode int) {
	if mode < 0 || mode >= len(libraryModes) {
		return
	}
	v.mode = mode
	if v.modes.Selected != mode {
		v.modes.Selected = mode
		v.modes.Invalidate()
	}
	if v.search.Text != "" {
		v.search.SetText("")
	}
	v.reload()
	v.window.FocusOn(v.groups)
}

// call asks the daemon, with a bound; a failure is said in the status line
// and the player's.
func (v *libraryView) call(do func(ctx context.Context, player musicapi.Player) error) bool {
	if v.s.player == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	if err := do(ctx, v.s.player); err != nil {
		v.status.SetText(err.Error())
		v.s.daemonError(err)
		return false
	}
	return true
}

// reload lists the library again, as the mode and the search say, the
// group chosen kept where it is still there.
func (v *libraryView) reload() {
	v.version = v.s.library.Version
	chosen := ""
	if i := v.groups.Selected; i >= 0 && i < len(v.groupList) {
		chosen = v.groupKey(i)
	}
	v.groupList, v.albumList, v.trackList, v.artists = nil, nil, nil, 0
	text := v.search.Text
	v.found = text != ""
	if v.found {
		var found musicapi.Found
		if v.call(func(ctx context.Context, player musicapi.Player) (err error) {
			found, err = player.Search(ctx, musicapi.Search{Text: text, Limit: searchLimit})
			return err
		}) {
			v.groupList = append(append(v.groupList, found.Artists...), found.Albums...)
			v.artists = len(found.Artists)
			v.trackList = found.Tracks
		}
		v.groupsHead.SetText("Artists and albums")
		v.groups.SetAccessibleName("Artists and albums found")
		v.tracksHead.SetText("Tracks found")
		if found.More {
			v.tracksHead.SetText("Tracks found (the first " + strconv.Itoa(searchLimit) + ")")
		}
		v.tracks.SetAccessibleName("Tracks found")
		v.albumsColumn.SetVisible(false)
	} else {
		by := libraryModes[v.mode]
		v.top = musicapi.Browse{By: by}
		var browsed musicapi.Browsed
		if v.call(func(ctx context.Context, player musicapi.Player) (err error) {
			browsed, err = player.Browse(ctx, v.top)
			return err
		}) {
			v.groupList = browsed.Groups
		}
		head := v.modes.Segments[v.mode]
		v.groupsHead.SetText(head)
		v.groups.SetAccessibleName(head)
		v.tracksHead.SetText("Tracks")
		v.tracks.SetAccessibleName("Tracks")
		v.albumsColumn.SetVisible(by != musicapi.BrowseAlbums)
	}
	v.show(v.groups, len(v.groupList))
	v.show(v.albums, 0)
	v.show(v.tracks, len(v.trackList))
	index := 0
	for i := range v.groupList {
		if v.groupKey(i) == chosen {
			index = i
			break
		}
	}
	if len(v.groupList) > 0 && !v.found {
		v.groups.Selected = index
		v.groups.EnsureVisible(index)
		v.chooseGroup(index)
	} else {
		v.groups.Selected = -1
		if len(v.trackList) > 0 {
			v.tracks.Selected = 0
		}
	}
	v.showStatus()
	v.box.RequestLayout()
	v.box.Invalidate()
}

// show gives a list its count, its choice kept within it.
func (v *libraryView) show(l *widgets.ListView, count int) {
	l.Count = count
	if l.Selected >= count {
		l.Selected = count - 1
	}
	if count == 0 {
		l.Selected = -1
	}
	l.OffsetY = 0
	l.RequestLayout()
	l.Invalidate()
}

// chooseGroup lists a group's albums (an artist's, a genre's, a year's)
// and all its tracks; an album's tracks.
func (v *libraryView) chooseGroup(i int) {
	v.last = v.groups
	if i < 0 || i >= len(v.groupList) {
		return
	}
	group := v.groupList[i]
	top := v.top
	if v.found {
		top = musicapi.Browse{By: musicapi.BrowseArtists}
		if i >= v.artists {
			top.By = musicapi.BrowseAlbums
		}
	}
	v.albumList = nil
	if top.By != musicapi.BrowseAlbums && !v.found {
		inner := inside(top, group)
		var browsed musicapi.Browsed
		if v.call(func(ctx context.Context, player musicapi.Player) (err error) {
			browsed, err = player.Browse(ctx, inner)
			return err
		}) {
			v.albumList = browsed.Groups
		}
	}
	v.show(v.albums, len(v.albumList))
	v.albums.Selected = -1
	v.listTracks(tracksOf(top, group))
}

// chooseAlbum lists an album's tracks.
func (v *libraryView) chooseAlbum(i int) {
	v.last = v.albums
	if i < 0 || i >= len(v.albumList) {
		return
	}
	g := v.groups.Selected
	if g < 0 || g >= len(v.groupList) {
		return
	}
	v.listTracks(tracksOf(inside(v.top, v.groupList[g]), v.albumList[i]))
}

// listTracks lists the tracks a Browse finds.
func (v *libraryView) listTracks(browse musicapi.Browse) {
	var browsed musicapi.Browsed
	v.trackList = nil
	if v.call(func(ctx context.Context, player musicapi.Player) (err error) {
		browsed, err = player.Browse(ctx, browse)
		return err
	}) {
		v.trackList = browsed.Tracks
	}
	v.show(v.tracks, len(v.trackList))
	if len(v.trackList) > 0 {
		v.tracks.Selected = 0
	}
	v.showStatus()
}

// goOn goes from a list to the next one (Enter, a double click).
func (v *libraryView) goOn(from *widgets.ListView) {
	switch {
	case from == v.groups && v.albumsColumn.Visible() && len(v.albumList) > 0:
		if v.albums.Selected < 0 {
			v.albums.Selected = 0
			v.chooseAlbum(0)
		}
		v.window.FocusOn(v.albums)
	case len(v.trackList) > 0:
		if v.tracks.Selected < 0 {
			v.tracks.Selected = 0
		}
		v.window.FocusOn(v.tracks)
	}
}

// chosen is what the buttons act on: in the tracks list its track, else
// the tracks listed (the group's or the album's); and which to start at.
func (v *libraryView) chosen() ([]musicapi.Track, int) {
	if v.last == v.tracks {
		if i := v.tracks.Selected; i >= 0 && i < len(v.trackList) {
			return v.trackList, i
		}
	}
	return v.trackList, -1
}

// playChosen plays what is chosen in place of the queue: the tracks
// listed, from the track chosen.
func (v *libraryView) playChosen() {
	tracks, from := v.chosen()
	v.play(tracks, max(0, from))
}

// playFrom plays the tracks listed, from the one at i.
func (v *libraryView) playFrom(i int) {
	v.last = v.tracks
	v.play(v.trackList, i)
}

// queueChosen adds what is chosen to the queue: the track chosen, else the
// tracks listed.
func (v *libraryView) queueChosen() {
	tracks, from := v.chosen()
	if from >= 0 {
		tracks = tracks[from : from+1]
	}
	if len(tracks) == 0 {
		return
	}
	added, ok := v.add(tracks, false)
	if ok {
		text := strconv.Itoa(added.Tracks) + " tracks added to the queue"
		if added.Tracks == 1 {
			text = "1 track added to the queue"
		}
		if added.Skipped != "" {
			text += "; " + added.Skipped
		}
		v.status.SetText(text)
		v.window.Announce(text)
	}
}

// play puts tracks in place of the queue and plays the one at index.
func (v *libraryView) play(tracks []musicapi.Track, index int) {
	if index < 0 || index >= len(tracks) {
		return
	}
	added, ok := v.add(tracks, true)
	if !ok || added.Tracks == 0 {
		return
	}
	if index > 0 && index < added.Tracks {
		v.call(func(ctx context.Context, player musicapi.Player) error {
			state, err := player.Play(ctx, index)
			if err == nil {
				v.s.ShowState(state)
			}
			return err
		})
	}
	text := "Playing " + trackLine(tracks[index])
	v.status.SetText(text)
	v.window.Announce(text)
}

// add adds tracks' files to the queue, or in its place.
func (v *libraryView) add(tracks []musicapi.Track, replace bool) (musicapi.Added, bool) {
	paths := make([]string, len(tracks))
	for i, each := range tracks {
		paths[i] = each.Path
	}
	var added musicapi.Added
	if v.s.player == nil {
		return added, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), libraryTimeout)
	defer cancel()
	added, err := v.s.player.Add(ctx, musicapi.Add{Paths: paths, Replace: replace})
	if err != nil {
		v.status.SetText(err.Error())
		v.s.daemonError(err)
		return added, false
	}
	v.s.ShowState(added.State)
	return added, true
}

// showStatus says what the library holds, and whether it is being read.
func (v *libraryView) showStatus() {
	library := v.s.library
	var text string
	switch {
	case library.Error != "":
		text = "The library: " + library.Error
	case library.Scanning:
		text = "Reading the library… " + strconv.Itoa(library.Scanned) + " files"
	case library.Tracks == 0 && len(library.Folders) == 0:
		text = "The library is empty: it reads the folders added in the terminal or web UI's settings."
	case library.Tracks == 0:
		text = "No music in the library's folders yet."
	default:
		text = fmt.Sprintf("%s · %s · %s · %s", count(library.Tracks, "track"), count(library.Artists, "artist"), count(library.Albums, "album"), clock(library.LengthMS))
	}
	if v.found {
		text = count(len(v.groupList), "artist or album", "artists and albums") + " and " + count(len(v.trackList), "track") + " found · " + text
	}
	v.status.SetText(text)
	v.playButton.SetEnabled(len(v.trackList) > 0)
	v.queueButton.SetEnabled(len(v.trackList) > 0)
}

// the lists' rows

func (v *libraryView) groupKey(i int) string {
	g := v.groupList[i]
	return g.Name + "\x00" + g.Artist + "\x00" + strconv.Itoa(g.Year)
}

func (v *libraryView) groupText(i int) string {
	if i < 0 || i >= len(v.groupList) {
		return ""
	}
	g := v.groupList[i]
	by := v.top.By
	if v.found {
		by = musicapi.BrowseArtists
		if i >= v.artists {
			by = musicapi.BrowseAlbums
		}
	}
	return groupName(by, g)
}

func (v *libraryView) groupDetail(i int) string {
	if i < 0 || i >= len(v.groupList) {
		return ""
	}
	if v.found {
		if i < v.artists {
			return "artist"
		}
		return "album"
	}
	return count(v.groupList[i].Tracks, "track")
}

func (v *libraryView) albumText(i int) string {
	if i < 0 || i >= len(v.albumList) {
		return ""
	}
	album := v.albumList[i]
	if v.top.By == musicapi.BrowseArtists {
		album.Artist = "" // the artist chosen's: said once, in the list beside
	}
	return groupName(musicapi.BrowseAlbums, album)
}

func (v *libraryView) albumDetail(i int) string {
	if i < 0 || i >= len(v.albumList) {
		return ""
	}
	if year := v.albumList[i].Year; year != 0 {
		return strconv.Itoa(year)
	}
	return ""
}

func (v *libraryView) trackText(i int) string {
	if i < 0 || i >= len(v.trackList) {
		return ""
	}
	t := v.trackList[i]
	text := trackLine(t)
	if t.Number > 0 {
		text = strconv.Itoa(t.Number) + ". " + text
	}
	return text
}

func (v *libraryView) trackDetail(i int) string {
	if i < 0 || i >= len(v.trackList) || v.trackList[i].LengthMS <= 0 {
		return ""
	}
	return clock(v.trackList[i].LengthMS)
}

// groupName is a group's name as a list of by shows it.
func groupName(by string, g musicapi.Group) string {
	switch {
	case by == musicapi.BrowseYears && g.Year == 0:
		return "(no year)"
	case by == musicapi.BrowseYears:
		return strconv.Itoa(g.Year)
	case g.Name == "":
		return "(unknown)"
	case by == musicapi.BrowseAlbums && g.Artist != "":
		return g.Name + " — " + g.Artist
	}
	return g.Name
}

// trackLine is a track's title and artist.
func trackLine(t musicapi.Track) string {
	if t.Artist == "" {
		return t.Title
	}
	return t.Title + " — " + t.Artist
}

// count is n things in words: "1 track", "12 tracks".
func count(n int, one string, many ...string) string {
	if n == 1 {
		return "1 " + one
	}
	if len(many) > 0 {
		return strconv.Itoa(n) + " " + many[0]
	}
	return strconv.Itoa(n) + " " + one + "s"
}

// clock is a length in milliseconds as h:mm:ss or m:ss.
func clock(ms int64) string {
	seconds := ms / 1000
	if seconds >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", seconds/3600, seconds/60%60, seconds%60)
	}
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}

// inside is what a group of a list holds: an artist's, a genre's or a
// year's albums, an album's tracks; the group of the tracks whose tags do
// not say ("(unknown)") holds those alone (Browse's Unknown). As the
// terminal UI's.
func inside(browse musicapi.Browse, group musicapi.Group) musicapi.Browse {
	browse.Offset, browse.Limit = 0, 0
	unknown := func(field string) {
		browse.Unknown = append(slices.Clone(browse.Unknown), field)
	}
	switch browse.By {
	case musicapi.BrowseArtists:
		browse.By, browse.Artist = musicapi.BrowseAlbums, group.Name
		if group.Name == "" {
			unknown("artist")
		}
	case musicapi.BrowseGenres:
		browse.By, browse.Genre = musicapi.BrowseAlbums, group.Name
		if group.Name == "" {
			unknown("genre")
		}
	case musicapi.BrowseYears:
		browse.By, browse.Year = musicapi.BrowseAlbums, group.Year
		if group.Year == 0 {
			unknown("year")
		}
	case musicapi.BrowseAlbums:
		browse.By, browse.Album = musicapi.BrowseTracks, group.Name
		if browse.Artist == "" {
			browse.Artist = group.Artist // the album of that artist, not another's of that name
		}
		if group.Name == "" {
			unknown("album")
		}
	}
	return browse
}

// tracksOf is the Browse of a group's tracks: all of them, in the
// library's order.
func tracksOf(browse musicapi.Browse, group musicapi.Group) musicapi.Browse {
	browse = inside(browse, group)
	browse.By = musicapi.BrowseTracks
	return browse
}
