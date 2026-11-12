package library

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/codemodify/media-player-music/internal/tags"
	"github.com/codemodify/media-player-music/internal/tags/tagstest"
	"github.com/codemodify/media-player-music/musicapi"
)

// audio is a second of real MP3 (the engine's test file).
func audio(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "playback", "testdata", "lame-mono22.mp3"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// song writes an MP3 with an ID3v2.4 tag of the fields given (title,
// artist, album, band (the album's artist), genre, track, year).
func song(t *testing.T, path string, fields map[string]string) string {
	t.Helper()
	ids := map[string]string{"title": "TIT2", "artist": "TPE1", "album": "TALB", "band": "TPE2", "genre": "TCON", "track": "TRCK", "year": "TDRC"}
	var frames []tagstest.Frame
	for _, name := range []string{"title", "artist", "album", "band", "genre", "track", "year"} {
		if value, ok := fields[name]; ok {
			frames = append(frames, tagstest.Frame{ID: ids[name], Data: tagstest.Text(4, value)})
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Join([][]byte{tagstest.V2(4, frames...), audio(t)}, nil), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// collection writes a small library into a folder.
func collection(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	song(t, filepath.Join(root, "Tessellators", "Low Tide", "01.mp3"), map[string]string{"title": "Harbour Lights", "artist": "The Tessellators", "album": "Low Tide", "genre": "Ambient", "track": "1/3", "year": "2019"})
	song(t, filepath.Join(root, "Tessellators", "Low Tide", "02.mp3"), map[string]string{"title": "Nine Volt", "artist": "The Tessellators", "album": "Low Tide", "genre": "Ambient", "track": "2/3", "year": "2019"})
	song(t, filepath.Join(root, "Alder", "Field Notes", "01.mp3"), map[string]string{"title": "Paper Anemometer", "artist": "Ruth Alder", "album": "Field Notes", "genre": "Folk", "track": "1", "year": "2021"})
	song(t, filepath.Join(root, "Mix", "01.mp3"), map[string]string{"title": "Beyoncé's Café", "artist": "Ostro", "band": "Various Artists", "album": "Dune Mix", "genre": "Folk", "track": "4", "year": "2021"})
	song(t, filepath.Join(root, "untagged.mp3"), nil)
	if err := os.WriteFile(filepath.Join(root, "cover.jpg"), []byte("not music"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func open(t *testing.T, options Options) *Library {
	t.Helper()
	l, err := Open(options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	return l
}

func wait(t *testing.T, l *Library) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := l.Wait(ctx); err != nil {
		t.Fatal(err)
	}
}

func names(groups []musicapi.Group) []string {
	var list []string
	for _, g := range groups {
		list = append(list, g.Name)
	}
	return list
}

func titles(tracks []musicapi.Track) []string {
	var list []string
	for _, each := range tracks {
		list = append(list, each.Title)
	}
	return list
}

func TestBrowseByArtistAlbumGenreAndYear(t *testing.T) {
	root := collection(t)
	l := open(t, Options{})
	if err := l.AddFolder(root); err != nil {
		t.Fatal(err)
	}
	wait(t, l)
	state := l.State()
	if state.Tracks != 5 || state.Artists != 4 || state.Albums != 4 || state.Genres != 2 || state.Scanning || state.LengthMS < 4500 || !slices.Equal(state.Folders, []string{root}) {
		t.Fatalf("state: %+v", state)
	}
	browse := func(b musicapi.Browse) musicapi.Browsed {
		t.Helper()
		found, err := l.Browse(b)
		if err != nil {
			t.Fatal(err)
		}
		return found
	}
	// names as a person sorts them: "The" left out, the unknown last
	if got := names(browse(musicapi.Browse{By: musicapi.BrowseArtists}).Groups); !slices.Equal(got, []string{"Ruth Alder", "The Tessellators", "Various Artists", ""}) {
		t.Fatalf("artists: %q", got)
	}
	albums := browse(musicapi.Browse{By: musicapi.BrowseAlbums, Genre: "folk"})
	if got := names(albums.Groups); !slices.Equal(got, []string{"Dune Mix", "Field Notes"}) || albums.Groups[0].Artist != "Various Artists" || albums.Groups[1].Year != 2021 {
		t.Fatalf("folk albums: %+v", albums.Groups)
	}
	// an album's tracks in its order; a track's own artist finds it in a compilation
	if got := titles(browse(musicapi.Browse{By: musicapi.BrowseTracks, Artist: "the tessellators", Album: "Low Tide"}).Tracks); !slices.Equal(got, []string{"Harbour Lights", "Nine Volt"}) {
		t.Fatalf("an album's tracks: %q", got)
	}
	if got := titles(browse(musicapi.Browse{By: musicapi.BrowseTracks, Artist: "Ostro"}).Tracks); !slices.Equal(got, []string{"Beyoncé's Café"}) {
		t.Fatalf("an artist of a compilation: %q", got)
	}
	years := browse(musicapi.Browse{By: musicapi.BrowseYears})
	if got := names(years.Groups); !slices.Equal(got, []string{"2021", "2019", ""}) || years.Groups[0].Tracks != 2 {
		t.Fatalf("years: %+v", years.Groups)
	}
	if got := names(browse(musicapi.Browse{By: musicapi.BrowseGenres}).Groups); !slices.Equal(got, []string{"Ambient", "Folk", ""}) {
		t.Fatalf("genres: %q", got)
	}
	if got := titles(browse(musicapi.Browse{By: musicapi.BrowseTracks, Unknown: []string{"artist"}}).Tracks); !slices.Equal(got, []string{"untagged"}) {
		t.Fatalf("the unknown artist's tracks: %q", got)
	}
	if got := names(browse(musicapi.Browse{By: musicapi.BrowseAlbums, Unknown: []string{"genre", "year"}}).Groups); !slices.Equal(got, []string{""}) {
		t.Fatalf("the albums of no genre and no year: %q", got)
	}
	paged := browse(musicapi.Browse{By: musicapi.BrowseTracks, Offset: 1, Limit: 2})
	if paged.Total != 5 || len(paged.Tracks) != 2 || paged.Tracks[0].Title != "Harbour Lights" {
		t.Fatalf("a page: %+v", paged)
	}
	if _, err := l.Browse(musicapi.Browse{By: "moods"}); err == nil {
		t.Fatal("browsed by moods")
	}
	if got := l.Tracks([]string{filepath.Join(root, "Mix", "01.mp3"), "/not/held.mp3"}); len(got) != 1 || got[0].AlbumArtist != "Various Artists" || got[0].Number != 4 {
		t.Fatalf("Tracks: %+v", got)
	}
}

func TestSearchFindsWordsWithoutCaseOrAccents(t *testing.T) {
	l := open(t, Options{})
	if err := l.AddFolder(collection(t)); err != nil {
		t.Fatal(err)
	}
	wait(t, l)
	search := func(text string) musicapi.Found {
		t.Helper()
		found, err := l.Search(musicapi.Search{Text: text})
		if err != nil {
			t.Fatal(err)
		}
		return found
	}
	if found := search("beyonce cafe"); !slices.Equal(titles(found.Tracks), []string{"Beyoncé's Café"}) {
		t.Fatalf("without accents: %+v", found)
	}
	found := search("TESSELL")
	if !slices.Equal(names(found.Artists), []string{"The Tessellators"}) || len(found.Albums) != 0 || len(found.Tracks) != 2 {
		t.Fatalf("an artist's name: %+v", found)
	}
	if found := search(`"field notes"`); !slices.Equal(names(found.Albums), []string{"Field Notes"}) || len(found.Tracks) != 1 {
		t.Fatalf("a phrase: %+v", found)
	}
	if found := search("genre:folk year:2021 artist:alder"); !slices.Equal(titles(found.Tracks), []string{"Paper Anemometer"}) {
		t.Fatalf("operators: %+v", found)
	}
	if found := search("title:volt"); !slices.Equal(titles(found.Tracks), []string{"Nine Volt"}) {
		t.Fatalf("title: %+v", found)
	}
	if found := search("nothing-like-this"); len(found.Tracks)+len(found.Artists)+len(found.Albums) != 0 {
		t.Fatalf("found %+v", found)
	}
	if found := search("   "); found.Tracks != nil {
		t.Fatal("an empty search found something")
	}
	if _, err := l.Search(musicapi.Search{Text: "year:recent"}); err == nil {
		t.Fatal("a year that is no number")
	}
	if found, _ := l.Search(musicapi.Search{Text: "mp3 OR *", Limit: 1}); found.More {
		t.Fatalf("what a person types is no language: %+v", found)
	}
}

// countReads reads tags as the library does, counting the files read.
func countReads(reads *atomic.Int32) func(string) (tags.Info, bool) {
	return func(path string) (tags.Info, bool) {
		reads.Add(1)
		return tags.Read(path)
	}
}

func TestTheIndexIsKeptAndOnlyChangedFilesAreReadAgain(t *testing.T) {
	root := collection(t)
	index := filepath.Join(t.TempDir(), "state", "library.index")
	var reads atomic.Int32
	options := Options{Index: index, SaveDelay: time.Millisecond, Read: countReads(&reads)}
	l, err := Open(options)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.AddFolder(root); err != nil {
		t.Fatal(err)
	}
	wait(t, l)
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(index)
	if err != nil || info.Mode().Perm() != 0o600 || info.Size() > 2000 {
		t.Fatalf("the index: %v, %v", info, err)
	}
	if reads.Load() != 5 {
		t.Fatalf("%d files read", reads.Load())
	}
	// a file changed, one removed, one added, while the daemon was not running
	time.Sleep(10 * time.Millisecond)
	song(t, filepath.Join(root, "Mix", "01.mp3"), map[string]string{"title": "Retitled", "artist": "Ostro"})
	if err := os.Remove(filepath.Join(root, "untagged.mp3")); err != nil {
		t.Fatal(err)
	}
	song(t, filepath.Join(root, "Alder", "Field Notes", "02.mp3"), map[string]string{"title": "Beacon", "artist": "Ruth Alder", "album": "Field Notes"})
	reads.Store(0)
	again := open(t, options)
	if state := again.State(); state.Tracks != 5 || !slices.Equal(state.Folders, []string{root}) {
		t.Fatalf("the index read back: %+v", state)
	}
	again.Start()
	wait(t, again)
	if reads.Load() != 2 {
		t.Fatalf("%d files read again, want the two changed", reads.Load())
	}
	all, _ := again.Browse(musicapi.Browse{By: musicapi.BrowseTracks})
	if got := titles(all.Tracks); !slices.Contains(got, "Retitled") || !slices.Contains(got, "Beacon") || len(got) != 5 {
		t.Fatalf("after the changes: %q", got)
	}
	// a damaged index is set aside, and the library starts anew
	again.Close()
	if err := os.WriteFile(index, []byte("not gzip"), 0o600); err != nil {
		t.Fatal(err)
	}
	fresh := open(t, Options{Index: index})
	if fresh.State().Tracks != 0 {
		t.Fatal("a damaged index was read")
	}
	if _, err := os.Stat(index + ".damaged"); err != nil {
		t.Fatal(err)
	}
}

func TestFoldersAreWatched(t *testing.T) {
	root := collection(t)
	changes := make(chan struct{}, 1000)
	l := open(t, Options{Watch: true, Changed: func() {
		select {
		case changes <- struct{}{}:
		default:
		}
	}})
	if err := l.AddFolder(root); err != nil {
		t.Fatal(err)
	}
	wait(t, l)
	until := func(what string, done func(musicapi.Library) bool) {
		t.Helper()
		deadline := time.Now().Add(15 * time.Second)
		for !done(l.State()) {
			if time.Now().After(deadline) {
				t.Fatalf("%s: %+v", what, l.State())
			}
			select {
			case <-changes:
			case <-time.After(50 * time.Millisecond):
			}
		}
	}
	time.Sleep(300 * time.Millisecond) // the watcher's first snapshot
	song(t, filepath.Join(root, "New", "01.mp3"), map[string]string{"title": "Fresh", "artist": "Newcomer"})
	until("a file added", func(s musicapi.Library) bool { return s.Tracks == 6 && s.Artists == 5 })
	if err := os.Rename(filepath.Join(root, "Tessellators"), filepath.Join(root, "The Tessellators")); err != nil {
		t.Fatal(err)
	}
	until("a folder renamed", func(musicapi.Library) bool {
		found, _ := l.Browse(musicapi.Browse{By: musicapi.BrowseTracks, Album: "Low Tide"})
		return len(found.Tracks) == 2 && filepath.Base(filepath.Dir(filepath.Dir(found.Tracks[0].Path))) == "The Tessellators"
	})
	if err := os.RemoveAll(filepath.Join(root, "Alder")); err != nil {
		t.Fatal(err)
	}
	until("a folder removed", func(s musicapi.Library) bool { return s.Tracks == 5 })
	l.RemoveFolder(root)
	if state := l.State(); state.Tracks != 0 || len(state.Folders) != 0 {
		t.Fatalf("a folder taken out: %+v", state)
	}
	if err := l.AddFolder(filepath.Join(root, "Mix", "01.mp3")); err == nil {
		t.Fatal("a file added as a folder")
	}
}
