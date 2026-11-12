// Package playlist reads and writes playlist files: M3U and M3U8 (with
// #EXTINF), PLS, and CUE sheets (a disc's tracks within one file, each
// from its INDEX 01 to the next). A playlist's relative paths are its
// folder's; file:// URLs are read as paths; other URLs (streams) are left
// out and counted. An M3U or a CUE sheet that is not UTF-8 is read as
// Windows-1252, as the programs that wrote them wrote them
// (goframework's codecs-text-charsets).
package playlist

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	charsets "goframework.io/codecs-text-charsets"
)

// Entry is a track of a playlist.
type Entry struct {
	// Path is the file, absolute.
	Path string
	// Title and Artist are what the playlist says; "" when it does not.
	Title, Artist string
	// Album is a CUE sheet's disc title.
	Album string
	// Number is a CUE sheet's track number.
	Number int
	// Length is what the playlist says; 0 when not.
	Length time.Duration
	// Start and End are where a CUE sheet's track is in its file; End 0
	// is the file's end.
	Start, End time.Duration
}

// Read is a playlist's tracks.
type Read struct {
	Entries []Entry
	// Skipped is how many entries were left out: URLs of streams.
	Skipped int
}

// MaxSize is the largest playlist file read.
const MaxSize = 16 << 20

// ErrTooLarge is a playlist past MaxSize.
var ErrTooLarge = errors.New("playlist: larger than 16 MiB")

// Is reports whether a file of this name is a playlist the package reads.
func Is(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".m3u", ".m3u8", ".pls", ".cue":
		return true
	}
	return false
}

// Open reads the playlist at path, by its extension.
func Open(path string) (Read, error) {
	file, err := os.Open(path)
	if err != nil {
		return Read{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxSize+1))
	if err != nil {
		return Read{}, err
	}
	if len(data) > MaxSize {
		return Read{}, ErrTooLarge
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return Read{}, err
	}
	folder := filepath.Dir(absolute)
	switch strings.ToLower(filepath.Ext(path)) {
	case ".m3u8":
		return M3U(string(bytes.TrimPrefix(data, bom)), folder), nil
	case ".m3u":
		return M3U(text(data), folder), nil
	case ".pls":
		return PLS(text(data), folder), nil
	case ".cue":
		return CUE(text(data), folder), nil
	}
	return Read{}, fmt.Errorf("%s: not a playlist (M3U, M3U8, PLS or CUE)", path)
}

var bom = []byte{0xef, 0xbb, 0xbf}

// text is a file's text: UTF-8 when it is (its byte order mark dropped),
// else Windows-1252.
func text(data []byte) string {
	data = bytes.TrimPrefix(data, bom)
	if utf8.Valid(data) {
		return string(data)
	}
	return charsets.Windows1252.Decode(data)
}

// lines are text's lines, without their ends.
func lines(text string) []string {
	var list []string
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 64*1024), MaxSize)
	for scanner.Scan() {
		list = append(list, strings.TrimRight(scanner.Text(), "\r"))
	}
	return list
}

// location is a playlist's line as a path: absolute, a file:// URL read,
// a relative one joined to folder; ok is false for another URL.
func location(line, folder string) (string, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", false
	}
	if scheme, _, found := strings.Cut(line, "://"); found && len(scheme) > 1 && !strings.ContainsAny(scheme, `/\`) {
		if !strings.EqualFold(scheme, "file") {
			return "", false
		}
		parsed, err := url.Parse(line)
		if err != nil || parsed.Path == "" {
			return "", false
		}
		path := filepath.FromSlash(parsed.Path)
		if len(path) > 2 && path[0] == '\\' && path[2] == ':' { // file:///C:/… on Windows
			path = path[1:]
		}
		return path, true
	}
	path := filepath.FromSlash(strings.ReplaceAll(line, `\`, "/"))
	if filepath.IsAbs(path) || filepath.VolumeName(line) != "" {
		return filepath.Clean(path), true
	}
	return filepath.Join(folder, path), true
}

// M3U reads an M3U or M3U8 playlist's text: a path a line, #EXTINF's
// length and "Artist - Title" before it.
func M3U(text, folder string) Read {
	var read Read
	var pending Entry
	for _, line := range lines(strings.TrimPrefix(text, "\ufeff")) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if rest, ok := strings.CutPrefix(line, "#EXTINF:"); ok {
			seconds, label, _ := strings.Cut(rest, ",")
			if attributes := strings.IndexByte(seconds, ' '); attributes >= 0 { // #EXTINF:123 tvg-id="…",Title
				seconds = seconds[:attributes]
			}
			pending = Entry{}
			if n, err := strconv.ParseFloat(strings.TrimSpace(seconds), 64); err == nil && n > 0 {
				pending.Length = time.Duration(n * float64(time.Second))
			}
			pending.Artist, pending.Title = split(label)
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		path, ok := location(line, folder)
		if !ok {
			read.Skipped++
			pending = Entry{}
			continue
		}
		pending.Path = path
		read.Entries = append(read.Entries, pending)
		pending = Entry{}
	}
	return read
}

// split reads a label written "Artist - Title".
func split(label string) (artist, title string) {
	label = strings.TrimSpace(label)
	if artist, title, found := strings.Cut(label, " - "); found {
		return strings.TrimSpace(artist), strings.TrimSpace(title)
	}
	return "", label
}

// PLS reads a PLS playlist's text: FileN, TitleN and LengthN of
// [playlist], in N's order.
func PLS(text, folder string) Read {
	text = strings.TrimPrefix(text, "\ufeff")
	type item struct {
		entry Entry
		file  bool
		url   bool
	}
	items := map[int]*item{}
	var order []int
	for _, line := range lines(text) {
		key, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if !found {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		var name string
		for _, prefix := range []string{"file", "title", "length"} {
			if strings.HasPrefix(key, prefix) {
				name = prefix
				break
			}
		}
		if name == "" {
			continue
		}
		n, err := strconv.Atoi(key[len(name):])
		if err != nil {
			continue
		}
		at := items[n]
		if at == nil {
			at = &item{}
			items[n] = at
			order = append(order, n)
		}
		value = strings.TrimSpace(value)
		switch name {
		case "file":
			path, ok := location(value, folder)
			at.entry.Path, at.file, at.url = path, ok, !ok
		case "title":
			at.entry.Artist, at.entry.Title = split(value)
		case "length":
			if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
				at.entry.Length = time.Duration(seconds) * time.Second
			}
		}
	}
	slices.Sort(order)
	var read Read
	for _, n := range order {
		switch at := items[n]; {
		case at.file:
			read.Entries = append(read.Entries, at.entry)
		case at.url:
			read.Skipped++
		}
	}
	return read
}

// CUE reads a CUE sheet's text: each AUDIO track of each FILE, from its
// INDEX 01 to the next track's in the same file (the last to the file's
// end), with its TITLE and PERFORMER, else the disc's.
func CUE(text, folder string) Read {
	text = strings.TrimPrefix(text, "\ufeff")
	var read Read
	var album, albumArtist, file string
	var current *Entry
	audio := false
	for _, line := range lines(text) {
		command, rest := word(strings.TrimSpace(line))
		switch strings.ToUpper(command) {
		case "FILE":
			name, _ := quoted(rest)
			path, ok := location(name, folder)
			if !ok {
				path = ""
			}
			file, current = path, nil
		case "TRACK":
			number, kind := word(rest)
			audio = strings.EqualFold(strings.TrimSpace(kind), "AUDIO")
			current = nil
			if !audio || file == "" {
				continue
			}
			n, _ := strconv.Atoi(number)
			read.Entries = append(read.Entries, Entry{Path: file, Number: n, Album: album, Artist: albumArtist, Start: -1})
			current = &read.Entries[len(read.Entries)-1]
		case "TITLE":
			value, _ := quoted(rest)
			if current != nil {
				current.Title = value
			} else if file == "" || len(read.Entries) == 0 {
				album = value
			}
		case "PERFORMER":
			value, _ := quoted(rest)
			if current != nil {
				current.Artist = value
			} else if file == "" || len(read.Entries) == 0 {
				albumArtist = value
			}
		case "INDEX":
			number, at := word(rest)
			if current == nil || number != "01" {
				continue
			}
			if start, ok := cueTime(strings.TrimSpace(at)); ok {
				current.Start = start
			}
		}
	}
	// each track ends where the next of its file starts
	var kept []Entry
	for i, entry := range read.Entries {
		if entry.Start < 0 {
			continue
		}
		if i+1 < len(read.Entries) && read.Entries[i+1].Path == entry.Path && read.Entries[i+1].Start > entry.Start {
			entry.End = read.Entries[i+1].Start
			entry.Length = entry.End - entry.Start
		}
		kept = append(kept, entry)
	}
	read.Entries = kept
	return read
}

// word cuts line's first word off the rest.
func word(line string) (string, string) {
	line = strings.TrimSpace(line)
	first, rest, _ := strings.Cut(line, " ")
	return first, strings.TrimSpace(rest)
}

// quoted is a CUE value: between quotes, else up to the first space.
func quoted(text string) (string, bool) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, `"`) {
		if end := strings.LastIndex(text, `"`); end > 0 {
			return text[1:end], true
		}
		return text[1:], true
	}
	value, _ := word(text)
	return value, value != ""
}

// cueTime is mm:ss:ff, a frame a seventy-fifth of a second.
func cueTime(text string) (time.Duration, bool) {
	parts := strings.Split(text, ":")
	if len(parts) != 3 {
		return 0, false
	}
	var n [3]int
	for i, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return 0, false
		}
		n[i] = value
	}
	if n[1] >= 60 || n[2] >= 75 {
		return 0, false
	}
	return time.Duration(n[0])*time.Minute + time.Duration(n[1])*time.Second + time.Duration(n[2])*time.Second/75, true
}

// The formats Write writes.
const (
	FormatM3U8 = "m3u8"
	FormatPLS  = "pls"
)

// Item is a track to write.
type Item struct {
	Path, Title, Artist string
	Length              time.Duration
}

// FormatOf is the format of a playlist file's name: FormatPLS for .pls,
// FormatM3U8 for .m3u and .m3u8 (written in UTF-8); "" for another.
func FormatOf(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".m3u", ".m3u8":
		return FormatM3U8
	case ".pls":
		return FormatPLS
	}
	return ""
}

// Write writes items as a playlist of format, with absolute paths: an
// error for a path holding a line's end, which neither format can hold.
func Write(w io.Writer, format string, items []Item) error {
	var b strings.Builder
	label := func(item Item) string {
		text := item.Title
		if item.Artist != "" {
			text = item.Artist + " - " + item.Title
		}
		return strings.NewReplacer("\r", " ", "\n", " ").Replace(text)
	}
	for _, item := range items {
		if strings.ContainsAny(item.Path, "\r\n") {
			return fmt.Errorf("%q: a path holding a line's end cannot be written in a playlist", item.Path)
		}
	}
	switch format {
	case FormatM3U8:
		b.WriteString("#EXTM3U\n")
		for _, item := range items {
			seconds := int64(-1)
			if item.Length > 0 {
				seconds = int64(item.Length.Seconds())
			}
			fmt.Fprintf(&b, "#EXTINF:%d,%s\n%s\n", seconds, label(item), item.Path)
		}
	case FormatPLS:
		b.WriteString("[playlist]\n")
		for i, item := range items {
			seconds := int64(-1)
			if item.Length > 0 {
				seconds = int64(item.Length.Seconds())
			}
			fmt.Fprintf(&b, "File%d=%s\nTitle%d=%s\nLength%d=%d\n", i+1, item.Path, i+1, label(item), i+1, seconds)
		}
		fmt.Fprintf(&b, "NumberOfEntries=%d\nVersion=2\n", len(items))
	default:
		return fmt.Errorf("playlist format %q: m3u8 or pls", format)
	}
	_, err := io.WriteString(w, b.String())
	return err
}
