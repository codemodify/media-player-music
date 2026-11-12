package tui

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/codemodify/media-player-music/musicapi"
	ui "goframework.io/terminal-ui"
	widgets "goframework.io/terminal-ui-widgets"
)

// level is a list of the library's, one step of the way down: the top's
// artists, genres or years, an artist's albums, an album's tracks.
type level struct {
	browse   musicapi.Browse
	title    string
	selected string // the row chosen there, by its key, kept for the way back
}

// libraryPage is the library's view: its artists, genres or years (Tab),
// then their albums, then the albums' tracks, as the daemon lists them.
type libraryPage struct {
	view    *View
	box     *ui.Flex
	heading *widgets.Label
	count   *widgets.Label
	list    *widgets.List

	levels []level
	groups []musicapi.Group
	tracks []musicapi.Track
	asked  uint64 // the listing asked for last: an older one's answer is left
	listed bool   // the level shown is listed
}

// tops are the lists at the top, in Tab's order.
var tops = []struct{ by, title string }{{musicapi.BrowseArtists, "Artists"}, {musicapi.BrowseGenres, "Genres"}, {musicapi.BrowseYears, "Years"}}

func newLibraryPage(view *View) *libraryPage {
	thisRef := &libraryPage{view: view}
	thisRef.levels = []level{{browse: musicapi.Browse{By: musicapi.BrowseArtists}, title: "Artists"}}
	thisRef.heading = widgets.NewLabel("Artists")
	thisRef.heading.SetTone(widgets.ToneAccent)
	thisRef.heading.SetTruncate(widgets.TruncateStart)
	thisRef.heading.SetWidth(ui.Fill(1))
	thisRef.count = widgets.NewLabel("")
	thisRef.count.SetTone(widgets.ToneMuted)
	thisRef.list = widgets.NewList(0, thisRef.paint)
	thisRef.list.SetHeight(ui.Fill(1))
	thisRef.list.OnActivate(thisRef.open)
	thisRef.list.AddCommand(ui.Command{Name: "library.add", Title: "Add to the queue", Keys: []string{"a", "e"}, Run: func() { thisRef.addSelected(false) }})
	thisRef.list.AddCommand(ui.Command{Name: "library.replace", Title: "Replace the queue", Keys: []string{"E"}, Run: func() { thisRef.addSelected(true) }})
	thisRef.list.AddCommand(ui.Command{Name: "library.back", Title: "Back", Keys: []string{"backspace", "escape"}, Handle: thisRef.back})
	thisRef.list.AddCommand(ui.Command{Name: "library.by", Title: "Artists, genres, years", Keys: []string{"tab"}, Run: thisRef.nextTop})
	header := ui.Row(thisRef.heading, thisRef.count)
	header.SetGap(1)
	header.SetPadding(ui.Symmetric(0, 1))
	thisRef.box = ui.Column(header, thisRef.list)
	return thisRef
}

func (thisRef *libraryPage) root() ui.Widget { return thisRef.box }
func (thisRef *libraryPage) focus()          { thisRef.list.Focus() }

// top is the level at the top's by.
func (thisRef *libraryPage) top() string { return thisRef.levels[0].browse.By }

// nextTop lists the next of artists, genres and years.
func (thisRef *libraryPage) nextTop() {
	at := 0
	for index, each := range tops {
		if each.by == thisRef.top() {
			at = index
		}
	}
	next := tops[(at+1)%len(tops)]
	thisRef.showTop(next.by)
}

// showTop lists the library's artists, genres or years.
func (thisRef *libraryPage) showTop(by string) {
	for _, each := range tops {
		if each.by == by {
			thisRef.levels = []level{{browse: musicapi.Browse{By: by}, title: each.title}}
		}
	}
	thisRef.reload()
}

// showLevels lists the levels given, the last shown: the way to an artist
// or an album a search found.
func (thisRef *libraryPage) showLevels(levels []level) {
	thisRef.levels = levels
	thisRef.reload()
}

// current is the level shown.
func (thisRef *libraryPage) current() *level { return &thisRef.levels[len(thisRef.levels)-1] }

// reload lists the level shown, another than the one listed: the rows
// listed go at once, as Enter on one would open it in the wrong level.
func (thisRef *libraryPage) reload() {
	thisRef.groups, thisRef.tracks, thisRef.listed = nil, nil, false
	thisRef.list.SetCount(0)
	thisRef.ask()
}

// refresh lists the level shown again, the library changed: its rows stay
// until the new ones come, the one chosen chosen again.
func (thisRef *libraryPage) refresh() {
	thisRef.remember()
	thisRef.ask()
}

// ask asks the daemon for the level shown's list.
func (thisRef *libraryPage) ask() {
	thisRef.showHeading()
	if !thisRef.view.libraryKnown {
		return // not connected yet: nothing of its own
	}
	thisRef.asked++
	asked, browse := thisRef.asked, thisRef.current().browse
	thisRef.view.call(func(ctx context.Context, player musicapi.Player) error {
		browsed, err := player.Browse(ctx, browse)
		if err != nil {
			return err
		}
		thisRef.view.app.Do(func() {
			if asked == thisRef.asked {
				thisRef.listed = true
				thisRef.show(browsed)
			}
		})
		return nil
	})
}

// show shows a level's list, the row chosen there before chosen again.
func (thisRef *libraryPage) show(browsed musicapi.Browsed) {
	thisRef.groups, thisRef.tracks = browsed.Groups, browsed.Tracks
	thisRef.list.SetCount(thisRef.rows())
	selected := 0
	for index := range thisRef.rows() {
		if thisRef.key(index) == thisRef.current().selected {
			selected = index
			break
		}
	}
	thisRef.list.SetSelected(selected)
	thisRef.list.Invalidate()
	thisRef.showHeading()
}

// rows is how many rows the list has.
func (thisRef *libraryPage) rows() int { return len(thisRef.groups) + len(thisRef.tracks) }

// key names a row, as a level keeps its choice: a group's name, a track's
// path.
func (thisRef *libraryPage) key(index int) string {
	if index < len(thisRef.groups) {
		return groupKey(thisRef.groups[index])
	}
	if index -= len(thisRef.groups); index < len(thisRef.tracks) {
		return thisRef.tracks[index].Path
	}
	return ""
}

// showHeading shows where the list is: "Artists › ABBA › Arrival", and how
// many rows it has.
func (thisRef *libraryPage) showHeading() {
	var titles []string
	for _, each := range thisRef.levels {
		titles = append(titles, each.title)
	}
	thisRef.heading.SetText(strings.Join(titles, " › "))
	count := ""
	switch {
	case !thisRef.view.libraryKnown:
	case thisRef.view.library.Tracks == 0 && len(thisRef.levels) == 1:
		count = "empty: 4 adds a folder"
	case thisRef.listed:
		count = strconv.Itoa(thisRef.rows())
	}
	thisRef.count.SetText(count)
}

// paint draws a row: a group's name, with its tracks and length; a
// track's number, title and artist, with its length.
func (thisRef *libraryPage) paint(painter *ui.Painter, index int, style ui.Style) {
	if index < len(thisRef.groups) {
		group := thisRef.groups[index]
		name := groupTitle(thisRef.current().browse.By, group)
		if thisRef.current().browse.By == musicapi.BrowseAlbums {
			if group.Year > 0 {
				name += " (" + strconv.Itoa(group.Year) + ")"
			}
			if group.Artist != "" && group.Artist != thisRef.current().browse.Artist {
				name += " — " + group.Artist
			}
		}
		paintRow(painter, name, strconv.Itoa(group.Tracks)+" · "+clock(group.LengthMS), style)
		return
	}
	if index -= len(thisRef.groups); index < len(thisRef.tracks) {
		track := thisRef.tracks[index]
		text := track.Title
		if track.Number > 0 {
			text = twoDigits(int64(track.Number)) + ". " + text
		}
		if track.Artist != "" && track.Artist != thisRef.current().browse.Artist {
			text += " — " + track.Artist
		}
		right := ""
		if track.LengthMS > 0 {
			right = clock(track.LengthMS)
		}
		paintRow(painter, text, right, style)
	}
}

// remember keeps the row chosen in the level shown.
func (thisRef *libraryPage) remember() {
	if thisRef.rows() > 0 {
		thisRef.current().selected = thisRef.key(thisRef.list.Selected())
	}
}

// open goes into a group (an artist's albums, an album's tracks), or plays
// a track: added to the queue's end, and played there.
func (thisRef *libraryPage) open(index int) {
	if index >= len(thisRef.groups) {
		if index -= len(thisRef.groups); index < len(thisRef.tracks) {
			thisRef.view.add([]string{thisRef.tracks[index].Path}, false, true)
		}
		return
	}
	thisRef.remember()
	group := thisRef.groups[index]
	thisRef.levels = append(thisRef.levels, level{browse: inside(thisRef.current().browse, group), title: groupTitle(thisRef.current().browse.By, group)})
	thisRef.reload()
}

// groupTitle is a group's name as a list of by shows it: "(no year)" for
// the tracks whose tags say no year, "(unknown)" for another's.
func groupTitle(by string, group musicapi.Group) string {
	if group.Name == "" && by == musicapi.BrowseYears {
		return "(no year)"
	}
	return named(group.Name)
}

// back goes up a level; at the top, the key goes on.
func (thisRef *libraryPage) back() bool {
	if len(thisRef.levels) < 2 {
		return false
	}
	thisRef.levels = thisRef.levels[:len(thisRef.levels)-1]
	thisRef.reload()
	return true
}

// inside is what a group of a list holds: an artist's, a genre's or a
// year's albums, an album's tracks; the group of the tracks whose tags do
// not say ("(unknown)") holds those alone (Browse's Unknown).
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

// addSelected adds the row chosen to the queue: a track, or a group's
// tracks; or puts them in its place.
func (thisRef *libraryPage) addSelected(replace bool) {
	index := thisRef.list.Selected()
	if index < 0 || index >= thisRef.rows() {
		return
	}
	if index >= len(thisRef.groups) {
		thisRef.view.add([]string{thisRef.tracks[index-len(thisRef.groups)].Path}, replace, false)
		return
	}
	thisRef.view.addBrowsed(tracksOf(thisRef.current().browse, thisRef.groups[index]), replace)
}

// addBrowsed adds the tracks a Browse lists to the queue, or in its place.
func (thisRef *View) addBrowsed(browse musicapi.Browse, replace bool) {
	thisRef.call(func(ctx context.Context, player musicapi.Player) error {
		browsed, err := player.Browse(ctx, browse)
		if err != nil {
			return err
		}
		paths := make([]string, 0, len(browsed.Tracks))
		for _, each := range browsed.Tracks {
			paths = append(paths, each.Path)
		}
		thisRef.app.Do(func() { thisRef.add(paths, replace, false) })
		return nil
	})
}
