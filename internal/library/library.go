// Package library is the music library media-player-musicd keeps: the
// music in the folders the person chose, each file's tags (every container
// internal/tags reads), browsed by artist, album, genre and year, and
// searched. The folders are read when added and at each start (a file
// read again only when its size or time changed), and watched while the
// daemon runs (goframework's vfs-watch over vfs-source-disk), so a file
// added, changed, moved or removed shows at once.
//
// The index is kept in memory and in one file (Options.Index): every
// track's text, gzip-compressed, about 40 bytes a track. It is written a
// moment after a change, whole or not at all (vfs-source-disk's WriteFile:
// a temporary file, synced, renamed over the index).
package library

import (
	"cmp"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/codemodify/media-player-music/internal/playback"
	"github.com/codemodify/media-player-music/internal/tags"
	"github.com/codemodify/media-player-music/musicapi"
	"goframework.io/vfs"
	disk "goframework.io/vfs-source-disk"
	watch "goframework.io/vfs-watch"
)

// Options make a Library.
type Options struct {
	// Index is the index file's path; "" keeps the library in memory only.
	Index string
	// Changed hears that the library changed (its State), from the
	// library's goroutines, never under its lock; nil for none.
	Changed func()
	// Watch watches the folders for changes while the library is open.
	Watch bool
	// Read reads a file's tags: internal/tags' Read by default.
	Read func(path string) (tags.Info, bool)
	// SaveDelay is how long a write of the index waits after a change:
	// two seconds by default.
	SaveDelay time.Duration
}

// entry is a track and what tells whether its file changed.
type entry struct {
	track    musicapi.Track
	size     int64
	modified int64 // Unix nanoseconds
}

// Library is the music library.
type Library struct {
	options Options
	read    func(path string) (tags.Info, bool)

	lock     sync.RWMutex
	folders  []string
	entries  map[string]*entry // by path
	version  uint64
	scanning int // scans running
	scanned  int // files read by the scans running
	err      string

	ctx      context.Context
	cancel   context.CancelFunc
	work     sync.WaitGroup
	watchers map[string]context.CancelFunc // by folder
	scans    sync.Mutex                    // one scan at a time

	saveLock sync.Mutex
	writing  sync.Mutex // one write of the index at a time
	saveAt   *time.Timer
	dirty    bool
	closed   bool
}

// ErrNotFolder is AddFolder's error for a path that is no folder.
var ErrNotFolder = errors.New("not a folder")

// Open opens the library kept in options.Index: an index that does not
// read is set aside and the library starts empty. It reads nothing of
// the folders until Start.
func Open(options Options) (*Library, error) {
	if options.Read == nil {
		options.Read = tags.Read
	}
	if options.SaveDelay <= 0 {
		options.SaveDelay = 2 * time.Second
	}
	ctx, cancel := context.WithCancel(context.Background())
	l := &Library{options: options, read: options.Read, entries: map[string]*entry{}, ctx: ctx, cancel: cancel, watchers: map[string]context.CancelFunc{}, version: 1}
	if options.Index != "" {
		if err := l.load(); err != nil {
			cancel()
			return nil, err
		}
	}
	return l, nil
}

// Start reads every folder again (a file only when it changed) and
// watches them, in the background.
func (thisRef *Library) Start() {
	thisRef.lock.RLock()
	folders := slices.Clone(thisRef.folders)
	thisRef.lock.RUnlock()
	for _, folder := range folders {
		thisRef.begin(folder)
	}
}

// begin scans a folder, then watches it, in the background.
func (thisRef *Library) begin(folder string) {
	thisRef.lock.Lock()
	thisRef.scanning++
	if _, watched := thisRef.watchers[folder]; !watched && thisRef.options.Watch {
		ctx, cancel := context.WithCancel(thisRef.ctx)
		thisRef.watchers[folder] = cancel
		thisRef.work.Add(1)
		go func() {
			defer thisRef.work.Done()
			thisRef.scan(ctx, folder, true)
			thisRef.watch(ctx, folder)
		}()
		thisRef.lock.Unlock()
		thisRef.changed()
		return
	}
	thisRef.work.Add(1)
	thisRef.lock.Unlock()
	thisRef.changed()
	go func() {
		defer thisRef.work.Done()
		thisRef.scan(thisRef.ctx, folder, true)
	}()
}

// Close stops the scans and the watchers, and writes the index.
func (thisRef *Library) Close() error {
	thisRef.cancel()
	thisRef.work.Wait()
	thisRef.saveLock.Lock()
	thisRef.closed = true
	if thisRef.saveAt != nil {
		thisRef.saveAt.Stop()
	}
	thisRef.saveLock.Unlock()
	thisRef.writing.Lock() // a write running, ended
	thisRef.writing.Unlock()
	thisRef.saveLock.Lock()
	dirty := thisRef.dirty && thisRef.options.Index != ""
	thisRef.saveLock.Unlock()
	if dirty {
		return thisRef.save()
	}
	return nil
}

// Wait waits for the scans running to end: for tests.
func (thisRef *Library) Wait(ctx context.Context) error {
	for {
		thisRef.lock.RLock()
		scanning := thisRef.scanning
		thisRef.lock.RUnlock()
		if scanning == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// changed tells the library's listener and arms the index's write.
func (thisRef *Library) changed() {
	thisRef.saveLock.Lock()
	if !thisRef.closed && thisRef.options.Index != "" {
		thisRef.dirty = true
		if thisRef.saveAt == nil {
			thisRef.saveAt = time.AfterFunc(thisRef.options.SaveDelay, func() {
				if err := thisRef.save(); err != nil {
					log.Printf("library: writing the index: %v", err)
				}
			})
		} else {
			thisRef.saveAt.Reset(thisRef.options.SaveDelay)
		}
	}
	thisRef.saveLock.Unlock()
	if thisRef.options.Changed != nil {
		thisRef.options.Changed()
	}
}

// ---- the folders ------------------------------------------------------------

// Folders are the folders the library holds the music of.
func (thisRef *Library) Folders() []string {
	thisRef.lock.RLock()
	defer thisRef.lock.RUnlock()
	return slices.Clone(thisRef.folders)
}

// AddFolder adds a folder, read in the background: an error for one that
// is not there or no folder. A folder within one held, or holding one, is
// taken as the larger.
func (thisRef *Library) AddFolder(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s: %w", absolute, ErrNotFolder)
	}
	thisRef.lock.Lock()
	for _, folder := range thisRef.folders {
		if within(absolute, folder) {
			thisRef.lock.Unlock()
			return nil // held already
		}
	}
	var inner []string
	thisRef.folders = slices.DeleteFunc(thisRef.folders, func(folder string) bool {
		if within(folder, absolute) {
			inner = append(inner, folder)
			return true
		}
		return false
	})
	thisRef.folders = append(thisRef.folders, absolute)
	slices.Sort(thisRef.folders)
	for _, folder := range inner {
		if cancel, ok := thisRef.watchers[folder]; ok {
			cancel()
			delete(thisRef.watchers, folder)
		}
	}
	thisRef.version++
	thisRef.lock.Unlock()
	thisRef.begin(absolute)
	return nil
}

// RemoveFolder takes a folder and its music out of the library.
func (thisRef *Library) RemoveFolder(path string) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return
	}
	thisRef.lock.Lock()
	before := len(thisRef.folders)
	thisRef.folders = slices.DeleteFunc(thisRef.folders, func(folder string) bool { return folder == absolute })
	if len(thisRef.folders) == before {
		thisRef.lock.Unlock()
		return
	}
	if cancel, ok := thisRef.watchers[absolute]; ok {
		cancel()
		delete(thisRef.watchers, absolute)
	}
	thisRef.removeWithinLocked(absolute)
	thisRef.version++
	thisRef.lock.Unlock()
	thisRef.changed()
}

// Rescan reads every folder again, in the background.
func (thisRef *Library) Rescan() {
	thisRef.lock.RLock()
	folders := slices.Clone(thisRef.folders)
	thisRef.lock.RUnlock()
	for _, folder := range folders {
		thisRef.lock.Lock()
		thisRef.scanning++
		thisRef.work.Add(1)
		thisRef.lock.Unlock()
		go func() {
			defer thisRef.work.Done()
			thisRef.scan(thisRef.ctx, folder, true)
		}()
	}
	thisRef.changed()
}

// within reports whether path is folder or within it.
func within(path, folder string) bool {
	if path == folder {
		return true
	}
	rel, err := filepath.Rel(folder, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// removeWithinLocked takes out the tracks within path (path's own too).
func (thisRef *Library) removeWithinLocked(path string) bool {
	removed := false
	for each := range thisRef.entries {
		if within(each, path) {
			delete(thisRef.entries, each)
			removed = true
		}
	}
	return removed
}

// held reports whether a path is within a folder of the library.
func (thisRef *Library) heldLocked(path string) bool {
	for _, folder := range thisRef.folders {
		if within(path, folder) {
			return true
		}
	}
	return false
}

// ---- reading the folders ------------------------------------------------------

// batch is how many files a scan reads before it tells what it found.
const batch = 200

// scan reads folder (counted in scanning by its caller, which it ends):
// each file the player plays, its tags read again only when its size or
// time changed; the tracks of files gone are taken out. whole says folder
// is a folder of the library (the tracks within not seen are gone);
// otherwise a folder within one, moved in.
func (thisRef *Library) scan(ctx context.Context, folder string, whole bool) {
	thisRef.scans.Lock()
	defer thisRef.scans.Unlock()
	defer func() {
		thisRef.lock.Lock()
		thisRef.scanning--
		if thisRef.scanning == 0 {
			thisRef.scanned = 0
		}
		thisRef.version++
		thisRef.lock.Unlock()
		thisRef.changed()
	}()
	seen := map[string]bool{}
	var found []*entry
	flush := func() {
		if len(found) == 0 {
			return
		}
		thisRef.lock.Lock()
		for _, each := range found {
			if thisRef.heldLocked(each.track.Path) {
				thisRef.entries[each.track.Path] = each
			}
		}
		thisRef.scanned += len(found)
		thisRef.version++
		thisRef.lock.Unlock()
		found = found[:0]
		thisRef.changed()
	}
	err := filepath.WalkDir(folder, func(path string, item fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			if item != nil && item.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if item.IsDir() || !playback.Playable(path) {
			return nil
		}
		info, err := os.Stat(path) // follows a link to a file
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		seen[path] = true
		thisRef.lock.RLock()
		known := thisRef.entries[path]
		thisRef.lock.RUnlock()
		if known != nil && known.size == info.Size() && known.modified == info.ModTime().UnixNano() {
			return nil
		}
		found = append(found, thisRef.entryOf(path, info))
		if len(found) >= batch {
			flush()
		}
		return nil
	})
	flush()
	if ctx.Err() != nil {
		return
	}
	thisRef.lock.Lock()
	if err != nil {
		thisRef.err = err.Error()
	} else if whole {
		thisRef.err = ""
	}
	if whole || err == nil {
		for path := range thisRef.entries {
			if within(path, folder) && !seen[path] {
				delete(thisRef.entries, path)
			}
		}
	}
	thisRef.lock.Unlock()
}

// entryOf reads a file's tags.
func (thisRef *Library) entryOf(path string, info os.FileInfo) *entry {
	read, _ := thisRef.read(path)
	title := read.Title
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	return &entry{
		track: musicapi.Track{Path: path, Title: title, Artist: read.Artist, Album: read.Album, LengthMS: read.Length.Milliseconds(),
			AlbumArtist: read.AlbumArtist, Genre: read.Genre, Year: read.Year, Number: read.Number, Disc: read.Disc},
		size: info.Size(), modified: info.ModTime().UnixNano(),
	}
}

// watch follows folder's changes until ctx ends.
func (thisRef *Library) watch(ctx context.Context, folder string) {
	fsys, err := disk.Open(folder)
	if err != nil {
		thisRef.fail(folder, err)
		return
	}
	watcher, err := watch.Start(ctx, fsys, ".", watch.Recursive(), watch.Settle(300*time.Millisecond))
	if err != nil {
		thisRef.fail(folder, err)
		return
	}
	defer watcher.Close()
	for {
		select {
		case <-ctx.Done():
			return
		case batch, ok := <-watcher.Changes():
			if !ok {
				if err := watcher.Err(); err != nil && ctx.Err() == nil {
					thisRef.fail(folder, err)
				}
				return
			}
			thisRef.apply(ctx, folder, batch)
		}
	}
}

func (thisRef *Library) fail(folder string, err error) {
	thisRef.lock.Lock()
	thisRef.err = folder + ": " + err.Error()
	thisRef.version++
	thisRef.lock.Unlock()
	thisRef.changed()
}

// apply takes a watcher's batch: files made or changed read, those gone
// taken out, a folder moved in read whole.
func (thisRef *Library) apply(ctx context.Context, folder string, batch watch.Batch) {
	var folders []string
	thisRef.lock.Lock()
	for _, change := range batch.Changes {
		path := filepath.Join(folder, filepath.FromSlash(change.Path))
		switch change.Kind {
		case watch.Removed:
			thisRef.removeWithinLocked(path)
			continue
		case watch.Renamed:
			thisRef.removeWithinLocked(filepath.Join(folder, filepath.FromSlash(change.From)))
		}
		if change.Entry.IsDir() {
			if change.Kind == watch.Renamed {
				folders = append(folders, path)
			}
			continue
		}
		if !playback.Playable(path) {
			continue
		}
		thisRef.lock.Unlock()
		info, err := os.Stat(path)
		var found *entry
		if err == nil && info.Mode().IsRegular() {
			found = thisRef.entryOf(path, info)
		}
		thisRef.lock.Lock()
		if found != nil && thisRef.heldLocked(path) {
			thisRef.entries[path] = found
		}
	}
	thisRef.version++
	thisRef.lock.Unlock()
	thisRef.changed()
	for _, each := range folders {
		thisRef.lock.Lock()
		thisRef.scanning++
		thisRef.lock.Unlock()
		thisRef.scan(ctx, each, false)
	}
}

// ---- what the renderers ask ---------------------------------------------------

// State is the library in short.
func (thisRef *Library) State() musicapi.Library {
	thisRef.lock.RLock()
	defer thisRef.lock.RUnlock()
	artists, albums, genres := map[string]bool{}, map[[2]string]bool{}, map[string]bool{}
	var length int64
	for _, each := range thisRef.entries {
		artists[artistOf(each.track)] = true
		albums[[2]string{artistOf(each.track), each.track.Album}] = true
		if each.track.Genre != "" {
			genres[each.track.Genre] = true
		}
		length += each.track.LengthMS
	}
	return musicapi.Library{Version: thisRef.version, Folders: slices.Clone(thisRef.folders), Tracks: len(thisRef.entries),
		Artists: len(artists), Albums: len(albums), Genres: len(genres), LengthMS: length,
		Scanning: thisRef.scanning > 0, Scanned: thisRef.scanned, Error: thisRef.err}
}

// artistOf is the artist a track is browsed under: its album's artist,
// else its own.
func artistOf(t musicapi.Track) string {
	if t.AlbumArtist != "" {
		return t.AlbumArtist
	}
	return t.Artist
}

// matches reports whether a track is one of browse's: of its artist (the
// album's or the track's own), album, genre and year, those given, and
// without the fields named Unknown.
func matches(t musicapi.Track, browse musicapi.Browse) bool {
	if browse.Artist != "" && !strings.EqualFold(artistOf(t), browse.Artist) && !strings.EqualFold(t.Artist, browse.Artist) {
		return false
	}
	if browse.Album != "" && !strings.EqualFold(t.Album, browse.Album) {
		return false
	}
	if browse.Genre != "" && !strings.EqualFold(t.Genre, browse.Genre) {
		return false
	}
	if browse.Year != 0 && t.Year != browse.Year {
		return false
	}
	for _, field := range browse.Unknown {
		switch field {
		case "artist":
			if artistOf(t) != "" {
				return false
			}
		case "album":
			if t.Album != "" {
				return false
			}
		case "genre":
			if t.Genre != "" {
				return false
			}
		case "year":
			if t.Year != 0 {
				return false
			}
		}
	}
	return true
}

// order is the library's order of tracks: by artist, album, disc, number,
// then title and path.
func order(a, b musicapi.Track) int {
	return cmp.Or(compareText(artistOf(a), artistOf(b)), compareText(a.Album, b.Album), cmp.Compare(a.Disc, b.Disc),
		cmp.Compare(a.Number, b.Number), compareText(a.Title, b.Title), strings.Compare(a.Path, b.Path))
}

// compareText compares names as a person sorts them: without case, a
// leading "The " left out, "" (unknown) last.
func compareText(a, b string) int {
	switch {
	case a == "" && b == "":
		return 0
	case a == "":
		return 1
	case b == "":
		return -1
	}
	return cmp.Or(strings.Compare(sortKey(a), sortKey(b)), strings.Compare(a, b))
}

func sortKey(name string) string {
	name = fold(name)
	if rest, ok := strings.CutPrefix(name, "the "); ok && rest != "" {
		return rest
	}
	return name
}

// Browse lists the library: the groups of browse.By (artists, albums,
// genres, years) or its tracks, among the tracks of its artist, album,
// genre and year, those given; Offset and Limit page them.
func (thisRef *Library) Browse(browse musicapi.Browse) (musicapi.Browsed, error) {
	thisRef.lock.RLock()
	var tracks []musicapi.Track
	for _, each := range thisRef.entries {
		if matches(each.track, browse) {
			tracks = append(tracks, each.track)
		}
	}
	thisRef.lock.RUnlock()
	var result musicapi.Browsed
	switch browse.By {
	case musicapi.BrowseTracks, "":
		slices.SortFunc(tracks, order)
		result.Total = len(tracks)
		result.Tracks = page(tracks, browse.Offset, browse.Limit)
		return result, nil
	case musicapi.BrowseArtists, musicapi.BrowseAlbums, musicapi.BrowseGenres, musicapi.BrowseYears:
		groups := group(tracks, browse.By)
		result.Total = len(groups)
		result.Groups = page(groups, browse.Offset, browse.Limit)
		return result, nil
	}
	return musicapi.Browsed{}, fmt.Errorf("browse by %q: by artists, albums, genres, years or tracks", browse.By)
}

// group makes tracks groups of by, in order.
func group(tracks []musicapi.Track, by string) []musicapi.Group {
	type key struct {
		name, artist string
		year         int
	}
	groups := map[key]*musicapi.Group{}
	for _, t := range tracks {
		var k key
		switch by {
		case musicapi.BrowseArtists:
			k = key{name: artistOf(t)}
		case musicapi.BrowseAlbums:
			k = key{name: t.Album, artist: artistOf(t)}
		case musicapi.BrowseGenres:
			k = key{name: t.Genre}
		case musicapi.BrowseYears:
			k = key{name: yearText(t.Year), year: t.Year}
		}
		g := groups[k]
		if g == nil {
			g = &musicapi.Group{Name: k.name, Artist: k.artist, Year: k.year}
			groups[k] = g
		}
		g.Tracks++
		g.LengthMS += t.LengthMS
		if by == musicapi.BrowseAlbums && t.Year > g.Year {
			g.Year = t.Year
		}
	}
	list := make([]musicapi.Group, 0, len(groups))
	for _, g := range groups {
		list = append(list, *g)
	}
	slices.SortFunc(list, func(a, b musicapi.Group) int {
		if by == musicapi.BrowseYears {
			return cmp.Compare(b.Year, a.Year) // the newest first
		}
		return cmp.Or(compareText(a.Name, b.Name), compareText(a.Artist, b.Artist))
	})
	return list
}

func yearText(year int) string {
	if year == 0 {
		return ""
	}
	return strconv.Itoa(year)
}

func page[T any](list []T, offset, limit int) []T {
	offset = min(max(offset, 0), len(list))
	list = list[offset:]
	if limit > 0 && limit < len(list) {
		list = list[:limit]
	}
	return list
}

// Search finds what was typed in the library: each word or "phrase" in a
// track's title, artist, album, album's artist or genre, without case or
// accents; artist:, album:, genre:, title: and year: keep to that field.
// The artists and albums whose own names hold every word come first, then
// the tracks, at most Limit of each (50 by default).
func (thisRef *Library) Search(given musicapi.Search) (musicapi.Found, error) {
	query, err := parse(given.Text)
	if err != nil {
		return musicapi.Found{}, err
	}
	limit := given.Limit
	if limit <= 0 {
		limit = 50
	}
	if query.empty() {
		return musicapi.Found{}, nil
	}
	thisRef.lock.RLock()
	var tracks []musicapi.Track
	for _, each := range thisRef.entries {
		if query.track(each.track) {
			tracks = append(tracks, each.track)
		}
	}
	thisRef.lock.RUnlock()
	slices.SortFunc(tracks, order)
	var found musicapi.Found
	for _, g := range group(tracks, musicapi.BrowseArtists) {
		if g.Name != "" && query.name(g.Name) {
			found.Artists = append(found.Artists, g)
		}
	}
	for _, g := range group(tracks, musicapi.BrowseAlbums) {
		if g.Name != "" && query.name(g.Name) {
			found.Albums = append(found.Albums, g)
		}
	}
	found.More = len(tracks) > limit || len(found.Artists) > limit || len(found.Albums) > limit
	found.Artists, found.Albums, found.Tracks = page(found.Artists, 0, limit), page(found.Albums, 0, limit), page(tracks, 0, limit)
	return found, nil
}

// Tracks are the library's tracks of paths, in their order: for a
// renderer that queues what it browsed. A path the library does not hold
// is left out.
func (thisRef *Library) Tracks(paths []string) []musicapi.Track {
	thisRef.lock.RLock()
	defer thisRef.lock.RUnlock()
	var tracks []musicapi.Track
	for _, path := range paths {
		if each := thisRef.entries[path]; each != nil {
			tracks = append(tracks, each.track)
		}
	}
	return tracks
}

// ---- the index ---------------------------------------------------------------

// indexVersion is the index file's format.
const indexVersion = 1

// index is the index file, gzip-compressed JSON: each track a row of
// path, title, artist, album, album artist, genre, year, number, disc,
// length (ms), size and time (Unix ns).
type index struct {
	Version int      `json:"version"`
	Folders []string `json:"folders"`
	Tracks  [][]any  `json:"tracks"`
}

func (thisRef *Library) load() error {
	file, err := os.Open(thisRef.options.Index)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	kept, err := decode(file)
	if err != nil {
		aside := thisRef.options.Index + ".damaged"
		log.Printf("library: the index %s does not read (%v): set aside as %s, starting anew", thisRef.options.Index, err, aside)
		file.Close()
		return os.Rename(thisRef.options.Index, aside)
	}
	thisRef.folders = kept.Folders
	for _, row := range kept.Tracks {
		if each, ok := fromRow(row); ok {
			thisRef.entries[each.track.Path] = each
		}
	}
	return nil
}

func decode(reader io.Reader) (index, error) {
	unzipped, err := gzip.NewReader(reader)
	if err != nil {
		return index{}, err
	}
	var kept index
	decoder := json.NewDecoder(unzipped)
	decoder.UseNumber()
	if err := decoder.Decode(&kept); err != nil {
		return index{}, err
	}
	if kept.Version != indexVersion {
		return index{}, fmt.Errorf("index version %d", kept.Version)
	}
	return kept, nil
}

func (thisRef *Library) save() (err error) {
	thisRef.writing.Lock()
	defer thisRef.writing.Unlock()
	thisRef.saveLock.Lock()
	thisRef.dirty = false
	thisRef.saveLock.Unlock()
	defer func() {
		if err != nil { // written at the next change, or at Close
			thisRef.saveLock.Lock()
			thisRef.dirty = true
			thisRef.saveLock.Unlock()
		}
	}()
	thisRef.lock.RLock()
	kept := index{Version: indexVersion, Folders: slices.Clone(thisRef.folders), Tracks: make([][]any, 0, len(thisRef.entries))}
	for _, each := range thisRef.entries {
		kept.Tracks = append(kept.Tracks, row(each))
	}
	thisRef.lock.RUnlock()
	slices.SortFunc(kept.Tracks, func(a, b []any) int { return strings.Compare(a[0].(string), b[0].(string)) })
	return disk.Write(thisRef.options.Index, func(writer io.Writer) error {
		zipped := gzip.NewWriter(writer)
		if err := json.NewEncoder(zipped).Encode(kept); err != nil {
			return err
		}
		return zipped.Close()
	}, vfs.Mode(0o600))
}

func row(each *entry) []any {
	t := each.track
	return []any{t.Path, t.Title, t.Artist, t.Album, t.AlbumArtist, t.Genre, t.Year, t.Number, t.Disc, t.LengthMS, each.size, each.modified}
}

func fromRow(row []any) (*entry, bool) {
	if len(row) != 12 {
		return nil, false
	}
	text := func(i int) string { s, _ := row[i].(string); return s }
	number := func(i int) int64 {
		n, _ := row[i].(json.Number)
		v, _ := n.Int64()
		return v
	}
	path := text(0)
	if path == "" || !filepath.IsAbs(path) {
		return nil, false
	}
	return &entry{track: musicapi.Track{Path: path, Title: text(1), Artist: text(2), Album: text(3), AlbumArtist: text(4), Genre: text(5),
		Year: int(number(6)), Number: int(number(7)), Disc: int(number(8)), LengthMS: number(9)}, size: number(10), modified: number(11)}, true
}
