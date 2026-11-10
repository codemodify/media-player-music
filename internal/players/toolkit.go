package players

// The window systems and theme engines the player ships: every era engine,
// as the shipping build's theme_engine_all tag chose before. Every face, the
// shell and main import this package, so their tests link the same set.
// The engines are imported here, not only in main, because Themed looks up
// breeze-night while this package is initialised.
import (
	_ "goframework.io/desktop-ui-platform-all"
	_ "goframework.io/desktop-ui-themes-all"
)
