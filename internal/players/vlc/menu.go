package vlc

import (
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/style"
)

// The stock bar painter currently bypasses the skin's "bar" binding.
// Supply its authored background while keeping stock titles and menus.
type menuLook struct{ style.LookAndFeel }

func (l *menuLook) DrawMenuBar(ctx *paintengine2d.Context, r paintengine2d.Rect) {
	if !style.DrawSkinSprite(l.LookAndFeel, ctx, r, "control.bar", paintengine2d.Color{}) {
		l.LookAndFeel.DrawMenuBar(ctx, r)
	}
}
