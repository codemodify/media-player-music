package players

// The window systems and theme engines the player ships. Every face, the
// shell and main import this package, so their tests link the same set.
// The engines are imported here, not only in main, because Themed looks up
// breeze-night while this package is initialised.
//
// The engines are the ones the player draws with: Breeze for the default
// interface (Themed, breeze-night), Windows 95's and Luna's for the skins
// built over win95 and luna (internal/skins/plans). A look.json written by
// another desktop-ui program can name any pack; one whose engine is not
// here is shown as the nearest pack the player has, and desktop-ui says so
// in its log (style.NearestThemePack).
import (
	_ "goframework.io/desktop-ui-platform-all"
	_ "goframework.io/desktop-ui-themes-breeze"
	_ "goframework.io/desktop-ui-themes-luna"
	_ "goframework.io/desktop-ui-themes-win95"
)
