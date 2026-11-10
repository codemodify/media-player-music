# Handover: media-player-music (as of 2026-11-09, commit 1da0430 plus the rewiring, uncommitted)

For whoever picks this up next, a Claude session or a person: what this
is, the rules the owner set, how to build and check it, where it
stands, and what comes next. Read this first, then
[README.md](README.md) (what the player does, the skins, the keys),
[docs/faces.md](docs/faces.md) (the layout, the app/framework boundary,
how it was verified), then this app's rows in
`~/go/src/goframework.io/app-requests.md` (R21, R33, R34).

> **Since 2026-11-09 goframework-gaps.md is retired** (its last version is at
> 1da0430; a "goframework-gaps.md G<n>" or a link to it in the code or the docs,
> docs/faces.md's included, means that file). What media-player-music needs
> from the framework, and what it still has to adopt, is in the framework's
> one ledger, `/home/user/go/src/goframework.io/app-requests.md` (G1-G5 are
> R21 there). New needs go by message to the framework session, with a row
> added there (../CLAUDE.md, "The loop"), and each workaround is tagged in
> code with its ledger id (`// R21`). Read every "log a gap in
> goframework-gaps.md" below that way.

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
5. **Gaps go to the framework's ledger** (app-requests.md, see the note
   at the top): what we need and why, a repro or file:line, the
   workaround we keep (file:line, tagged `// R<n>` in code), and an API
   to suggest. A row closes only once the fix holds in our own use, with
   a line on how that was checked.
6. **Heavy Go commands run capped**: through
   `~/go/src/goframework.io/tools/capped.sh` (the owner, 2026-11-09), and
   with `free -g` showing at least 16G available first. An exit 137 means
   the cap was hit: run less at once, never raise the cap.

## Build, test, vet (checked 2026-11-09)

go.mod names no goframework module and has no `replace` block: the
go.work that `app-workspace.py` writes wires every workspace module
(`/go.work` and `/go.work.sum` are in .gitignore). Run it again whenever
the framework gains a module. Never run `go mod tidy` (it ignores
go.work). Run Go only in a clean environment that cannot reach the
display or the desktop bus, and heavy commands through capped.sh (GOOS,
GOMEMLIMIT and GOFLAGS go **inside** the `env -i`, which drops anything
set before it). Never launch the player for real, and never play audio.

```sh
C=~/go/src/goframework.io/tools/capped.sh
E="env -i HOME=$HOME PATH=$PATH GOPROXY=off DBUS_SESSION_BUS_ADDRESS=unix:path=/nonexistent GOMEMLIMIT=2GiB GOFLAGS=-p=4"
python3 ~/go/src/goframework.io/tools/app-workspace.py .     # writes go.work (not in git)
$C $E go build ./...
for os in linux windows darwin freebsd; do $C $E GOOS=$os go vet ./...; done
$C $E go vet -tags font_complex ./... ; $C $E go vet -tags font_emoji ./...
# the packages that start no audio engine, no tray and no native window
# (XDG_* pointed at a scratch folder keeps the settings tests off ~/.config):
$C $E XDG_CONFIG_HOME=$T/config XDG_CACHE_HOME=$T/cache XDG_DATA_HOME=$T/data XDG_RUNTIME_DIR=$T/run \
  go test -race -timeout 40m . ./internal/appicon ./internal/players ./internal/players/playertest ./internal/skins/...
$C $E XDG_CONFIG_HOME=$T/config XDG_CACHE_HOME=$T/cache XDG_DATA_HOME=$T/data XDG_RUNTIME_DIR=$T/run \
  go test -race ./internal/players/{amarok,fancy,fandom,halflife2,headspace,itunes,realplayer,series9,standard,vlc,xbox}
# the rest only with the owner's agreement (see the audio warning):
$E tools/testenv.sh go test ./internal/playback ./internal/shell ./internal/players/winamp
$E go generate ./internal/skins                              # after changing a skin plan: rewrites internal/skins/packs
$E go run . -skin default -shot "$(mktemp -d)" -at 1m37s     # headless pictures, no audio backend
```

Results on 2026-11-09 (linux/amd64, go 1.27.3, workspace d445be2):
- `go build ./...`, and `go vet ./...` for linux, windows, darwin and
  freebsd, and with `-tags font_complex` and `-tags font_emoji`: pass.
  All 54 probes vet.
- `go test -race`: pass for the root package, internal/appicon,
  internal/players, the eleven faces other than winamp, internal/skins
  and its subpackages (internal/skins takes about 8 minutes under -race,
  past go test's default 10-minute timeout when it shares the cap with
  other packages, hence `-timeout 40m`).
- Not run (see the audio warning): internal/playback, internal/shell,
  internal/players/winamp.
- internal/skins' TestGeneratedPacksMatchBundle found
  packs/winamp-silver/art/chrome.png and chrome@2x.png out of date with
  their plan: 22 and 40 pixels on anti-aliased edges, largest channel
  difference 9 and 11 of 255, from the framework's rasteriser since
  a5ebdc8. They were regenerated (the generator's output into a scratch
  folder matches every other pack byte for byte). Re-shooting the
  pictures (next steps) would show it.

- **Audio warning:** some playback and spectrum tests in internal/shell
  and internal/playback start the installed mpv (and VLC, MPlayer,
  mpg123) with null audio output; internal/players/winamp's tests run
  the shell (`shell.Start` plays the queue), and shell's tray tests and
  tools/testenv.sh use a session bus (a private one). They skip when the program is
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

**Committed** (on `dev`, latest first): 1da0430 (2026-11-09) "the port
from uitoolkit to goframework, as it stood" (it did not build: go.mod's
244-module replace block was stale); e55ef1e (2026-10-05) "unify the
music player with classic skins and swappable backends"; before it,
September's commits on uitoolkit. 1da0430 holds the port: imports renamed
in about 266 files by desktop-ui-porting.md's checklist, `UITK_*`
variables made `GOFRAMEWORK_DESKTOP_UI_*`, `style.SetFallback(true)` and
`style.SetFollowDesktopMotion(true)` in main, settings through
`style.ConfigSource("media-player-music.json")`, the skin packs
regenerated by desktop-ui-skingen, the new Fancy face, new tests and 24
new probes, and goframework-gaps.md in place of uitoolkit-gaps.md.

**Uncommitted, for the coordinator to commit** (2026-11-09): the
rewiring, and the framework's answers adopted.
- **Wiring:** go.mod's replace block and every goframework.io require
  are gone; go.work alone wires the framework, as comms-mail bb1c695
  does. `/go.work` and `/go.work.sum` are in .gitignore. The third-party
  requires that stay are all used: go-winio (internal/playback's Windows
  pipe), golang.org/x/image (internal/appicon, internal/skins/plans),
  and go-text/typesetting, x/sys and x/text (indirect, and the probes).
  The build needed no change for the framework's API since a5ebdc8.
- **godbus gone:** v5.1.0 does not build for FreeBSD (`*unixTransport`
  lacks `SendNullByte`), so `GOOS=freebsd go vet` failed. Its two users
  (internal/players/winamp/native_kwin_test.go and
  tools/probes/status_icon_priority.go) are on system-dbus now, and
  godbus is out of go.mod and go.sum. The KWin test is opt-in
  (`MEDIA_PLAYER_KWIN_AUDIT_BUS`) and was vetted, not run.
- **R33 (G4):** `platform.EnvBackend` and `platform.EnvTray` in backend.go,
  backend_test.go, the four native tests and tray_test.go;
  `platform.EnvScale` in `-scale`'s help. No spelled
  `GOFRAMEWORK_DESKTOP_UI_*` is left in Go code; the probes' headers and
  the README keep them in shell commands, where a name is what one types.
- **G1-G3:** hold here. desktop-ui-platform-win32 now requires
  system-native-windows v0.0.5 and release.py's prepare sets every
  requirement to the release (G1); with go.work and no goframework
  requirement in go.mod, the two placeholders and the second tidy (G2,
  G3) no longer arise here. A release uses `app-workspace.py --remove`,
  then requires the published version.
- **G5 (R21) still open:** desktop-ui-rack's `Group` exists but cannot
  take the racked faces as they are (see "next steps" and the R21
  comment on `playerBackend` in backend.go). The X11 preference stays,
  tagged `// R21`.
- All 54 probes under tools/probes vet with the tags their headers name.

## goframework modules

**Used now** (imported in code): desktop-ui (its `platform`, `style`,
`widget`, `a11y`, `layout` packages), desktop-ui-widgets,
desktop-ui-skins, desktop-ui-skingen, desktop-ui-rack,
desktop-ui-text-complex (behind tags), desktop-ui-platform-all,
desktop-ui-themes-all, graphics-paint2d, graphics-paint2d-gpu-gles;
system-dbus in the KWin audit test and the tray probe.

**Could use but does not yet:**

| Module | For |
| --- | --- |
| `audio-playback` | gapless in-process playback, written for this player (media.md, "the players"). It has the equaliser, ReplayGain, balance, volume and an analyser tap. It would replace the external mpv, VLC, MPlayer and mpg123 engines, their IPC and the mpv spectrum FIFO in `internal/playback` |
| `audio-dsp` | `eq`, `replaygain`, `spectrum`, `resample`: the equaliser bands and analyser the faces draw, now faked or done through mpv's filters |
| `codecs-audio-*` | flac, vorbis, opus, alac, g711 (and the spec). There is no mp3 or aac module yet (media.md lists mp3 next; aac is the owner's decision). mpg123 and MP3 files need the external engine until then |
| `codecs-container-*` | ogg, mp4 (M4A), matroska, wav, aiff, au: opening the files, and their tags for the playlist and library |
| `system-audio-*` | the sound output (pulse, alsa, oss, sndio, macos, windows; `-all`), behind audio-playback |
| `system-dbus` | MPRIS (media keys, desktop "now playing"); it already replaced godbus in the tests and probes |
| `system-process-*` | starting and watching the external engines, while they stay (`internal/playback/process.go`, `command_*.go` use os/exec) |
| `vfs-watch`, `vfs` | watching music folders for the iTunes library and dropped folders |
| `config-*` / `vfs-statefile` | the saved session and settings, now a JSON file through `style.ConfigSource` |
| `codecs-text-charsets` | old ID3 and other tags in legacy encodings |
| `logging-*` | the app logs with the standard `log` package |

## Next steps, in order

1. **Commit the rewiring** (the coordinator), then update this app's
   rows in app-requests.md: R33 adopted here (no spelled name left in
   Go code), R21's G1-G4 adopted, G5 still open with the two needs below.
2. **G5 / R21, `rack.Group`.** Group realises a group on Wayland as one
   window per part, which is what G5 asked for, but two things keep the
   four racked faces (winamp, fancy, fandom, halflife2) on `rack.Desk`:
   - Group opens every member with `DecorationsNone`, and a hosted part
     shares one window among its members, so the Winamp panels, which
     each get their skin frame from `Window.SetFrameRole` ("strip",
     "equaliser", "playlist") on client decorations, would lose it.
     Asked for: a member's frame role (say `Member.FrameRole`), each
     member's pane dressed in the look's frame for its role, hosted or
     not.
   - Group's settle puts back only the declared bonds; a saved, undocked
     panel is left where the window manager mapped it, where the faces'
     own settle retries their saved placement. Asked for: declared
     positions kept like bonds (say `Group.Place(i, x, y)` before
     Realise, or the member's `Window.X/Y` honoured through settle).
   Once both are in, move the faces to Group (fancy, fandom and
   halflife2 draw their own frames already, so they need only the
   second), drop `playerBackend` and the README's "prefers X11"
   paragraph, check headless and on the offscreen backend, and native
   Wayland only with the owner.
3. **Run the skipped tests** with the owner's agreement: internal/playback,
   internal/shell and internal/players/winamp (audio engines, the shell's
   playback, the tray on a private bus).
4. **Re-shoot the pictures** with `-shot` for every skin at 1× and 1.75×
   and compare them with the uitoolkit pictures, as was done at a5ebdc8.
5. **Move playback into the framework** (ask the owner: it changes what
   the app is). Add an audio-playback engine beside the external ones,
   behind the same `playback.Engine`, using system-audio-* for output,
   audio-dsp for the equaliser and analyser, and codecs-container-* for
   tags. Keep mpv and the others for MP3 and AAC until the codecs exist,
   and log what is missing (MP3 first) as gaps.
6. **MPRIS on system-dbus.**
7. **Smaller uses**: system-process for the external engines,
   vfs-watch for library folders, logging, config.

Ask the owner before starting any of these, and commit only when asked.
