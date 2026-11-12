package service

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/codemodify/media-player-music/internal/model"
	"github.com/codemodify/media-player-music/internal/playback"
	"github.com/codemodify/media-player-music/internal/playlist"
	"github.com/codemodify/media-player-music/internal/tags"
)

// Expanded is what Expand found.
type Expanded struct {
	Tracks      []model.Track
	skipped     int
	firstError  error
	unsupported map[string]int // files in a format the engine does not play, by format
	streams     int            // a playlist's entries that are streams
}

// Skipped says what was left out, in words; "" when nothing was.
func (thisRef Expanded) Skipped() string {
	var skipped []string
	if thisRef.skipped > 0 {
		skipped = append(skipped, fmt.Sprintf("skipped %d unreadable entries: %v", thisRef.skipped, thisRef.firstError))
	}
	if len(thisRef.unsupported) > 0 {
		skipped = append(skipped, unsupportedText(thisRef.unsupported))
	}
	if thisRef.streams > 0 {
		skipped = append(skipped, fmt.Sprintf("left out %d streams (the player plays files)", thisRef.streams))
	}
	return strings.Join(skipped, "; ")
}

// unsupportedText says how many files were left out for their format.
func unsupportedText(unsupported map[string]int) string {
	count, formats := 0, make([]string, 0, len(unsupported))
	for format, n := range unsupported {
		count += n
		formats = append(formats, format)
	}
	slices.Sort(formats)
	files := "files"
	if count == 1 {
		files = "file"
	}
	return fmt.Sprintf("left out %d %s in formats the player does not play (%s)", count, files, strings.Join(formats, ", "))
}

func (thisRef *Expanded) skip(err error) {
	thisRef.skipped++
	if thisRef.firstError == nil {
		thisRef.firstError = err
	}
}

// Expand makes paths tracks: an explicit file is a track unless its
// extension names a format the engine does not play (WMA, APE...), which
// is an error; a folder gives the files the engine plays within,
// recursively, in name order (audio files, and the audio of MP4,
// Matroska and WebM videos). A bad explicit path is an error before
// anything changes; unreadable entries within a folder are skipped and
// said. Folders are walked without following their links to folders, so
// that a cycle cannot recurse forever; links to files are followed.
func Expand(ctx context.Context, paths []string) (Expanded, error) {
	var result Expanded
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return Expanded{}, err
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return Expanded{}, err
		}
		info, err := os.Stat(absolute)
		if err != nil {
			return Expanded{}, fmt.Errorf("open music path: %w", err)
		}
		if !info.IsDir() && playlist.Is(absolute) {
			if err := result.playlist(ctx, absolute); err != nil {
				return Expanded{}, err
			}
			continue
		}
		if !info.IsDir() {
			if _, unsupported := playback.UnsupportedFormat(absolute); unsupported {
				return Expanded{}, errors.New(playback.UnsupportedMessage(absolute))
			}
			track, err := Track(absolute)
			if err != nil {
				return Expanded{}, err
			}
			result.Tracks = append(result.Tracks, track)
			continue
		}
		// A folder given through a link is a valid root: resolved once;
		// links to folders within are still not followed.
		root, err := filepath.EvalSymlinks(absolute)
		if err != nil {
			return Expanded{}, fmt.Errorf("open music folder: %w", err)
		}
		err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if walkErr != nil {
				result.skip(walkErr)
				if entry != nil && entry.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if entry.IsDir() {
				return nil
			}
			if format, unsupported := playback.UnsupportedFormat(path); unsupported {
				if result.unsupported == nil {
					result.unsupported = map[string]int{}
				}
				result.unsupported[format]++
				return nil
			}
			if !playback.Playable(path) {
				return nil
			}
			info, err := os.Stat(path) // follows a link to a file; one to a folder is left out
			if err != nil {
				result.skip(err)
				return nil
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			track, err := Track(path)
			if err != nil {
				result.skip(err)
				return nil
			}
			result.Tracks = append(result.Tracks, track)
			return nil
		})
		if err != nil {
			return Expanded{}, fmt.Errorf("scan music folder: %w", err)
		}
	}
	if len(result.Tracks) == 0 && len(paths) > 0 {
		if len(result.unsupported) > 0 {
			return Expanded{}, errors.New("no files the player plays: " + unsupportedText(result.unsupported) + "; it plays " + playback.Formats)
		}
		if result.firstError != nil {
			return Expanded{}, fmt.Errorf("no readable audio files found: %w", result.firstError)
		}
		return Expanded{}, errors.New("no audio files found in the selected folders")
	}
	return result, nil
}

// FromTags is a track of the queue as a file's tags say: its title, else
// the file's name.
func FromTags(path string, info tags.Info) model.Track {
	title := info.Title
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	return model.Track{Path: path, Title: title, Artist: info.Artist, Album: info.Album, Length: info.Length, Kind: "audio",
		AlbumArtist: info.AlbumArtist, Genre: info.Genre, Year: info.Year, Number: info.Number, Disc: info.Disc}
}

// playlist adds a playlist's tracks: its files the player plays, with
// what their tags say, else what the playlist says; a CUE sheet's tracks
// as the sheet says, parts of their file. Files missing, those the
// player does not play and streams are left out, and said.
func (thisRef *Expanded) playlist(ctx context.Context, path string) error {
	read, err := playlist.Open(path)
	if err != nil {
		return fmt.Errorf("open playlist: %w", err)
	}
	if read.Skipped > 0 {
		thisRef.streams += read.Skipped
	}
	lengths := map[string]time.Duration{} // a CUE sheet's files', read once
	for _, entry := range read.Entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if format, unsupported := playback.UnsupportedFormat(entry.Path); unsupported {
			if thisRef.unsupported == nil {
				thisRef.unsupported = map[string]int{}
			}
			thisRef.unsupported[format]++
			continue
		}
		info, err := os.Stat(entry.Path)
		if err != nil || !info.Mode().IsRegular() {
			thisRef.skip(fmt.Errorf("%s: %s is not there", filepath.Base(path), entry.Path))
			continue
		}
		cue := entry.Start > 0 || entry.End > 0 || strings.EqualFold(filepath.Ext(path), ".cue")
		if !cue {
			track, err := Track(entry.Path)
			if err != nil {
				thisRef.skip(err)
				continue
			}
			if track.Title == strings.TrimSuffix(filepath.Base(entry.Path), filepath.Ext(entry.Path)) && entry.Title != "" {
				track.Title, track.Artist = entry.Title, cmp.Or(track.Artist, entry.Artist)
			}
			if track.Length == 0 {
				track.Length = entry.Length
			}
			thisRef.Tracks = append(thisRef.Tracks, track)
			continue
		}
		length := entry.Length
		if entry.End == 0 {
			whole, known := lengths[entry.Path]
			if !known {
				if info, ok := tags.Read(entry.Path); ok {
					whole = info.Length
				}
				lengths[entry.Path] = whole
			}
			length = max(whole-entry.Start, 0)
		}
		title := cmp.Or(entry.Title, fmt.Sprintf("Track %02d", entry.Number))
		thisRef.Tracks = append(thisRef.Tracks, model.Track{Path: entry.Path, Title: title, Artist: entry.Artist, Album: entry.Album,
			Number: entry.Number, Length: length, Start: entry.Start, End: entry.End, Kind: "audio"})
	}
	return nil
}

// Track is a file as a track of the queue: what its tags say (every
// container's), else its name.
func Track(path string) (model.Track, error) {
	info, err := os.Stat(path)
	if err != nil {
		return model.Track{}, fmt.Errorf("open music file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return model.Track{}, fmt.Errorf("%s is not a regular music file", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return model.Track{}, fmt.Errorf("read music file: %w", err)
	}
	if err := file.Close(); err != nil {
		return model.Track{}, fmt.Errorf("close music file: %w", err)
	}
	track := model.Track{Path: path, Title: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), Kind: "audio"}
	if info, ok := tags.Read(path); ok {
		track = FromTags(path, info)
	}
	return track, nil
}
