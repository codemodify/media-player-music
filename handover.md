# Handover: media-player-music (as of 2026-11-09, commit e55ef1e plus uncommitted work)

For whoever picks this up next, a Claude session or a person: what this
is, the rules the owner set, how to build and check it, where it
stands, and what comes next. Read this first, then
[README.md](README.md) (what the player does, the skins, the keys),
[docs/faces.md](docs/faces.md) (the layout, the app/framework boundary,
how it was verified) and [goframework-gaps.md](goframework-gaps.md).

**The build is broken right now** (see "next steps", step 1). And
another session is probably still working in this tree: about 300 files
are changed and not committed. Treat the tree as read-only until that
work is committed.

## What it is

A desktop music player in Go, on goframework.io's desktop-ui (the local
workspace at `~/go/src/goframework.io`). One player model has one
default interface (a 920 × 560 window) and twenty-one optional skins.
They are called "faces" in the code: eleven Winamp skins with compact
snapping panels; Windows Media Player's Headspace, Xbox, Half Life 2,
Fandom and Series 9; iTunes 4 with its MiniPlayer; VLC 0.8, RealPlayer 8
and Amarok 1.4; and Fancy. Audio is played by an **external program**
(mpv, VLC, MPlayer or mpg123), driven over its IPC or remote-control
protocol; nothing is decoded in-process. `-shot DIR` paints posed
screenshots headless with synthetic tracks and starts no audio backend.

Until recently the app was built on the owner's uitoolkit
(github.com/codemodify/uitoolkit). The uncommitted work moves it to
goframework desktop-ui (see "where it stands").

## The owner's rules (follow them)

1. **No Claude attribution, ever.** No `Co-Authored-By: Claude…`
   trailer, no "Generated with Claude Code" line, no footer, in commits,
   PRs or tags. Commits are the owner's alone. This is in
   `~/.claude/CLAUDE.md` and overrides any tool default that asks for
   attribution.
2. **Commit or push only when asked.** The branch is `dev`.
3. **Develop against the local goframework**, through a `go.work`
   written by `tools/app-workspace.py` (not committed), always with
   `GOPROXY=off`. Never a published copy. Do not run `go mod tidy`, and
   do not add framework modules to go.mod by hand.
4. **Use the framework for everything it offers.** Before you write
   infrastructure (audio, codecs, D-Bus, config, process control, file
   watching, UI), look in `~/go/src/goframework.io` (todo.md, media.md,
   desktop-ui.md, the module folders and their `readme`). If it is
   there, use it. If it falls short, keep the smallest workaround and
   log a gap.
5. **Gaps go in goframework-gaps.md**: what we need and why, what the
   framework does today, how to reproduce it, the workaround we keep
   (and its file), and an API to suggest. New gaps go under **Open**. An
   entry moves to **Fixed** only once the fix holds in our own use, with
   a line on how that was checked. **Numbers are never reused.** This
   file numbers them G1, G2, …, so the next one is G6.

## Build, test, vet (to verify once the wiring is fixed)

None of these work today: the build stops with "updates to go.mod
needed". Run Go only in a clean environment that cannot reach the
desktop bus. Never launch the player for real, and never play audio.

```sh
E="env -i HOME=$HOME PATH=$PATH GOPROXY=off DBUS_SESSION_BUS_ADDRESS=unix:path=/nonexistent"
python3 ~/go/src/goframework.io/tools/app-workspace.py .     # writes go.work (not in git)
$E go build -o /dev/null ./...
$E go vet ./...
for os in linux windows darwin; do $E GOOS=$os CGO_ENABLED=0 go vet ./...; done
$E go vet -tags font_complex ./... ; $E go vet -tags font_emoji ./...
$E tools/testenv.sh go test ./...                            # no display, private D-Bus and XDG folders
$E tools/testenv.sh go test -race ./...
$E tools/testenv.sh go test -tags font_complex ./...
$E tools/testenv.sh go test -tags font_emoji ./...
$E go generate ./internal/skins                              # after changing a skin plan: rewrites internal/skins/packs
$E go run . -skin default -shot "$(mktemp -d)" -at 1m37s     # headless pictures, no audio backend
```

- **Audio warning:** some playback and spectrum tests in internal/shell
  and internal/playback start the installed mpv (and VLC, MPlayer,
  mpg123) with null audio output. They skip when the program is
  missing. Under the "never play audio" rule, run only the packages you
  need, or `-run` around those tests, unless the owner agrees to them.
- The native opt-in tests (`MUSIC_PLAYER_NATIVE_XDND_TEST`,
  `MEDIA_PLAYER_NATIVE_WINDOW_TEST`) need a real X11 display. Do not run
  them without the owner.
- `tools/probes/` holds 54 standalone probe programs and tests for
  framework behaviour. Each header names the build tags it needs.

## Where the code is

| Path | What |
| --- | --- |
| `main.go` | flags (`-skin`, `-scale`, `-audio-backend`, `-shot`, `-at`), the theme, `style.SetFallback`, the app icon, signals, `-shot` mode |
| `backend.go` | `playerBackend`: prefers X11/XWayland on Linux so skin windows can be placed (the G5 workaround) |
| `text_complex*.go` | the `font_complex` / `font_emoji` tags bring in desktop-ui-text-complex (shaping, bidi, colour emoji) |
| `internal/playback` | the external engines: `Engine`, mpv (Unix socket or Windows named pipe via go-winio), VLC (local HTTP), MPlayer, mpg123; discovery (`catalog.go`), process control, the spectrum read from an mpv FIFO |
| `internal/players` | the shared model and pieces of interface (host, queue view, settings, skins list, `toolkit.go` imports platform-all and themes-all); one package for each face: `standard` (default), `winamp`, `headspace`, `xbox`, `halflife2`, `fandom`, `itunes`, `series9`, `vlc`, `realplayer`, `amarok`, `fancy`; `playertest` holds the test helpers |
| `internal/shell` | the running app: face switcher, session (saved queue, position, skin, backend), playback wiring, files and drops, tray |
| `internal/skins` | skin registration; `plans/` (the Go plans `skingen` renders), `packs/` (the generated skin.json and art), `sources/` (original archives and credits), `cmd/generate` |
| `internal/appicon` | logo.png made into window and tray icons |
| `tools/testenv.sh` | runs a command with no display and a private session bus |
| `docs/faces.md`, `docs/screenshots/` | the long-form design and the README's pictures |

`media-player-music` (the 56 MB binary at the root) and `out/` are build
and shot output, ignored by git.

## Where it stands

**Committed** (on `dev`, latest first): e55ef1e (2026-10-05) "unify the
music player with classic skins and swappable backends"; before it,
September's commits: the face switcher, one model behind three faces, the
Winamp strip ("minim"), "marquee" and "lantern" faces, and the headless
test environment. All of that is on uitoolkit.

**Uncommitted** (302 entries in `git status` before this handover.md was added; 249 tracked files changed,
+9,778 / −37,941 lines). It is mostly the port to goframework:

- **uitoolkit → desktop-ui**, by desktop-ui-porting.md's checklist:
  imports renamed in about 266 files (`ui.Application`, `ui.Window`,
  `skins.SkinLayoutOf`, …); `UITK_*` variables now
  `GOFRAMEWORK_DESKTOP_UI_*`; `style.SetFallback(true)` and
  `style.SetFollowDesktopMotion(true)` in main; settings read through
  `style.ConfigSource("media-player-music.json")`.
- **go.mod**: +307 lines, the framework requirements and a 244-line
  `replace` block written by `app-workspace.py --go-mod` (the stale
  wiring, below).
- **Skin packs regenerated** by desktop-ui-skingen: every
  `internal/skins/packs/*/skin.json` is much shorter (the old `design`
  keys are gone, and the arrays are compact).
- **A new face, Fancy**: `internal/players/fancy/`,
  `internal/skins/fancy/`, `internal/skins/packs/fancy/`,
  `internal/skins/plans/fancy*.go`, `docs/screenshots/fancy-*.png`, and
  README and docs/faces.md sections.
- **New tests and probes**: desk tests for fandom, halflife2 and winamp;
  font fallback, complex-text and emoji tests in itunes, playertest and
  shell; 24 new probes in tools/probes.
- `uitoolkit-gaps.md` deleted, replaced by the new (untracked)
  `goframework-gaps.md`. `internal/players/vlc/menu.go` deleted.

**The wiring is stale.** go.mod's `replace` block (relative
`../../../goframework.io/…`) was written before the framework added
modules that desktop-ui now pulls in: graphics-svg, system-xkb,
codecs-text-unicode, codecs-text-markdown and maybe others. So
`GOPROXY=off go build ./...` fails with "go: updates to go.mod needed".
Nothing has been built or tested since. The last good run is the one
goframework-gaps.md records, at framework commit a5ebdc8 (2026-10-23).

**Gaps** (goframework-gaps.md was checked against a5ebdc8; the
workspace is now at 6227181, 2026-11-09, 637 commits later):

- Open in our file: G1 (desktop-ui-platform-win32 requires
  system-native-windows v0.0.3, which cannot build it), G2 (`--go-mod`
  then tidy leaves two placeholder versions), G3 (the porting checklist
  does not say to tidy again after step 6), G4 (no exported constants
  for `…_BACKEND`, `…_SCALE`, `…_TRAY`, `…_XDND_DEBUG`), G5 (no portable
  fallback for grouping windows; carried over from uitoolkit item 9).
- The framework's notes (desktop-ui.md, "# media-player-music's gaps")
  say G1–G4 were fixed on 2026-10-23/24 and G5 is now built:
  `rack.Group` in desktop-ui-rack, with one window for each part on
  Wayland. It was probed on KWin and sway on 2026-11-04. None of this is
  in use here yet, so all five stay Open until checked here.
- The 55 uitoolkit items are summarised in the file: all are resolved
  except item 9, which became G5.

## goframework modules

**Used now** (imported in code): desktop-ui (its `platform`, `style`,
`widget`, `a11y`, `layout` packages), desktop-ui-widgets,
desktop-ui-skins, desktop-ui-skingen, desktop-ui-rack,
desktop-ui-text-complex (behind tags), desktop-ui-platform-all,
desktop-ui-themes-all, graphics-paint2d, graphics-paint2d-gpu-gles.

**Could use but does not yet:**

| Module | For |
| --- | --- |
| `audio-playback` | gapless in-process playback, written for this player (media.md, "the players"). It has the equaliser, ReplayGain, balance, volume and an analyser tap. It would replace the external mpv, VLC, MPlayer and mpg123 engines, their IPC and the mpv spectrum FIFO in `internal/playback` |
| `audio-dsp` | `eq`, `replaygain`, `spectrum`, `resample`: the equaliser bands and analyser the faces draw, now faked or done through mpv's filters |
| `codecs-audio-*` | flac, vorbis, opus, alac, g711 (and the spec). There is no mp3 or aac module yet (media.md lists mp3 next; aac is the owner's decision). mpg123 and MP3 files need the external engine until then |
| `codecs-container-*` | ogg, mp4 (M4A), matroska, wav, aiff, au: opening the files, and their tags for the playlist and library |
| `system-audio-*` | the sound output (pulse, alsa, oss, sndio, macos, windows; `-all`), behind audio-playback |
| `system-dbus` | MPRIS (media keys, desktop "now playing"); it would also replace `github.com/godbus/dbus/v5`, which only the tests and probes use now (`internal/players/winamp/native_kwin_test.go`, `tools/probes/status_icon_priority.go`) |
| `system-process-*` | starting and watching the external engines, while they stay (`internal/playback/process.go`, `command_*.go` use os/exec) |
| `vfs-watch`, `vfs` | watching music folders for the iTunes library and dropped folders |
| `config-*` / `vfs-statefile` | the saved session and settings, now a JSON file through `style.ConfigSource` |
| `codecs-text-charsets` | old ID3 and other tags in legacy encodings |
| `logging-*` | the app logs with the standard `log` package |

## Next steps, in order

1. **Fix the wiring, once the in-flight work is committed** (ask the
   owner first; do not touch go.mod while another session is working).
   Run `python3 ~/go/src/goframework.io/tools/app-workspace.py .` to
   write a go.work, add `/go.work` and `/go.work.sum` to .gitignore, then
   remove go.mod's `replace` block (lines 68–318, from the comment
   "Replacements written by goframework.io's tools/app-workspace.py
   --go-mod" to "End of the replacements…"; the tool's `--remove
   --go-mod` takes it out), as
   commander, comms-pbx and office-calculator already do. Then build,
   vet and test with the commands above. Update goframework-gaps.md's
   first paragraph, which says the app builds through the replace block.
   G2 and G3 are about that path, so note whether they still matter to
   us.
2. **Check for framework changes** since a5ebdc8: `git log` in
   `~/go/src/goframework.io` and in each module folder (each is its own
   repo), and grep the notes for "media-player-music" (desktop-ui.md,
   desktop-ui-porting.md, media.md, todo.md). Rebuild, then confirm the
   G1–G4 fixes here and move them to Fixed, with how each was checked.
   Update the "checked against" commit.
3. **Adopt G5's `rack.Group`**: declare each skin's windows as a group
   and drop `playerBackend`'s X11 preference (`backend.go`, the README's
   "prefers X11" paragraph). Check headless and on the offscreen
   backend; native Wayland only with the owner.
4. **Re-shoot the pictures** with `-shot` for every skin at 1× and 1.75×
   and compare them with the uitoolkit pictures, as was done at a5ebdc8.
5. **Move playback into the framework** (ask the owner: it changes what
   the app is). Add an audio-playback engine beside the external ones,
   behind the same `playback.Engine`, using system-audio-* for output,
   audio-dsp for the equaliser and analyser, and codecs-container-* for
   tags. Keep mpv and the others for MP3 and AAC until the codecs exist,
   and log what is missing (MP3 first) as gaps.
6. **MPRIS on system-dbus**, and port the godbus tests and probes to it.
7. **Smaller uses**: system-process for the external engines,
   vfs-watch for library folders, logging, config.

Ask the owner before starting any of these, and commit only when asked.
