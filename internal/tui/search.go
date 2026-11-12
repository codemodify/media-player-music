package tui

import (
	"context"
	"strings"

	"github.com/codemodify/media-player-music/musicapi"
	ui "goframework.io/terminal-ui"
	widgets "goframework.io/terminal-ui-widgets"
)

// searchPage is the search's view: what is typed, searched in the library
// as it is typed, and what was found, artists, albums and tracks.
type searchPage struct {
	view  *View
	box   *ui.Flex
	input *widgets.Input
	count *widgets.Label
	list  *widgets.List

	text  string // what the results are of
	found musicapi.Found
	asked uint64
}

func newSearchPage(view *View) *searchPage {
	thisRef := &searchPage{view: view}
	thisRef.input = widgets.NewInput("Search: words, \"a phrase\", artist: album: genre: title: year:")
	thisRef.input.SetWidth(ui.Fill(1))
	thisRef.input.OnChange(func(string) { thisRef.ask() })
	thisRef.input.OnSubmit(func(string) { thisRef.toResults() })
	thisRef.input.AddCommand(ui.Command{Name: "search.results", Title: "Results", Keys: []string{"down"}, Hidden: true, Run: thisRef.toResults})
	// Escape leaves what is typed, so that the keys are the player's again
	thisRef.input.AddCommand(ui.Command{Name: "search.leave", Title: "Leave", Keys: []string{"escape"}, Hidden: true, Run: func() { thisRef.list.Focus() }})
	thisRef.count = widgets.NewLabel("")
	thisRef.count.SetTone(widgets.ToneMuted)
	thisRef.list = widgets.NewList(0, thisRef.paint)
	thisRef.list.SetHeight(ui.Fill(1))
	thisRef.list.OnActivate(thisRef.open)
	thisRef.list.AddCommand(ui.Command{Name: "search.add", Title: "Add to the queue", Keys: []string{"a", "e"}, Run: func() { thisRef.addSelected(false) }})
	thisRef.list.AddCommand(ui.Command{Name: "search.replace", Title: "Replace the queue", Keys: []string{"E"}, Run: func() { thisRef.addSelected(true) }})
	thisRef.list.AddCommand(ui.Command{Name: "search.edit", Title: "Search", Keys: []string{"escape"}, Hidden: true, Run: thisRef.input.Focus})
	header := ui.Row(thisRef.input, thisRef.count)
	header.SetGap(1)
	header.SetPadding(ui.Symmetric(0, 1))
	thisRef.box = ui.Column(header, thisRef.list)
	return thisRef
}

func (thisRef *searchPage) root() ui.Widget { return thisRef.box }

// focus goes to the results, or to the search while there are none.
func (thisRef *searchPage) focus() {
	if thisRef.rows() == 0 {
		thisRef.input.Focus()
	} else {
		thisRef.list.Focus()
	}
}

// startSearch shows the search, the focus in what is typed, all of it
// chosen so that typing searches anew.
func (thisRef *View) startSearch() {
	thisRef.Show(ViewSearch)
	if thisRef.prompt.Hidden() {
		thisRef.search.input.Focus()
		thisRef.search.input.Select(0, len(thisRef.search.input.Text()))
	}
}

// toResults puts the focus in the results, when there are some.
func (thisRef *searchPage) toResults() {
	if thisRef.rows() > 0 {
		thisRef.list.Focus()
	}
}

// ask searches what is typed; the answer of an older search is left.
func (thisRef *searchPage) ask() {
	thisRef.asked++
	asked, text := thisRef.asked, strings.TrimSpace(thisRef.input.Text())
	if text == "" {
		thisRef.show(text, musicapi.Found{})
		return
	}
	thisRef.view.call(func(ctx context.Context, player musicapi.Player) error {
		found, err := player.Search(ctx, musicapi.Search{Text: text})
		if err != nil {
			return err
		}
		thisRef.view.app.Do(func() {
			if asked == thisRef.asked {
				thisRef.show(text, found)
			}
		})
		return nil
	})
}

// show shows what was found.
func (thisRef *searchPage) show(text string, found musicapi.Found) {
	thisRef.text, thisRef.found = text, found
	thisRef.list.SetCount(thisRef.rows())
	thisRef.list.SetSelected(0)
	thisRef.list.Invalidate()
	switch {
	case text == "":
		thisRef.count.SetText("")
	case thisRef.rows() == 0:
		thisRef.count.SetText("nothing found")
	case found.More:
		thisRef.count.SetText("first " + itoa(thisRef.rows()))
	default:
		thisRef.count.SetText(itoa(thisRef.rows()))
	}
}

func (thisRef *searchPage) rows() int {
	return len(thisRef.found.Artists) + len(thisRef.found.Albums) + len(thisRef.found.Tracks)
}

// row is what a row of the results is: one of an artist, an album, a
// track.
func (thisRef *searchPage) row(index int) (artist, album *musicapi.Group, track *musicapi.Track) {
	found := thisRef.found
	switch {
	case index < 0:
	case index < len(found.Artists):
		return &found.Artists[index], nil, nil
	case index < len(found.Artists)+len(found.Albums):
		return nil, &found.Albums[index-len(found.Artists)], nil
	case index < thisRef.rows():
		return nil, nil, &found.Tracks[index-len(found.Artists)-len(found.Albums)]
	}
	return nil, nil, nil
}

// paint draws a result: what it is, then its name.
func (thisRef *searchPage) paint(painter *ui.Painter, index int, style ui.Style) {
	artist, album, track := thisRef.row(index)
	muted := painter.Theme().Muted
	muted.Background = style.Background
	kind, text, right := "", "", ""
	switch {
	case artist != nil:
		kind, text, right = "artist", named(artist.Name), itoa(artist.Tracks)+" · "+clock(artist.LengthMS)
	case album != nil:
		kind, text, right = "album", named(album.Name), itoa(album.Tracks)+" · "+clock(album.LengthMS)
		if album.Artist != "" {
			text += " — " + album.Artist
		}
	case track != nil:
		kind, text = "track", trackText(*track)
		if track.LengthMS > 0 {
			right = clock(track.LengthMS)
		}
	default:
		return
	}
	painter.Text(1, 0, kind, muted)
	paintRow(painter.Area(ui.Rect{X: 7, Width: max(painter.Size().Width-7, 0), Height: 1}), text, right, style)
}

// open plays a track found (added to the queue's end), or shows an
// artist's albums or an album's tracks in the library.
func (thisRef *searchPage) open(index int) {
	artist, album, track := thisRef.row(index)
	library := thisRef.view.library_
	switch {
	case track != nil:
		thisRef.view.add([]string{track.Path}, false, true)
	case artist != nil:
		top := level{browse: musicapi.Browse{By: musicapi.BrowseArtists}, title: "Artists", selected: groupKey(*artist)}
		library.showLevels([]level{top, {browse: inside(top.browse, *artist), title: named(artist.Name)}})
		thisRef.view.Show(ViewLibrary)
	case album != nil:
		top := level{browse: musicapi.Browse{By: musicapi.BrowseArtists}, title: "Artists"}
		levels := []level{top}
		if album.Artist != "" {
			byArtist := musicapi.Group{Name: album.Artist}
			levels[0].selected = groupKey(byArtist)
			levels = append(levels, level{browse: inside(top.browse, byArtist), title: album.Artist, selected: groupKey(*album)})
		} else {
			levels = append(levels, level{browse: musicapi.Browse{By: musicapi.BrowseAlbums}, title: "Albums", selected: groupKey(*album)})
		}
		parent := levels[len(levels)-1].browse
		library.showLevels(append(levels, level{browse: inside(parent, *album), title: named(album.Name)}))
		thisRef.view.Show(ViewLibrary)
	}
}

// addSelected adds what the result chosen holds to the queue, or puts it
// in its place.
func (thisRef *searchPage) addSelected(replace bool) {
	artist, album, track := thisRef.row(thisRef.list.Selected())
	switch {
	case track != nil:
		thisRef.view.add([]string{track.Path}, replace, false)
	case artist != nil:
		thisRef.view.addBrowsed(tracksOf(musicapi.Browse{By: musicapi.BrowseArtists}, *artist), replace)
	case album != nil:
		thisRef.view.addBrowsed(tracksOf(musicapi.Browse{By: musicapi.BrowseAlbums}, *album), replace)
	}
}
