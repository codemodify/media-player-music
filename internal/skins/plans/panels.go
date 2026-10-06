package plans

import (
	"strings"

	"github.com/codemodify/media-player-music/internal/skins/panel"
)

// The layouts the Winamp panel skins state, including the playlist's
// responsive anchors for resizing.
//
// The rects are panel's (internal/skins/panel), the same numbers
// the faces were drawn round, so the slots the manifest states and the holes
// in the art are one set of numbers. The player reads nothing but the
// manifest: it binds its controls to the slot names and asks the skin where
// each one is.

// winampLayouts are a panel face's three layouts, with every key's art bound
// from the sprites the sheet actually has.
func winampLayouts(f *panel.Face, sh *Sheet, playlistHeight ...int) []LayoutSpec {
	have := map[string]bool{}
	for _, c := range sh.Cells {
		have[c.Name] = true
	}
	var out []LayoutSpec
	for _, lay := range f.Layouts() {
		ls := LayoutSpec{Name: lay.Name, W: lay.W, H: lay.H, Art: lay.Face}
		if lay.Name == panel.LayoutPlaylist && len(playlistHeight) != 0 {
			ls.H = playlistHeight[0]
		}
		for _, sl := range lay.Slots {
			ss := SlotSpec{Name: sl.Name, At: [4]int(sl.R)}
			if lay.Name == panel.LayoutPlaylist {
				ss.StretchX, ss.StretchY, ss.FromRight, ss.FromBottom = playlistAnchors(sl.Name)
				switch {
				case ss.StretchX:
					ss.At[2] = ls.W - ss.At[0] - ss.At[2]
				case ss.FromRight:
					ss.At[0] = ls.W - ss.At[0] - ss.At[2]
				}
				switch {
				case ss.StretchY:
					ss.At[3] = ls.H - ss.At[1] - ss.At[3]
				case ss.FromBottom:
					ss.At[1] = ls.H - ss.At[1] - ss.At[3]
				}
			}
			if sl.Key != "" && have[sl.Key] {
				if strings.HasPrefix(sl.Key, "list.") {
					ss.Art = [][2]string{{"normal", sl.Key}, {"pressed", sl.Key}}
				} else {
					ss.Art = keyStates(sl.Key, have)
				}
			}
			ls.Slots = append(ls.Slots, ss)
		}
		out = append(out, ls)
	}
	return out
}

func playlistAnchors(name string) (stretchX, stretchY, right, bottom bool) {
	switch {
	case name == "list" || name == "rows" || name == "well.rows":
		return true, true, false, false
	case name == "scroll" || name == "well.scroll":
		return false, true, true, false
	case name == "row":
		return true, false, false, false
	case name == "add" || name == "rem" || name == "sel" || name == "misc":
		return false, false, false, true
	case strings.HasPrefix(name, "mini.") || name == "info" || name == "clock" || name == "opts" ||
		name == "well.info" || name == "well.clock" || name == "scroll.arrows":
		return false, false, true, true
	}
	return false, false, false, false
}

// keyStates binds a key's sprites to the control states: "key.play" at
// rest, ".hover" under the pointer, ".down" held, ".on" switched on, and
// their pairs.
//
// A key drawn with no ".on" face is not a toggle, and its switched-on state
// is its resting face, stated outright — the part fallback would otherwise
// show a pause key that is on (the player is paused) as held down, which is
// not what the panel's art means by it.
func keyStates(key string, have map[string]bool) [][2]string {
	pick := func(names ...string) string {
		for _, n := range names {
			if have[n] {
				return n
			}
		}
		return ""
	}
	var out [][2]string
	add := func(state, sprite string) {
		if sprite != "" {
			out = append(out, [2]string{state, sprite})
		}
	}
	add("normal", pick(key))
	add("hover", pick(key+".hover"))
	add("pressed", pick(key+".down"))
	if have[key+".on"] {
		add("checked", key+".on")
		add("checkedHover", pick(key+".on.hover"))
		add("checkedPressed", pick(key+".on.down", key+".down"))
	} else {
		add("checked", key)
		add("checkedHover", pick(key+".hover"))
		add("checkedPressed", pick(key+".down"))
	}
	return out
}
