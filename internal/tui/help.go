package tui

import (
	"strconv"

	ui "goframework.io/terminal-ui"
	widgets "goframework.io/terminal-ui-widgets"
)

// line is a line of the help's view: a heading, or a name and a value.
type line struct {
	name, value string
	heading     bool
}

// helpPage is what plays, all the tags say of it, then the keys.
type helpPage struct {
	view  *View
	list  *widgets.List
	lines []line
}

func newHelpPage(view *View) *helpPage {
	thisRef := &helpPage{view: view}
	thisRef.list = widgets.NewList(0, thisRef.paint)
	thisRef.show()
	return thisRef
}

func (thisRef *helpPage) root() ui.Widget { return thisRef.list }
func (thisRef *helpPage) focus()          { thisRef.list.Focus() }

// show makes the lines again: the track playing's as the state tells it.
func (thisRef *helpPage) show() {
	thisRef.lines = nil
	add := func(name, value string) {
		if value != "" && value != "0" {
			thisRef.lines = append(thisRef.lines, line{name: name, value: value})
		}
	}
	state := thisRef.view.state
	if thisRef.view.stated {
		thisRef.lines = append(thisRef.lines, line{name: "Now playing", heading: true})
		if state.Index < 0 || state.Queue.Length == 0 {
			add("", "nothing: the queue is empty")
		} else {
			track := state.Track
			add("Title", track.Title)
			add("Artist", track.Artist)
			add("Album", track.Album)
			if track.AlbumArtist != track.Artist {
				add("Album artist", track.AlbumArtist)
			}
			add("Genre", track.Genre)
			add("Year", strconv.Itoa(track.Year))
			if track.Number > 0 {
				add("Track", strconv.Itoa(track.Number))
			}
			add("Disc", strconv.Itoa(track.Disc))
			if track.LengthMS > 0 {
				add("Length", clock(track.LengthMS))
			}
			format := ""
			if track.Rate > 0 {
				format = strconv.Itoa(track.Rate) + " Hz"
			}
			if track.Channels > 0 {
				format += " · " + strconv.Itoa(track.Channels) + " channels"
			}
			if track.Bitrate > 0 {
				format += " · " + strconv.Itoa(track.Bitrate/1000) + " kbit/s"
			}
			add("Format", format)
			add("File", track.Path)
			add("In the queue", strconv.Itoa(state.Index+1)+" of "+strconv.Itoa(state.Queue.Length))
		}
		add("Renderers", strconv.Itoa(state.Clients))
	}
	thisRef.lines = append(thisRef.lines, line{name: "Keys", heading: true})
	for _, each := range thisRef.view.bindings() {
		thisRef.lines = append(thisRef.lines, line{name: keyText(each.keys), value: each.title})
	}
	for _, each := range viewKeys {
		thisRef.lines = append(thisRef.lines, line{name: each.keys, value: each.title})
	}
	thisRef.lines = append(thisRef.lines, line{name: "Commands", heading: true})
	for _, each := range commandKeys {
		thisRef.lines = append(thisRef.lines, line{name: each.keys, value: each.title})
	}
	thisRef.list.SetCount(len(thisRef.lines))
	thisRef.list.Invalidate()
}

// paint draws a line: a heading, or a name and its value beside it.
func (thisRef *helpPage) paint(painter *ui.Painter, index int, style ui.Style) {
	if index >= len(thisRef.lines) {
		return
	}
	each := thisRef.lines[index]
	if each.heading {
		accent := painter.Theme().Accent
		accent.Background = style.Background
		paintRow(painter, each.name, "", accent)
		return
	}
	width := painter.Size().Width
	column := min(26, width/2)
	name := each.name
	if painter.Width(name) > column-2 {
		name = painter.Model().Truncate(name, max(column-2, 0), "…")
	}
	muted := painter.Theme().Muted
	muted.Background = style.Background
	painter.Text(1, 0, name, muted)
	paintRow(painter.Area(ui.Rect{X: column, Width: max(width-column, 0), Height: 1}), each.value, "", style)
}
