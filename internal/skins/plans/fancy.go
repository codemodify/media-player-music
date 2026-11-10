package plans

import (
	fl "github.com/codemodify/media-player-music/internal/skins/fancy"
	"goframework.io/graphics-paint2d"
)

// Fancy interprets the supplied Player Interface reference as an original
// application-owned design. All chrome is drawn with public paths/gradients
// independently at 1x and 2x; no screenshot or third-party skin is embedded.
func Fancy() *Plan {
	sh := &Sheet{Name: "fancy-chrome"}
	l := &lay{sh: sh, gap: 3}
	for _, face := range []struct {
		name  string
		w, h  int
		slice [4]int
		draw  func(*paint2d.Context, float32, float32)
	}{
		{"main", fl.MainW, fl.MainH, [4]int{}, fancyMainFace},
		{"equalizer", fl.EqW, fl.EqH, [4]int{}, fancyEqFace},
		{"playlist", fl.ListW, fl.ListH, [4]int{39, 100, 39, 48}, fancyListFace},
		{"compact", fl.CompactW, fl.CompactH, [4]int{19, 20, 19, 20}, fancyCompactFace},
	} {
		l.row(face.h)
		l.cell("face."+face.name, face.w, face.h, face.slice, "stretch", false, face.draw)
	}
	l.row(16)
	l.cell("app.icon", 16, 16, [4]int{}, "", false, func(ctx *paint2d.Context, w, h float32) {
		paintPlayerLogo(ctx, paint2d.XYWH(0, 0, w, h))
	})
	l.row(24)
	for _, art := range []struct {
		name string
		w, h int
		draw func(*paint2d.Context, float32, float32)
	}{
		{"playlist.load", 45, 17, fancyListButton},
		{"playlist.save", 43, 17, fancyListButton},
		{"playlist.clear", 43, 17, fancyListButton},
		{"playlist.toggle", 60, 18, fancyListToggle},
		{"playlist.logo", 24, 24, fancyListLogo},
	} {
		l.cell(art.name, art.w, art.h, [4]int{}, "", false, art.draw)
	}
	l.close()
	main := LayoutSpec{Name: fl.LayoutMain, W: fl.MainW, H: fl.MainH}
	add := func(layout *LayoutSpec, name string, at [4]int) {
		layout.Slots = append(layout.Slots, SlotSpec{Name: name, At: at})
	}
	shape := func(layout *LayoutSpec, name string) {
		layout.Slots = append(layout.Slots, SlotSpec{Name: "shape." + name, At: [4]int{}, StretchX: true, StretchY: true, Art: [][2]string{{"normal", "face." + name}}})
	}
	shape(&main, "main")
	for _, slot := range []struct {
		name string
		at   [4]int
	}{
		{"header", [4]int{43, 47, 83, 13}},
		{"analyser", [4]int{162, 31, 96, 45}},
		{"time", [4]int{157, 82, 106, 30}},
		{"track", [4]int{148, 115, 124, 14}},
		{"status", [4]int{161, 131, 98, 11}},
		{"seek", [4]int{135, 145, 150, 10}},
		{"volume", [4]int{23, 63, 40, 40}},
		{"balance", [4]int{357, 63, 40, 40}},
		{"eq", [4]int{70, 68, 44, 17}},
		{"pl", [4]int{70, 91, 44, 17}},
		{"shuffle", [4]int{306, 68, 44, 17}},
		{"repeat", [4]int{306, 91, 44, 17}},
		{"mute", [4]int{23, 109, 40, 17}},
		{"skin", [4]int{72, 113, 42, 17}},
		{"settings", [4]int{355, 109, 43, 17}},
		{"prev", [4]int{42, 165, 28, 24}},
		{"back", [4]int{83, 173, 31, 25}},
		{"stop", [4]int{122, 184, 26, 23}},
		{"play", [4]int{181, 161, 58, 58}},
		{"pause", [4]int{181, 161, 58, 58}},
		{"forward", [4]int{306, 173, 31, 25}},
		{"next", [4]int{351, 165, 28, 24}},
		{"open", [4]int{355, 144, 30, 17}},
		{"compact", [4]int{310, 145, 22, 17}},
		{"minimize", [4]int{377, 47, 16, 14}},
		{"close", [4]int{396, 47, 16, 14}},
	} {
		add(&main, slot.name, slot.at)
	}
	main.Slots = append(main.Slots, SlotSpec{Name: "logo", At: [4]int{23, 45, 16, 16}, Art: [][2]string{{"normal", "app.icon"}}})
	eq := LayoutSpec{Name: fl.LayoutEqualizer, W: fl.EqW, H: fl.EqH}
	shape(&eq, "equalizer")
	for _, slot := range []struct {
		name string
		at   [4]int
	}{
		{"header", [4]int{37, 8, 67, 15}},
		{"on", [4]int{40, 105, 26, 14}},
		{"auto", [4]int{70, 105, 32, 14}},
		{"load", [4]int{103, 7, 42, 17}},
		{"reset", [4]int{149, 7, 42, 17}},
		{"save", [4]int{195, 7, 42, 17}},
		{"presets", [4]int{241, 7, 68, 17}},
		{"close", [4]int{393, 7, 18, 17}},
		{"preamp", [4]int{56, 32, 14, 65}},
		{"labels", [4]int{52, 98, 242, 11}},
		{"status", [4]int{327, 33, 64, 66}},
	} {
		add(&eq, slot.name, slot.at)
	}
	for i := 0; i < 10; i++ {
		add(&eq, "band."+string(rune('0'+i)), [4]int{84 + 21*i, 32, 14, 65})
	}
	playlist := LayoutSpec{Name: fl.LayoutPlaylist, W: fl.ListW, H: fl.ListH}
	shape(&playlist, "playlist")
	for _, slot := range []SlotSpec{
		{Name: "header", At: [4]int{42, 8, 73, 16}},
		{Name: "open", At: [4]int{119, 7, 45, 17}, Art: [][2]string{{"normal", "playlist.load"}}},
		{Name: "save", At: [4]int{168, 7, 43, 17}, Art: [][2]string{{"normal", "playlist.save"}}},
		{Name: "clear", At: [4]int{215, 7, 43, 17}, Art: [][2]string{{"normal", "playlist.clear"}}},
		{Name: "close", At: [4]int{9, 7, 18, 17}, FromRight: true},
		{Name: "list", At: [4]int{42, 29, 93, 26}, StretchX: true, StretchY: true},
		{Name: "rows", At: [4]int{45, 32, 102, 29}, StretchX: true, StretchY: true},
		{Name: "scroll", At: [4]int{93, 32, 7, 29}, FromRight: true, StretchY: true},
		{Name: "shuffle", At: [4]int{25, 35, 60, 18}, FromRight: true, Art: [][2]string{{"normal", "playlist.toggle"}}},
		{Name: "repeat", At: [4]int{25, 63, 60, 18}, FromRight: true, Art: [][2]string{{"normal", "playlist.toggle"}}},
		{Name: "logo", At: [4]int{48, 85, 24, 24}, FromRight: true, Art: [][2]string{{"normal", "playlist.logo"}}},
		{Name: "add", At: [4]int{44, 5, 28, 17}, FromBottom: true},
		{Name: "remove", At: [4]int{77, 5, 42, 17}, FromBottom: true},
		{Name: "status", At: [4]int{126, 7, 114, 14}, FromBottom: true, StretchX: true},
		{Name: "resize", At: [4]int{16, 16, 13, 13}, FromRight: true, FromBottom: true},
	} {
		playlist.Slots = append(playlist.Slots, slot)
	}
	compact := LayoutSpec{Name: fl.LayoutCompact, W: fl.CompactW, H: fl.CompactH}
	shape(&compact, "compact")
	for _, slot := range []struct {
		name string
		at   [4]int
	}{
		{"track", [4]int{23, 7, 207, 14}}, {"time", [4]int{235, 7, 42, 14}},
		{"seek", [4]int{23, 25, 255, 5}}, {"prev", [4]int{280, 11, 18, 18}},
		{"stop", [4]int{299, 11, 18, 18}}, {"play", [4]int{318, 11, 18, 18}},
		{"pause", [4]int{318, 11, 18, 18}}, {"next", [4]int{337, 11, 18, 18}},
		{"restore", [4]int{357, 8, 21, 20}}, {"skin", [4]int{380, 8, 18, 20}}, {"close", [4]int{401, 8, 15, 20}},
	} {
		add(&compact, slot.name, slot.at)
	}
	return &Plan{Name: "fancy", Label: "Fancy", Year: 2026, Family: "fancy", Base: "dark",
		Lineage: "Application artwork inspired by the supplied Player Interface reference by Sinator Designer",
		Summary: "Graphite sculpted wings, a round cyan spectrum display, separate equalizer and playlist, and a compact player strip.",
		Sheets:  []*Sheet{sh}, Layouts: []LayoutSpec{main, eq, playlist, compact}, Window: &WindowSpec{Layout: ":"},
		Colors:  map[string]string{"window": "#11151b", "panel": "#141920", "control": "#242b34", "text": "#dbe7ed", "textMuted": "#8a99a6", "selection": "#183840", "selectionText": "#85e6ee", "focus": "#68d6e5"},
		Metrics: map[string]float32{"menuItemH": 22},
		Text:    []TextRole{{Name: "player.text", Color: "#83dee7", Size: 10}, {Name: "player.row", Color: "#abbcc6", Size: 10}, {Name: "player.current", Color: "#83e5ed"}, {Name: "player.selected", Color: "#142b33"}, {Name: "player.selectedText", Color: "#98eaf2"}, {Name: "player.info", Color: "#8493a0", Size: 9}},
	}
}
