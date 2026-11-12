package library

import (
	"strconv"
	"strings"

	"github.com/codemodify/media-player-music/musicapi"
	search "goframework.io/data-search"
)

// fields are the operators a search reads: artist:, album:, genre:, title:
// and year:.
var fields = search.Fields{"artist": search.Text, "album": search.Text, "genre": search.Text, "title": search.Text, "year": search.Text}

// query is what was typed, folded: each term and each operator's value
// without case or accents.
type query struct {
	terms                       []string
	artist, album, genre, title []string
	years                       []int
}

// parse reads what was typed with goframework's data-search: words,
// "phrases" and the fields' operators; nothing typed is a language, so a
// search never fails on what a person types (but a year that is no
// number).
func parse(text string) (query, error) {
	parsed, err := search.Parse(text, fields)
	if err != nil {
		return query{}, err
	}
	var q query
	for _, term := range parsed.Terms {
		if folded := fold(term.Text); folded != "" {
			q.terms = append(q.terms, folded)
		}
	}
	for _, operator := range parsed.Operators {
		value := fold(operator.Value)
		switch operator.Name {
		case "artist":
			q.artist = append(q.artist, value)
		case "album":
			q.album = append(q.album, value)
		case "genre":
			q.genre = append(q.genre, value)
		case "title":
			q.title = append(q.title, value)
		case "year":
			year, err := strconv.Atoi(operator.Value)
			if err != nil {
				return query{}, &yearError{operator.Value}
			}
			q.years = append(q.years, year)
		}
	}
	return q, nil
}

type yearError struct{ value string }

func (thisRef *yearError) Error() string { return "year:" + thisRef.value + " is not a year" }

func (thisRef query) empty() bool {
	return len(thisRef.terms) == 0 && len(thisRef.artist) == 0 && len(thisRef.album) == 0 && len(thisRef.genre) == 0 &&
		len(thisRef.title) == 0 && len(thisRef.years) == 0
}

// track reports whether a track holds every term in its text, and every
// operator's value in its field.
func (thisRef query) track(t musicapi.Track) bool {
	all := func(values []string, in string) bool {
		if len(values) == 0 {
			return true
		}
		folded := fold(in)
		for _, value := range values {
			if !strings.Contains(folded, value) {
				return false
			}
		}
		return true
	}
	if !all(thisRef.artist, t.Artist+"\n"+t.AlbumArtist) || !all(thisRef.album, t.Album) || !all(thisRef.genre, t.Genre) || !all(thisRef.title, t.Title) {
		return false
	}
	for _, year := range thisRef.years {
		if t.Year != year {
			return false
		}
	}
	return all(thisRef.terms, strings.Join([]string{t.Title, t.Artist, t.Album, t.AlbumArtist, t.Genre}, "\n"))
}

// name reports whether a name holds every term: an artist's or an
// album's own, to list it beside its tracks.
func (thisRef query) name(name string) bool {
	if len(thisRef.terms) == 0 {
		return false
	}
	folded := fold(name)
	for _, term := range thisRef.terms {
		if !strings.Contains(folded, term) {
			return false
		}
	}
	return true
}

// fold is text without case, and without the accents of the Latin
// letters that carry them ("Beyoncé" as "beyonce", "Sigur Rós" as "sigur
// ros"), so a search finds a name typed without them.
func fold(text string) string {
	return accents.Replace(strings.ToLower(text))
}

var accents = func() *strings.Replacer {
	plain := map[string]string{
		"a": "àáâãäåāăą", "c": "çćĉċč", "d": "ďđ", "e": "èéêëēĕėęě", "g": "ĝğġģ", "h": "ĥħ", "i": "ìíîïĩīĭįı",
		"j": "ĵ", "k": "ķ", "l": "ĺļľŀł", "n": "ñńņňŉ", "o": "òóôõöøōŏő", "r": "ŕŗř", "s": "śŝşšș", "t": "ţťŧț",
		"u": "ùúûüũūŭůűų", "w": "ŵ", "y": "ýÿŷ", "z": "źżž", "ae": "æ", "oe": "œ", "ss": "ß", "th": "þ",
	}
	var pairs []string
	for to, from := range plain {
		for _, r := range from {
			pairs = append(pairs, string(r), to)
		}
	}
	return strings.NewReplacer(pairs...)
}()
