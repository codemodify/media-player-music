package plans

import (
	"strings"

	"goframework.io/desktop-ui-skingen"
	"goframework.io/graphics-paint2d"
)

// Utility is a plain, cool-grey utility window: a slate caption band, flat
// square controls with a one-pixel rule, and a toolbar of small two-tone
// icons drawn on the pixel grid (slate marks, a teal play and an orange
// stop). The face draws the menus, the equaliser and the playlist on it.
func Utility() *Plan {
	p := utilityPalette()
	text := map[string]skingen.TextRole{
		"control":             {Color: "#1C252E", Disabled: "#8A9199"},
		"menu":                {Color: "#1C252E", Hover: "#FFFFFF", Pressed: "#FFFFFF", Checked: "#FFFFFF", Disabled: "#8A9199"},
		"player.row":          {Color: "#1C252E"},
		"player.selected":     {Color: "#2F5D8A"},
		"player.selectedText": {Color: "#FFFFFF"},
		"player.text":         {Color: "#1C252E"},
	}
	colors := map[string]string{"control": "#D5D8DB", "focus": "#2F5D8A", "panel": "#D5D8DB", "selection": "#2F5D8A", "selectionText": "#FFFFFF", "text": "#1C252E", "textMuted": "#59626B", "window": "#D5D8DB"}
	return genericSkin("utility", "Utility", "A plain cool-grey utility window: a slate caption, flat square controls and a toolbar of small two-tone icons, with the equaliser and the playlist below.", &geomUtility, p, func(u use) drawFunc { return utilityArt(p, u) }, text, colors)
}

func utilityPalette() *palette {
	return &palette{body: rgb("#d5d8db"), bodyTop: rgb("#e1e3e5"), bodyBottom: rgb("#c9ccd0"), edge: rgb("#6f767d"), rim: rgb("#f4f5f6"),
		well: rgb("#ffffff"), wellDark: rgb("#7d8186"), wellLight: rgb("#ffffff"),
		key: rgb("#d7d9db"), keyTop: rgb("#e9eaeb"), keyBottom: rgb("#c4c7ca"), keyEdge: rgb("#80858a"),
		ink: rgb("#2b3a4a"), inkHover: rgb("#2f5d8a"), accent: rgb("#e0742a"), lcd: rgb("#1b1d20"), lcdDim: rgb("#9a9ea3"),
		radius: 0, keyRadius: 2, gloss: 0, round: false}
}

// The toolbar icons' colours: every icon is drawn in whole, opaque pixels
// of these and nothing else, so it stays crisp at any scale.
var (
	utInk   = rgb("#2b3a4a") // a mark
	utLight = rgb("#7b93ab") // its lit edge
	utTeal  = rgb("#18907c")
	utTealL = rgb("#5cc8b2")
	utRed   = rgb("#d0552a")
	utRedL  = rgb("#f29a62")
	utPaper = rgb("#f4f6f8")
)

// utilityIcons are the 16 × 16 icons, one character a pixel:
// '#' ink, '+' its light, 't'/'T' teal and light teal, 'r'/'R' orange and
// light orange, 'w' paper.
var utilityIcons = map[string][]string{
	"play": {
		"................",
		"................",
		"....T...........",
		"....tT..........",
		"....ttT.........",
		"....tttT........",
		"....ttttT.......",
		"....tttttT......",
		"....tttttt......",
		"....ttttt.......",
		"....tttt........",
		"....ttt.........",
		"....tt..........",
		"....t...........",
		"................",
		"................",
	},
	"pause": {
		"................",
		"................",
		"................",
		"....++#..++#....",
		"....++#..++#....",
		"....++#..++#....",
		"....++#..++#....",
		"....++#..++#....",
		"....++#..++#....",
		"....++#..++#....",
		"....++#..++#....",
		"....++#..++#....",
		"....###..###....",
		"................",
		"................",
		"................",
	},
	"stop": {
		"................",
		"................",
		"................",
		"................",
		"....RRRRRRRr....",
		"....RrrrrrRr....",
		"....Rrrrrrrr....",
		"....Rrrrrrrr....",
		"....Rrrrrrrr....",
		"....Rrrrrrrr....",
		"....Rrrrrrrr....",
		"....rrrrrrrr....",
		"................",
		"................",
		"................",
		"................",
	},
	"prev": {
		"................",
		"................",
		"................",
		"...##.....+..+..",
		"...##....+#.+#..",
		"...##...+##+##..",
		"...##..+######..",
		"...##.+#######..",
		"...##.########..",
		"...##..#######..",
		"...##...###.##..",
		"...##....##..#..",
		"...##.....#.....",
		"................",
		"................",
		"................",
	},
	"next": {
		"................",
		"................",
		"................",
		"..+..+.....##...",
		"..#+.#+....##...",
		"..##+##+...##...",
		"..######+..##...",
		"..#######+.##...",
		"..########.##...",
		"..#######..##...",
		"..##.###...##...",
		"..#..##....##...",
		".....#.....##...",
		"................",
		"................",
		"................",
	},
	"eject": {
		"................",
		"................",
		"................",
		".......+........",
		"......+#+.......",
		".....+###+......",
		"....+#####+.....",
		"...+#######+....",
		"...#########....",
		"................",
		"...+++++++++....",
		"...#########....",
		"...#########....",
		"................",
		"................",
		"................",
	},
	"playlist": {
		"................",
		"................",
		"..++.+++++++++..",
		"..##.#########..",
		"................",
		"..++.+++++++++..",
		"..##.#########..",
		"................",
		"..tT.+++++++++..",
		"..tt.#########..",
		"................",
		"..++.+++++++++..",
		"..##.#########..",
		"................",
		"................",
		"................",
	},
	"playlist_small": {
		"................",
		"................",
		"................",
		"................",
		"...++.++++++....",
		"...##.######....",
		"................",
		"...tT.++++++....",
		"...tt.######....",
		"................",
		"...++.++++++....",
		"...##.######....",
		"................",
		"................",
		"................",
		"................",
	},
	"speaker": {
		"................",
		"................",
		"................",
		"......+#....+...",
		".....+##..+..#..",
		"..++++##...#..#.",
		"..######...#..#.",
		"..######...#..#.",
		"..######...#..#.",
		"..######...#..#.",
		"..##++##...#..#.",
		".....+##..+..#..",
		"......+#....+...",
		"................",
		"................",
		"................",
	},
	"speaker_mute": {
		"................",
		"................",
		"................",
		"......+#........",
		".....+##........",
		"..++++##..r...r.",
		"..######...r.r..",
		"..######....r...",
		"..######...r.r..",
		"..######..r...r.",
		"..##++##........",
		".....+##........",
		"......+#........",
		"................",
		"................",
		"................",
	},
	"shuffle": {
		"................",
		"................",
		"................",
		"...........#....",
		"..##+....+####..",
		"....#+..+#.#....",
		".....#..#.......",
		"......##........",
		"......##........",
		".....#..#.......",
		"....#+..+#.#....",
		"..##+....+####..",
		"...........#....",
		"................",
		"................",
		"................",
	},
	"trash": {
		"................",
		"................",
		"......####......",
		"...##########...",
		"...++++++++++...",
		"....#+#+#+#+....",
		"....#+#+#+#+....",
		"....#+#+#+#+....",
		"....#+#+#+#+....",
		"....#+#+#+#+....",
		"....#+#+#+#+....",
		"....#+#+#+#+....",
		"....########....",
		"................",
		"................",
		"................",
	},
	"type_unknown": {
		"................",
		"...#######......",
		"...#wwwww+#.....",
		"...#wwwww+w#....",
		"...#wwwww####...",
		"...#wwww+wwww#..",
		"...#www#+#www#..",
		"...#wwwww#www#..",
		"...#wwww#wwww#..",
		"...#wwww#wwww#..",
		"...#wwwwwwwww#..",
		"...#wwww#wwww#..",
		"...#wwwwwwwww#..",
		"...###########..",
		"................",
		"................",
	},
}

func utilityIcon(rows []string) drawFunc {
	ink := map[rune]paint2d.Color{'#': utInk, '+': utLight, 't': utTeal, 'T': utTealL, 'r': utRed, 'R': utRedL, 'w': utPaper}
	return func(ctx *paint2d.Context, w, h float32) {
		for y, line := range rows {
			for x, c := range line {
				if col, ok := ink[c]; ok {
					px(ctx, float32(x), float32(y), 1, 1, col)
				}
			}
		}
	}
}

func utilityArt(p *palette, u use) func(*paint2d.Context, float32, float32) {
	name := u.name
	if icon, ok := strings.CutPrefix(name, "icon."); ok {
		if rows, ok := utilityIcons[icon]; ok {
			return utilityIcon(rows)
		}
	}
	slate, slateTop := rgb("#33475c"), rgb("#4d6680")
	rule, light := rgb("#7a8189"), rgb("#f7f8f9")
	switch name {
	case "app.icon":
		return func(ctx *paint2d.Context, w, h float32) {
			emblem(ctx, paint2d.XYWH(0, 0, w, h), rgb("#e0742a"), rgb("#f4f6f8"))
		}
	case "face.main":
		return func(ctx *paint2d.Context, w, h float32) {
			all := paint2d.XYWH(0, 0, w, h)
			fill(ctx, all, rgb("#3a4148"))
			fill(ctx, all.Inset(1), p.body)
			fill(ctx, paint2d.XYWH(1, 1, w-2, 1), light)
			fill(ctx, paint2d.XYWH(1, 1, 1, h-2), light)
			fill(ctx, paint2d.XYWH(w-2, 1, 1, h-2), rgb("#aab0b6"))
			fill(ctx, paint2d.XYWH(1, h-2, w-2, 1), rgb("#aab0b6"))
			// the caption band, in the top slice
			band := paint2d.XYWH(3, 3, w-6, 20)
			ctx.DrawRect(band, vpaint(band, stop(0, slateTop), stop(1, slate)))
			fill(ctx, paint2d.XYWH(3, 3, w-6, 1), rgb("#6f8aa6"))
			fill(ctx, paint2d.XYWH(3, 22, w-6, 1), rgb("#22313f"))
		}
	case "control.up", "control.down", "control.disabled", "control.scroll", "control.groove":
		return func(ctx *paint2d.Context, w, h float32) {
			r := paint2d.XYWH(0, 0, w, h)
			switch name {
			case "control.up":
				ctx.DrawRect(r, vpaint(r, stop(0, rgb("#f1f2f3")), stop(1, rgb("#d3d6d9"))))
				ring(ctx, r, 0, 1, rule)
			case "control.down":
				fill(ctx, r, rgb("#bfc5cb"))
				ring(ctx, r, 0, 1, rgb("#4f5861"))
				fill(ctx, paint2d.XYWH(1, 1, w-2, 1), rgb("#a3aab1"))
			case "control.disabled":
				fill(ctx, r, rgb("#dcdee0"))
				ring(ctx, r, 0, 1, rgb("#b0b5ba"))
			case "control.scroll":
				fill(ctx, r, rgb("#e6e8ea"))
				ring(ctx, r, 0, 1, rgb("#b0b5ba"))
			default: // the groove
				fill(ctx, r, rgb("#9aa1a8"))
				fill(ctx, r.Inset(1), rgb("#eef0f2"))
				fill(ctx, paint2d.XYWH(1, 1, w-2, 1), rgb("#c2c7cc"))
			}
		}
	case "control.bar", "control.menu":
		return func(ctx *paint2d.Context, w, h float32) {
			fill(ctx, paint2d.XYWH(0, 0, w, h), p.body)
			if name == "control.bar" {
				fill(ctx, paint2d.XYWH(0, h-1, w, 1), rgb("#aab0b6"))
			}
		}
	case "control.menu.hot":
		return func(ctx *paint2d.Context, w, h float32) {
			fill(ctx, paint2d.XYWH(0, 0, w, h), rgb("#2f5d8a"))
		}
	case "control.check", "control.check.on", "control.check.disabled":
		return func(ctx *paint2d.Context, w, h float32) {
			r := paint2d.XYWH(0, 0, w, h)
			back := rgb("#ffffff")
			if name == "control.check.disabled" {
				back = rgb("#e3e5e7")
			}
			fill(ctx, r, back)
			ring(ctx, r, 0, 1, rgb("#59626b"))
			if name == "control.check.on" {
				fill(ctx, r.Inset(3), rgb("#2f5d8a"))
			}
		}
	}
	return nil
}
