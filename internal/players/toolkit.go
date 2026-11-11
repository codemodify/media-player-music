package players

// The window systems and theme engines the player ships: every era engine,
// as the shipping build's theme_engine_all tag chose before. Every face, the
// shell and main import this package, so their tests link the same set.
// The engines are imported here, not only in main, because Themed looks up
// breeze-night while this package is initialised.
//
// R28: the shared look.json can name any pack, so the player links every
// engine (desktop-ui-themes-all, about 5.5 MB) though its faces draw their
// own skins; themes on demand (a desktop set and a nearest-linked
// fallback) would let it import fewer.
import (
	_ "goframework.io/desktop-ui-platform-all"
	_ "goframework.io/desktop-ui-themes-all"
)
