package tui

import (
	"strconv"

	"github.com/codemodify/media-player-music/musicapi"
	ui "goframework.io/terminal-ui"
)

// clock writes milliseconds as a player does: 3:07, 1:02:07.
func clock(ms int64) string {
	seconds := max(ms, 0) / 1000
	hours, minutes := seconds/3600, seconds/60%60
	text := strconv.FormatInt(seconds%60, 10)
	if seconds%60 < 10 {
		text = "0" + text
	}
	if hours > 0 {
		text = strconv.FormatInt(hours, 10) + ":" + twoDigits(minutes) + ":" + text
	} else {
		text = strconv.FormatInt(minutes, 10) + ":" + text
	}
	return text
}

func twoDigits(n int64) string {
	if n < 10 {
		return "0" + strconv.FormatInt(n, 10)
	}
	return strconv.FormatInt(n, 10)
}

// trackText is a track as a status line names it: "Artist - Title".
func trackText(track musicapi.Track) string {
	if track.Artist == "" {
		return track.Title
	}
	return track.Artist + " - " + track.Title
}

// statusMark is the transport's status as a mark.
func statusMark(status string) string {
	switch status {
	case musicapi.StatusPlaying:
		return "▶"
	case musicapi.StatusPaused:
		return "‖"
	}
	return "■"
}

// named is a group's name, or what stands for one the tags do not say.
func named(name string) string {
	if name == "" {
		return "(unknown)"
	}
	return name
}

// paintRow draws a list's row: left at its start, cut by display width
// where it does not fit, and right at its end, muted, left out first
// where the row is narrow.
func paintRow(painter *ui.Painter, left, right string, style ui.Style) {
	width := painter.Size().Width
	muted := painter.Theme().Muted
	muted.Background = style.Background
	room := width - 2
	if right != "" && painter.Width(right)+painter.Width(left)+3 > width && painter.Width(right) > width/3 {
		right = "" // the row's text before its details
	}
	if right != "" {
		room -= painter.Width(right) + 1
	}
	if room < 1 {
		return
	}
	if painter.Width(left) > room {
		left = painter.Model().Truncate(left, room, "…")
	}
	painter.Text(1, 0, left, style)
	if right != "" {
		painter.Text(width-painter.Width(right)-1, 0, right, muted)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

// groupKey names a group of the library's, as a list keeps its choice.
func groupKey(group musicapi.Group) string {
	return group.Name + "\x00" + group.Artist + "\x00" + strconv.Itoa(group.Year)
}
