# goframework.io desktop-ui: integration gaps

Review target: the goframework.io workspace at `~/go/src/goframework.io`,
commit `a5ebdc8d96f2836d833849a09d6ec8828f9bdff0` (2026-10-23), unreleased
(desktop-ui v0.0.4 is not published). The player builds against it through
the replace block `tools/app-workspace.py --go-mod` writes in go.mod, with
`GOPROXY=off`, so nothing is fetched from goframework.io. It was ported
from uitoolkit v0.23.21 / paintengine2d v0.12.2 by
`desktop-ui-porting.md`, in the order of its checklist.

This report replaces `uitoolkit-gaps.md`. Its one open item (9) is carried
over below; its 54 resolved items are summarised in
[carried over from uitoolkit](#carried-over-from-uitoolkit).

| Item | Area | Status |
| --- | --- | --- |
| [G1](#g1-desktop-ui-platform-win32-requires-a-system-native-windows-that-cannot-build-it) | module graph | Open; Windows build breaks once the replacements go |
| [G2](#g2---go-mod-then-go-mod-tidy-requires-goframework-modules-at-two-placeholder-versions) | app-workspace.py, porting guide | Open; the release steps miss half the requirements |
| [G3](#g3-the-checklist-does-not-say-go-mod-tidy-again-after-the-imports-of-step-6) | porting guide | Open; documentation |
| [G4](#g4-no-exported-constants-for-backend-scale-tray-and-xdnd_debug) | core, x11 | Open; small API gap |
| [G5](#g5-portable-logical-window-grouping-has-no-fallback-carried-from-uitoolkit-9) | rack, dock, platform | Open requirement carried from uitoolkit 9 |

## Verification at a5ebdc8

What was run, on linux/amd64, go 1.27.3, after the port:

- `go build ./...` and `go vet ./...`: pass for linux with and without cgo,
  windows, darwin without cgo; with each of the player's tags
  `font_fallback`, `font_complex`, `font_emoji` and
  `font_emoji,music_player_emoji_family_probe`. darwin with cgo was not
  built (no cross toolchain here; not a framework finding).
- All 47 probe programs under tools/probes build (each with the tags its
  header names) and its probe test files vet.
- `tools/testenv.sh go test ./...` (no display, private D-Bus and XDG
  folders): every package passes; the `font_fallback`, `font_complex` and
  `font_emoji` runs of the main, players and shell packages too.
- Pictures: the uitoolkit build of the same tree (snapshot of the working
  tree before the port, `-tags theme_engine_all,font_fallback`) and the
  desktop-ui build each wrote `-shot` pictures of all 22 appearances, at
  1× and 1.75×: **all 102 panels are pixel-identical** (51 at each scale,
  largest channel difference 0). No diag finding was printed by any run
  (no offscreen-window warning, no init-order or missing-engine warning).
- The port itself matched the guide's record of "a second port":
  `imports -qualifiers` rewrote 997 imports in 266 files (2,975 references
  requalified, 2 imports kept their old names), `requalify` requalified
  410 references in 100 files (100 imports added, 32 removed, 11 files
  given `uiskins`).

Not run: a native desktop session (X11, Wayland, tray, drag and drop), the
`-race` suite, the native opt-in tests (`MUSIC_PLAYER_NATIVE_XDND_TEST`,
`MEDIA_PLAYER_NATIVE_WINDOW_TEST`), Windows or macOS at run time.

## G1. desktop-ui-platform-win32 requires a system-native-windows that cannot build it

**High severity for the release; hidden while replacements are in place.**
`desktop-ui-platform-win32/go.mod` requires
`goframework.io/system-native-windows v0.0.3`, the last published
release, where every other goframework requirement is the unreleased
`v1.0.0` placeholder. Its code uses API that v0.0.3 does not have: the
workspace's system-native-windows is 29 files and 3,411 lines past the
v0.0.3 tag. The replace block (and a go.work) replace every version, so
the workspace build passes; nothing in the module graph says the real
minimum.

Reproduce, from the player's folder:

```
git -C ~/go/src/goframework.io/system-native-windows worktree add --detach /tmp/snw003 v0.0.3
GOPROXY=off go mod edit -replace goframework.io/system-native-windows=/tmp/snw003
GOPROXY=off GOOS=windows go build -o /dev/null .
```

```
# goframework.io/desktop-ui-platform-win32
desktop-ui-platform-win32/status_windows.go:45:17: undefined: native.HWND
desktop-ui-platform-win32/status_windows.go:46:17: undefined: native.NOTIFYICONDATA
desktop-ui-platform-win32/win32_windows.go:150:15: undefined: native.HWND
desktop-ui-platform-win32/win32_windows.go:201:22: undefined: native.WINDOWPLACEMENT
desktop-ui-platform-win32/win32_windows.go:236:19: undefined: native.HIMC
desktop-ui-platform-win32/win32_windows.go:472:20: undefined: native.POINT
desktop-ui-platform-win32/win32_windows.go:652:14: undefined: native.HBITMAP
... too many errors
```

In the player's graph the requirement arrives through
desktop-ui-platform-win32, desktop-ui-platform-all and desktop-ui-skingen
(`go mod graph | grep system-native-windows`), and `go mod tidy` writes
`goframework.io/system-native-windows v0.0.3 // indirect` in the player's
go.mod. Across the workspace 52 go.mod files require it at v0.0.3.

Expected: a requirement names a version that builds the module. Once
v0.0.4 is published, the porting guide's step 3 (`go mod tidy`) keeps
v0.0.3 by minimal version selection unless something requires more, so a
Windows build of any ported program fails. Requested: require
system-native-windows at the `v1.0.0` placeholder like the other
unreleased modules (or at v0.0.4 in the release's rewrite), and have the
release check that each goframework requirement is at the one lockstep
version, since the porting guide's step 4 (`go list -m all | grep
goframework.io`: "one version throughout") would show v0.0.3 here.

## G2. `--go-mod` then `go mod tidy` requires goframework modules at two placeholder versions

**Medium severity; release-path and documentation.** The tool's help and
the guide say that after `app-workspace.py --go-mod`, `GOPROXY=off go mod
tidy` adds the requirements "at v0.0.0-00010101000000-000000000000". It
writes two placeholders: a module some other goframework module already
requires gets that module's `v1.0.0`, and the rest get the zero
pseudo-version. The player's go.mod after the port:

```
goframework.io/desktop-ui v1.0.0
goframework.io/desktop-ui-platform-all v0.0.0-00010101000000-000000000000
goframework.io/desktop-ui-rack v0.0.0-00010101000000-000000000000
goframework.io/desktop-ui-skingen v0.0.0-00010101000000-000000000000
goframework.io/desktop-ui-skins v1.0.0
goframework.io/desktop-ui-text-complex v0.0.0-00010101000000-000000000000
goframework.io/desktop-ui-themes-all v0.0.0-00010101000000-000000000000
goframework.io/desktop-ui-widgets v1.0.0
goframework.io/graphics-paint2d v1.0.0
```

Which modules get which depends on the order things were imported:
desktop-ui-widgets was `v0.0.0-…` after the first tidy and `v1.0.0` after
the second, once the theme engines, which require it at v1.0.0, joined the
graph.

The guide's "once v0.0.4 is published" step 2 re-pins only `v1.0.0`
requirements ("any other goframework requirement at v1.0.0 the same way");
the `v0.0.0-00010101…` ones are left. After `--remove --go-mod` they name
a version that exists nowhere, and `go mod tidy` fails on them (expected
from the go command's rules; not reproduced here, since the release does
not exist yet).

Requested, either: `--go-mod` (or a documented `go mod edit`) writes one
placeholder for every goframework requirement, v1.0.0 as go.work users
get; or `--remove --go-mod` rewrites every goframework requirement,
whichever placeholder, to the release; and in both cases the guide's
release steps name both spellings.

## G3. The checklist does not say `go mod tidy` again after the imports of step 6

**Low severity; documentation.** "setting up" says "No requirement has to
be added for the modules an application imports: in a workspace their
packages are found without one". That holds for a go.work. With `--go-mod`
the go command needs a requirement for each module, so the build-tag
imports added in checklist step 6 fail until tidy runs again:

```
internal/players/toolkit.go:9:2: module goframework.io/desktop-ui-platform-all provides package goframework.io/desktop-ui-platform-all and is replaced but not required; to add it:
	go get goframework.io/desktop-ui-platform-all
```

The checklist runs tidy only at setup (step 3, through the `--go-mod`
bullet). Requested: step 6 (and "setting up"'s `--go-mod` bullet) say that
with replace lines, `GOPROXY=off go mod tidy` follows any new goframework
import.

## G4. No exported constants for BACKEND, SCALE, TRAY and XDND_DEBUG

**Low severity; API.** The guide's "environment variables" and checklist
step 7 say a program that sets the variables "should use the constants",
and lists eleven. Four variables the player sets or documents have none,
so the player spells them as strings:

| Variable | Where it is read | Constant |
| --- | --- | --- |
| `GOFRAMEWORK_DESKTOP_UI_BACKEND` | desktop-ui `windowsystem.go` (`backendEnv`, unexported), `platform/backend.go` (literal) | none exported |
| `GOFRAMEWORK_DESKTOP_UI_SCALE` | `platform/scale.go` (literal) | none |
| `GOFRAMEWORK_DESKTOP_UI_TRAY` | `status.go`, `platform/status.go` (literals) | none |
| `GOFRAMEWORK_DESKTOP_UI_XDND_DEBUG` | desktop-ui-platform-x11 `x11_dnd_linux.go` (literal) | none |

The player's uses: `backend.go` (reads BACKEND to respect an explicit
choice), `backend_test.go` and four native tests (set BACKEND),
`internal/shell/tray_test.go` (sets TRAY), `-scale`'s help text, and the
README's XDND trace command. Requested: export `platform.BackendEnv` (or
`ui.BackendEnv`), `platform.ScaleEnv`, `platform.TrayEnv` and
`x11.EnvXdndDebug` beside the others, and use them where the literals are.

## G5. Portable logical-window grouping has no fallback (carried from uitoolkit 9)

**Medium severity; open requirement, unchanged by the move.** The player's
skins are groups of separate windows (Main, equaliser, playlist) that
snap and move together through desktop-ui-rack. On native Wayland an
ordinary toplevel cannot be placed (`wlSurface.Move` in
desktop-ui-platform-wayland's `wayland_linux.go` returns false), so
`Desk.Places()` is false and the group cannot move together. The player
still prefers X11/XWayland on Linux (`playerBackend` in `backend.go`) as a
temporary workaround; an explicit `GOFRAMEWORK_DESKTOP_UI_BACKEND` and
headless runs take precedence, and without X11 the skins render as
separate windows that move independently.

Requested, as before: an application declares the logical group (main
anchor, members, its own skins and layouts, snap and detach behaviour) and
desktop-ui realises it with whatever the platform offers, choosing the
physical representation (separate windows where they can be placed, one
host with docked panes, or compositor cooperation where available) and
reporting what remains impossible. The player would then drop its X11
preference; choosing the window system belongs to the framework. This is
not a request for an impossible Wayland `Move`.

## The port

What the player changed by hand after the two rename commands (checklist
steps 6 to 8):

- [internal/players/toolkit.go](internal/players/toolkit.go) imports
  desktop-ui-platform-all and desktop-ui-themes-all (the shipping build's
  `theme_engine_all`). Every face, the shell and main import
  internal/players, so their tests link the same set; it also keeps
  `Themed`'s init-time `style.LoadTheme("breeze-night")` working (the
  init-order case the guide describes).
- `style.SetFallback(true)` in main (every build said `font_fallback`), and
  in the `font_fallback` test helper's init.
- The tags `font_complex` and `font_emoji` are the player's own: files
  behind them in main and internal/players/playertest import
  desktop-ui-text-complex, and `font_complex` alone calls
  `textcomplex.SetColourEmoji(false)`.
- `UITK_*` variables renamed to `GOFRAMEWORK_DESKTOP_UI_*`
  (`style.SystemFontsEnv` where style is imported, G4 elsewhere); the
  player's own `UITK_PROBE_*` became `MUSIC_PLAYER_PROBE_*`, the prefix
  the player already uses, and the probe tag `uitk_emoji_family_probe`
  became `music_player_emoji_family_probe`.
- Settings are read through `style.ConfigSource("media-player-music.json")`,
  so a file saved under `~/.config/uitoolkit` carries over until the first
  save writes `~/.config/goframework-desktop-ui`; a test pins it.
- One probe test the imports command left importing desktop-ui twice (as
  `app` and `uitoolkit`, its documented fallback where `ui` is taken) uses
  `app` alone.

## Carried over from uitoolkit

`uitoolkit-gaps.md` reviewed uitoolkit v0.23.3 to v0.23.21. Its full text
is in this repository's history (`git log -- uitoolkit-gaps.md`). At
v0.23.21 its 55 items stood as follows; the pictures above show the
resolved behaviour unchanged after the move, and the test suite that pins
those items passes on desktop-ui.

| Item | v0.23.21 result |
| --- | --- |
| 1 Embedded skin registration | Resolved |
| 2 Nested slider callbacks | Resolved |
| 3 List wrapper focus and accessible geometry | Resolved |
| 4 Popup accessibility action | Resolved |
| 5 Hidden grid gaps | Resolved |
| 6 List activation/context policy | Resolved |
| 7 Main root and multi-pane rejoin | Resolved |
| 8 Nearest snapping anchor | Resolved |
| 9 Portable grouping fallback | **Open**: G5 |
| 10 Movement notifications and update loop | Resolved for verified cases |
| 11 Compact-host file chooser | Resolved |
| 12 Utility ownership and taskbar policy | Resolved for tested Linux lifecycle; Wayland capability limits remain |
| 13 Role preservation across X11 recreation | Resolved |
| 14 Remote file URI authority | Resolved |
| 15 Initial X11 frame position | Resolved |
| 16 Programmatic X11 resize geometry | Resolved |
| 17 Generator authoring fields | Resolved |
| 18 X11 shape rectangle ordering | Resolved |
| 19 X11 move-event coordinates | Resolved |
| 20 Registered skins missing from enumeration | Resolved |
| 21 Registered skin replacement paints stale assets | Resolved |
| 22 Disabled list pointer input | Resolved |
| 23 Tray raster icon priority | Resolved |
| 24 Fractional resized panel slices | Resolved |
| 25 Popup focus jumps do not scroll | Resolved |
| 26 Generator loses smooth sheet overrides | Resolved |
| 27 macOS requested positions revert | Attribution withdrawn upstream; no local macOS test |
| 28 List RowAt ignores horizontal viewport | Resolved |
| 29 List double-click history survives interruptions | Resolved |
| 30 Recreated parent leaves stale utility ownership | Resolved |
| 31 GPU image cache retains first sampling filter | Resolved (paintengine2d v0.12.0) |
| 32 Text-field placeholder ignores explicit proportional font | Resolved |
| 33 MenuBar bypasses its bound bar skin part | Resolved |
| 34 Multilingual song titles collapse to missing-glyph boxes | Resolved with fallback and an installed covering font |
| 35 Fallback-family changes leave cached glyphs stale | Resolved |
| 36 Another Application suppresses a GPU window's closing hook | Resolved for tested native Linux lifecycle |
| 37 Window.Close removes its GPU-closing handler early | Resolved |
| 38 Complex-script fallback keeps the no-shaping exclusion | Resolved for verified cases |
| 39 RTL caret, prefix and wrapping helpers assume LTR | Resolved for verified cases |
| 40 Cached emoji runs lose colour when their atlas grows | Resolved |
| 41 Skin-tone emoji split across providers | Resolved for verified cases |
| 42 Emoji presentation leaks across repeated runes | Resolved for verified cases |
| 43 Contextual prefixes exceed width after reshaping | Resolved for verified cases |
| 44 Concurrent emoji fonts share mutable face state | Resolved |
| 45 Joined family emoji split into separate pictures | Resolved |
| 46 Prefix returns less text than fits | Resolved for verified cases |
| 47 Concurrent shaped outline fonts reuse invalid segments | Resolved for verified cases |
| 48 Keycap emoji split across outline and colour faces | Resolved for verified cases |
| 49 Cached shape refresh mutates returned glyph groups | Resolved |
| 50 Keycap marking adds a colour cell to plain text | Resolved |
| 51 Emoji runs swallow following supplementary text | Resolved for recorded cases |
| 52 BMP default emoji presentation needs a selector | Resolved for recorded cases |
| 53 Emoji tag flags lose their country artwork | Resolved for verified flags |
| 54 Maximized-window flip coverage skips under race | Resolved |
| 55 Skip reporter substitutes an earlier log for the skip reason | Resolved for recorded cases |

The font items (34 to 53) were last verified with uitoolkit's build tags;
on desktop-ui the player's `font_fallback`, `font_complex` and
`font_emoji` test runs pass, but their probes under tools/probes were
built, not rerun.
