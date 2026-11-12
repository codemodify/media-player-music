// Code generated once from the old amarok pack's skin.json: its sizes, slices,
// slots, frames and text roles (numbers and names; no pixels). Maintained
// by hand from here on.

package plans

import (
	"goframework.io/desktop-ui-skingen"
)

var geomSidebar = geometry{
	family: "sidebar", base: "win95",
	sheets: []sheetSpec{
		{"art", false},
	},
	sprites: []sprite{
		{"app.icon", 0, 16, 16, [4]int{0, 0, 0, 0}, "", false},
		{"app.logo", 0, 48, 48, [4]int{0, 0, 0, 0}, "", false},
		{"chrome.title", 0, 12, 26, [4]int{0, 3, 0, 3}, "", false},
		{"cover.empty", 0, 149, 135, [4]int{0, 0, 0, 0}, "", false},
		{"icon.add", 0, 22, 22, [4]int{0, 0, 0, 0}, "", false},
		{"icon.context", 0, 22, 22, [4]int{0, 0, 0, 0}, "", false},
		{"icon.eq", 0, 22, 22, [4]int{0, 0, 0, 0}, "", false},
		{"icon.next", 0, 22, 22, [4]int{0, 0, 0, 0}, "", false},
		{"icon.open", 0, 22, 22, [4]int{0, 0, 0, 0}, "", false},
		{"icon.pause", 0, 22, 22, [4]int{0, 0, 0, 0}, "", false},
		{"icon.play", 0, 22, 22, [4]int{0, 0, 0, 0}, "", false},
		{"icon.playlist", 0, 22, 22, [4]int{0, 0, 0, 0}, "", false},
		{"icon.prev", 0, 22, 22, [4]int{0, 0, 0, 0}, "", false},
		{"icon.remove", 0, 22, 22, [4]int{0, 0, 0, 0}, "", false},
		{"icon.repeat", 0, 22, 22, [4]int{0, 0, 0, 0}, "", false},
		{"icon.repeat.off", 0, 22, 22, [4]int{0, 0, 0, 0}, "", false},
		{"icon.repeat.one", 0, 22, 22, [4]int{0, 0, 0, 0}, "", false},
		{"icon.settings", 0, 22, 22, [4]int{0, 0, 0, 0}, "", false},
		{"icon.shuffle", 0, 22, 22, [4]int{0, 0, 0, 0}, "", false},
		{"icon.stop", 0, 22, 22, [4]int{0, 0, 0, 0}, "", false},
	},
	layouts: []skingen.LayoutSpec{
		{Name: "sidebar.main", W: 920, H: 620, Art: "", Slots: []skingen.SlotSpec{
			{Name: "add", At: [4]int{40, 32, 28, 28}},
			{Name: "analyser", At: [4]int{205, 64, 190, 28}, StretchX: true, FromBottom: true},
			{Name: "appearance", At: [4]int{136, 32, 28, 28}},
			{Name: "caption", At: [4]int{1, 1, 1, 25}, Art: [][2]string{{"normal", "chrome.title"}}, StretchX: true},
			{Name: "close", At: [4]int{5, 4, 22, 18}, FromRight: true},
			{Name: "context", At: [4]int{38, 68, 218, 102}, StretchY: true},
			{Name: "eq", At: [4]int{104, 32, 28, 28}},
			{Name: "equalizer", At: [4]int{266, 102, 8, 190}, StretchX: true, FromBottom: true},
			{Name: "minimize", At: [4]int{31, 4, 22, 18}, FromRight: true},
			{Name: "next", At: [4]int{100, 64, 26, 26}, Art: [][2]string{{"normal", "icon.next"}}, FromBottom: true},
			{Name: "open", At: [4]int{8, 32, 28, 28}},
			{Name: "play", At: [4]int{40, 64, 26, 26}, Art: [][2]string{{"normal", "icon.play"}}, FromBottom: true},
			{Name: "playlist", At: [4]int{200, 32, 28, 28}},
			{Name: "prev", At: [4]int{10, 64, 26, 26}, Art: [][2]string{{"normal", "icon.prev"}}, FromBottom: true},
			{Name: "queue", At: [4]int{266, 68, 8, 102}, StretchX: true, StretchY: true},
			{Name: "rail", At: [4]int{6, 68, 27, 102}, StretchY: true},
			{Name: "remove", At: [4]int{72, 32, 28, 28}},
			{Name: "repeat", At: [4]int{169, 64, 26, 26}, FromBottom: true},
			{Name: "resize", At: [4]int{1, 1, 17, 17}, FromRight: true, FromBottom: true},
			{Name: "search", At: [4]int{306, 32, 8, 28}, StretchX: true},
			{Name: "seek", At: [4]int{47, 31, 118, 18}, StretchX: true, FromBottom: true},
			{Name: "settings", At: [4]int{168, 32, 28, 28}},
			{Name: "shuffle", At: [4]int{139, 64, 26, 26}, FromBottom: true},
			{Name: "status", At: [4]int{6, 2, 20, 22}, StretchX: true, FromBottom: true},
			{Name: "stop", At: [4]int{70, 64, 26, 26}, Art: [][2]string{{"normal", "icon.stop"}}, FromBottom: true},
			{Name: "title", At: [4]int{28, 3, 76, 20}, StretchX: true},
			{Name: "volume", At: [4]int{12, 62, 155, 26}, FromRight: true, FromBottom: true},
		}},
	},
	window: &skingen.WindowSpec{Layout: ":"},
	text: []skingen.TextRole{
		{Name: "player.current", Color: "#2A2A75"},
		{Name: "player.info", Color: "#303039", Size: 11},
		{Name: "player.row", Color: "#151515", Size: 11},
		{Name: "player.selected", Color: "#6869AA"},
		{Name: "player.selectedText", Color: "#FFFFFF"},
		{Name: "player.text", Color: "#181818", Size: 11},
	},
	colors:  map[string]string{"focus": "#5F5DAD"},
	metrics: map[string]float32{"menuItemH": 20},
}
