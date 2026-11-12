package playlist

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestM3UWithExtinfRelativePathsAndURLs(t *testing.T) {
	folder := filepath.FromSlash("/music/lists")
	if runtime.GOOS == "windows" {
		folder = `C:\music\lists`
	}
	read := M3U("\ufeff#EXTM3U\r\n#EXTINF:227,The Tessellators - Harbour Lights\r\n../Low Tide/01.mp3\r\n\r\n# a comment\r\n"+
		"#EXTINF:-1 tvg-id=\"x\",Radio\r\nhttp://radio.example/stream\r\nfile:///music/Field%20Notes/02.flac\r\n#EXTINF:12.5,Untitled\r\nsub/c.ogg\r\n", folder)
	if read.Skipped != 1 || len(read.Entries) != 3 {
		t.Fatalf("%+v", read)
	}
	first := read.Entries[0]
	if first.Path != filepath.Join(folder, "..", "Low Tide", "01.mp3") || first.Artist != "The Tessellators" || first.Title != "Harbour Lights" || first.Length != 227*time.Second {
		t.Fatalf("first: %+v", first)
	}
	if second := read.Entries[1]; second.Path != filepath.FromSlash("/music/Field Notes/02.flac") || second.Title != "" {
		t.Fatalf("a file URL: %+v (the #EXTINF of the stream is not its)", second)
	}
	if third := read.Entries[2]; third.Path != filepath.Join(folder, "sub", "c.ogg") || third.Title != "Untitled" || third.Length != 12500*time.Millisecond {
		t.Fatalf("third: %+v", third)
	}
}

func TestPLSInItsNumbersOrder(t *testing.T) {
	read := PLS("[playlist]\nFile2=b.mp3\nTitle2=Second\nFile1=/abs/a.mp3\nTitle1=First\nLength1=61\nFile3=https://example/stream\nNumberOfEntries=3\nVersion=2\n", filepath.FromSlash("/lists"))
	if len(read.Entries) != 2 || read.Skipped != 1 || read.Entries[0].Title != "First" || read.Entries[0].Length != 61*time.Second || read.Entries[1].Path != filepath.Join(filepath.FromSlash("/lists"), "b.mp3") {
		t.Fatalf("%+v", read)
	}
}

func TestCUETracksWithinAFile(t *testing.T) {
	sheet := `REM GENRE Folk
PERFORMER "Ruth Alder"
TITLE "Field Notes"
FILE "Field Notes.flac" WAVE
  TRACK 01 AUDIO
    TITLE "Paper Anemometer"
    INDEX 00 00:00:00
    INDEX 01 00:00:33
  TRACK 02 AUDIO
    TITLE "Beacon"
    PERFORMER "Ruth Alder & Ostro"
    INDEX 01 04:12:00
  TRACK 03 DATA
    INDEX 01 08:00:00
FILE "bonus.wav" WAVE
  TRACK 04 AUDIO
    TITLE "Bonus"
    INDEX 01 00:00:00
`
	read := CUE(sheet, filepath.FromSlash("/discs"))
	if len(read.Entries) != 3 {
		t.Fatalf("%+v", read)
	}
	first, second, bonus := read.Entries[0], read.Entries[1], read.Entries[2]
	if first.Path != filepath.Join(filepath.FromSlash("/discs"), "Field Notes.flac") || first.Title != "Paper Anemometer" || first.Artist != "Ruth Alder" || first.Album != "Field Notes" ||
		first.Number != 1 || first.Start != 440*time.Millisecond || first.End != 4*time.Minute+12*time.Second || first.Length != first.End-first.Start {
		t.Fatalf("first: %+v", first)
	}
	// the data track is no end: the second plays to the file's end
	if second.Artist != "Ruth Alder & Ostro" || second.Start != 4*time.Minute+12*time.Second || second.End != 0 {
		t.Fatalf("second: %+v", second)
	}
	if bonus.Path != filepath.Join(filepath.FromSlash("/discs"), "bonus.wav") || bonus.Start != 0 || bonus.End != 0 || bonus.Number != 4 {
		t.Fatalf("bonus: %+v", bonus)
	}
}

func TestLegacyTextIsWindows1252AndWriteReadsBack(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old.m3u")
	if err := os.WriteFile(old, []byte("#EXTINF:10,Beyonc\xe9 - Caf\xe9\nsong.mp3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	read, err := Open(old)
	if err != nil || len(read.Entries) != 1 || read.Entries[0].Artist != "Beyoncé" || read.Entries[0].Title != "Café" {
		t.Fatalf("%+v %v", read, err)
	}
	items := []Item{{Path: filepath.Join(dir, "a.mp3"), Title: "Café", Artist: "Beyoncé", Length: 61 * time.Second}, {Path: filepath.Join(dir, "b.mp3"), Title: "b"}}
	for _, format := range []string{FormatM3U8, FormatPLS} {
		path := filepath.Join(dir, "saved."+format)
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := Write(file, format, items); err != nil {
			t.Fatal(err)
		}
		file.Close()
		back, err := Open(path)
		if err != nil || len(back.Entries) != 2 || back.Entries[0].Path != items[0].Path || back.Entries[0].Title != "Café" || back.Entries[0].Artist != "Beyoncé" ||
			back.Entries[0].Length != 61*time.Second || back.Entries[1].Path != items[1].Path {
			t.Fatalf("%s: %+v %v", format, back, err)
		}
	}
	if err := Write(&strings.Builder{}, FormatM3U8, []Item{{Path: "a\nb.mp3"}}); err == nil {
		t.Fatal("a path with a line's end was written")
	}
	if FormatOf("x.M3U") != FormatM3U8 || FormatOf("x.pls") != FormatPLS || FormatOf("x.cue") != "" || !Is("x.CUE") || Is("x.mp3") {
		t.Fatal("FormatOf, Is")
	}
}

func FuzzPlaylists(f *testing.F) {
	f.Add("#EXTM3U\n#EXTINF:1,a - b\nc.mp3\n")
	f.Add("[playlist]\nFile1=a\nTitle1=b\nLength1=3\n")
	f.Add("FILE \"a.flac\" WAVE\nTRACK 01 AUDIO\nINDEX 01 00:00:00\nTRACK 02 AUDIO\nINDEX 01 01:02:03\n")
	f.Fuzz(func(t *testing.T, text string) {
		for _, read := range []Read{M3U(text, "/x"), PLS(text, "/x"), CUE(text, "/x")} {
			for _, entry := range read.Entries {
				if entry.Path == "" || entry.Start < 0 || entry.End != 0 && entry.End <= entry.Start {
					t.Fatalf("%+v", entry)
				}
			}
		}
	})
}
