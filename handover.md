# Handover: media-player-music (as of 2026-11-12, R94 and R95 adopted; see the last section)

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
and Amarok 1.4; and Fancy. Audio is played **in the player's process** by
default (the "Built-in" engine on goframework's audio-playback, since
2026-11-11, R36), or by an **external program** (mpv, VLC, MPlayer or
mpg123), driven over its IPC or remote-control protocol. `-shot DIR` paints posed
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
| `text_complex*.go` | the `font_complex` / `font_emoji` tags bring in desktop-ui-text-complex (shaping, bidi, colour emoji) |
| `internal/playback` | `builtin.go`: the default engine, in-process on goframework's audio-playback (R36, 2026-11-11); the external engines: `Engine`, mpv (Unix socket or Windows named pipe via go-winio), VLC (local HTTP), MPlayer, mpg123; discovery (`catalog.go`), process control, the spectrum read from an mpv FIFO |
| `internal/players` | the shared model and pieces of interface (host, queue view, settings, skins list, `toolkit.go` imports platform-all and the breeze, luna and win95 engines); `racked.go` the four panelled faces' shared Group pieces (windows named once, a layout saved and restored); one package for each face: `standard` (default), `winamp`, `headspace`, `xbox`, `halflife2`, `fandom`, `itunes`, `series9`, `vlc`, `realplayer`, `amarok`, `fancy`; `playertest` holds the test helpers |
| `internal/shell` | the running app: face switcher, session (saved queue, position, skin, backend), playback wiring, files and drops, tray |
| `internal/skins` | skin registration; `plans/` (the Go plans `skingen` renders), `packs/` (the generated skin.json and art), `sources/` (original archives and credits), `cmd/generate` |
| `internal/appicon` | logo.png made into window and tray icons |
| `internal/tags` | ID3 tags (codecs-container-id3) for the playlist and the front cover for the default layout's display; `tagstest` writes ID3 tags for tests |
| `tools/testenv.sh` | runs a command with no display and a private session bus |
| `docs/faces.md`, `docs/screenshots/` | the long-form design and the README's pictures |

`media-player-music` (the 56 MB binary at the root) and `out/` are build
and shot output, ignored by git.

## Where it stands

**Committed** (on `dev`, latest first): c64f45b (2026-11-10) the rewiring
(goframework through go.work only, `platform.Env*` constants for R33,
godbus replaced by system-dbus so FreeBSD vets, winamp-silver's chrome
regenerated, goframework-gaps.md retired); 1da0430 (2026-11-09) the port
from uitoolkit to goframework, as it stood; e55ef1e (2026-10-05) "unify
the music player with classic skins and swappable backends"; before it,
September's commits on uitoolkit.

**Uncommitted, for the coordinator to commit** (2026-11-10): the four
panelled faces moved from `rack.Desk` to `rack.Group` (R21, framework
b98836d), and the X11 preference dropped.
- **winamp, fancy, fandom, halflife2** declare their panels in `New` as a
  Group's members (`g.Add`: content, window options, the skin's shape
  function, `OnClose`, and for winamp `FrameRole` strip/equaliser/
  playlist), their bonds (`Attach`; `Stick` for fandom's playlist 46 down
  the player's right edge and halflife2's equaliser 80 up it), then
  `Realise`. `Main()`, `Eq()` and `List()` are `g.Window(i)` lookups;
  `Windows()` lists each window once (`players.GroupWindows`); show and
  hide are `g.Show`, close `g.Close`; every caption, display, footer,
  analyser and grip press is `g.StartMove(i)` / `g.StartResize(i, …)`
  (`players.DragsWindow.Move` and `AnalyserView.Move` are funcs now). A
  face's keys ask whether the focus is in the playlist's pane, not which
  window it came from. Gone: each face's group.go and docking.go (the
  settle, the follow polling, `placed`, `AdvanceGeometry`, `userMoved`),
  winamp's `open()`, the per-face placement code (one `CaptureGroup` /
  `RestoreGroup` in internal/players/racked.go, the saved bonds put back
  from the player out so a turned-round layout is not refused), and
  playertest's `AuditDeskRuntime` (the rack's own tests cover it).
  `configureUtilityWindows` and winamp's caption drop scope run only
  with a window each. Fancy's compact mode is `g.SetContent`/`SetSize`/
  `SetShape`; fandom's settings wing and halflife2's shutter recut
  through `g.SetShape`. A model sync on `g.Moved` keeps the equaliser
  and playlist switches true when the desktop closes a shared part's
  window. Each face's `Options.Group` can insist on a mode (tests force
  `rack.GroupHosted`).
- **The shell** gives a racked face its close-to-tray through
  `SetOnClose` (the first member's `OnClose`), not the main window's
  close handler, which the group replaces when it falls back to sharing
  windows (sway as an X11 window manager).
- **backend.go and backend_test.go are gone**: `ui.New` takes the
  toolkit's own backend choice. README and docs/faces.md say how the
  panels behave on each kind of desktop now.
- **R21 kept, one workaround** (internal/players/racked.go, `HeldSize`):
  Group keeps declared bonds and places while it settles but adopts each
  window's size from the desktop, so a late first configure undid the
  restored playlist size (winamp's TestPlaylistStartupResize…). Each face
  puts the restored size back on its clock for the two seconds a group
  may settle, released by the playlist's grip. What Group lacks: a size
  declared with SetSize before Realise, or while it settles, kept like a
  place. Also not in Group: a press on a caption during the settle
  (StartMove) does not end it, so a drag in the first fifth of a second
  is put back; fandom's test of that went, and no workaround is kept.
- Tests: the Desk tests moved to the Group (fandom and halflife2
  group_test.go, fancy's group test, winamp's docking, docking_event,
  placement and resize tests; settled with `playertest.SettleGroup`, the
  desktop's moves through `SimulateDesktopMove`); tests that added extra
  windows to the face's desk went (the rack's own tests cover a long
  chain and a reentrant follow). New: a hosted_test.go per face (one
  shared window, its size, the shapes, the keys' pane, hide and show in
  place, the saved layout unpositioned, a loose playlist in a window of
  its own, fancy's compact mode, winamp's framed panes). The four probes
  that open faces (tools/probes/fandom_*, halflife2_*) follow.
- Checked 2026-11-10 (linux/amd64, go 1.27.3, framework b98836d): `go
  build ./...`; `go vet ./...` for linux, windows, darwin and freebsd;
  `go test -race` passes for internal/players, fancy, fandom, halflife2,
  winamp (skipping TestShellTimerRebindsPlaylistCaptionDrops, which
  starts the shell's playback) and internal/skins/... The four probes
  that open faces vet. In the desk VM (as `desktop`, MEDIA_PLAYER_NATIVE_WINDOW_TEST=1):
  winamp's TestNativeDesktopMovesFollowAndRejoinWithoutPlaybackTicks
  passes under Xvfb, where the committed Desk code failed it (the stack
  never followed the strip); under KWin's Xwayland it fails as the Desk
  code did: KWin puts the playlist at 411,454 itself after the group has
  settled, and the desk takes that for the user's drag and detaches it
  (to look into with the framework). TestNativePlaylistResizeAndSavedSize
  skips there (no xprop in desk). Native Wayland was not run.

## goframework modules

**Used now** (imported in code): audio-playback (with `/all`: every
container and codec, MP3 and AAC among them), audio-dsp (eq, spectrum),
system-audio-spec (and audiotest in tests), codecs-container-id3 (R36,
2026-11-11); desktop-ui (its `platform`, `style`,
`widget`, `a11y`, `layout` packages), desktop-ui-widgets,
desktop-ui-skins, desktop-ui-skingen, desktop-ui-rack,
desktop-ui-text-complex (behind tags), desktop-ui-platform-all,
desktop-ui-themes-breeze, -luna and -win95 (R28), graphics-paint2d, graphics-paint2d-gpu-gles;
system-dbus in the KWin audit test and the tray probe.

**Could use but does not yet:**

| Module | For |
| --- | --- |
| `audio-dsp/replaygain` | ReplayGain (audio-playback's Settings.ReplayGain): the built-in engine plays without it, as mpv did by default; a setting for it would be the app's |
| `codecs-container-*` | the tags of FLAC, Ogg and M4A files for the playlist before they play (now ID3 only, `internal/tags`; the built-in engine reports every format's tags as it plays) |
| `system-dbus` | MPRIS (media keys, desktop "now playing"); it already replaced godbus in the tests and probes |
| `system-process-*` | starting and watching the external engines, while they stay (`internal/playback/process.go`, `command_*.go` use os/exec) |
| `vfs-watch`, `vfs` | watching music folders for the iTunes library and dropped folders |
| `config-*` / `vfs-statefile` | the saved session and settings, now a JSON file through `style.ConfigSource` |
| `codecs-text-charsets` | old ID3 and other tags in legacy encodings |
| `logging-*` | the app logs with the standard `log` package |

## Next steps, in order

1. **Commit the move to rack.Group** (the coordinator), then update R21
   in app-requests.md: G5 adopted (the faces on Group, the X11
   preference gone), and the new needs above as a row of their own (a
   declared size kept while the group settles; a caption press ending
   the settle), after the boundary test.
2. **Native Wayland with the owner**: the faces under KWin and sway over
   Wayland (one window per part, a panel dragged out and docked back),
   and the saved layout across a Wayland and an X11 session.
3. **Run the skipped tests** with the owner's agreement: internal/playback,
   internal/shell and internal/players/winamp (audio engines, the shell's
   playback, the tray on a private bus).
4. **Re-shoot the pictures** with `-shot` for every skin at 1× and 1.75×
   and compare them with the uitoolkit pictures, as was done at a5ebdc8.
5. **Done 2026-11-11 (R36):** the built-in engine is the default. Left
   from this step: whether the external engines stay (the owner's call). Was: **Move playback into the framework** (ask the owner: it changes what
   the app is). Add an audio-playback engine beside the external ones,
   behind the same `playback.Engine`, using system-audio-* for output,
   audio-dsp for the equaliser and analyser, and codecs-container-* for
   tags. Keep mpv and the others for MP3 and AAC until the codecs exist,
   and log what is missing (MP3 first) as gaps.
6. **MPRIS on system-dbus.**
7. **Smaller uses**: system-process for the external engines,
   vfs-watch for library folders, logging, config.

Ask the owner before starting any of these, and commit only when asked.


## goframework round, 2026-11-11 (R66, R33, R34): 51cc616, 9d94e8f
- internal/players/standard/body.go and internal/players/fancy/body.go
  import `desktop-ui/fonts/jetbrainsmono`: the default face's clock and
  countdown and Fancy's clock are drawn in the monospaced face
  (`lk.MonoFont()`), and since R66 JetBrains Mono is only in a program that
  imports it. (The player also gets it through desktop-ui-themes-all, whose
  Amiga look now imports it, but the faces say it themselves.) The winamp
  faces draw their clock in the skin's LED digits and their text in the
  skin's pixel font, so the winamp pictures do not depend on it.
- R33: nothing left in code; the only spelled names are the shell command
  lines in the comments of three ignore-tagged probes
  (tools/probes/font_fallback_regressions_test.go, unicode_song_titles.go,
  font_complex_0238.go), where a Go constant cannot stand.
- R34: internal/players/toolkit.go's desktop-ui-themes-all import tagged
  `R28` (themes on demand would let the player link fewer engines).
- R28 (closed 2026-11-11): toolkit.go imports only the engines the player
  draws with: breeze (the default face's breeze-night, `Themed`), win95
  and luna (the skins built over win95 and luna, internal/skins/plans).
  A look.json naming any other pack shows the nearest pack the player
  has (`style.NearestThemePack`), logged once by desktop-ui (diag, area
  `theme`). 40.65 to 34.10 MB, stripped, linux/amd64, CGO_ENABLED=0.
  Checked: vet on 4 systems; `go test -race ./...` passes, internal/skins
  with `-timeout 40m` as below. JetBrains Mono no longer comes through
  the Amiga engine: the faces import it themselves (R66).
- Not changed, for later: tools/probes' widget_regressions_0235_test.go and
  font_fallback_regressions_test.go use `style.FamilyMono` as a bundled
  family; they are ignore-tagged framework probes, run by hand, and need
  the jetbrainsmono import if they are run again.
- Checked: vet on linux, windows, darwin, freebsd; `go test -race` of
  internal/players, standard, fancy and internal/skins/... (the generated
  packs, winamp-silver's included, match the bundle: TestGeneratedPacksMatchBundle).
  Not run: internal/playback, internal/shell, internal/players/winamp
  (players, the shell, the bus); the native window tests stay skipped.

## the owner, 2026-11-11
- The regenerated winamp-silver pictures are kept.

## goframework round, 2026-11-11 (R36 adopted: MP3, AAC, ID3; R94, R95 found)
The owner approved adopting R36's audio half. What changed:
- **The built-in engine** (`internal/playback/builtin.go`, `BuiltinID`
  "builtin", "Built-in" in Settings, listed first by `Detect`): an
  `Engine` on audio-playback, `audio-playback/all` imported (MP3, MP1/MP2,
  AAC LC/HE-AAC/HE-AAC v2 in .aac and M4A, FLAC, Ogg Vorbis/Opus, ALAC,
  WAV, AIFF, AU, in MP4 and Matroska too) and codecs-container-id3 (the
  stream's Tags: now-playing title, artist, album). One audio-playback
  player per load (the frontend owns the queue; a load replaces the
  player, so nothing of an old generation reaches the new one); commands
  run in order on the engine's goroutine, off the UI thread. Volume is
  the slider's cube (mpv's curve), balance and the ten-band EQ are
  audio-playback's, the analyser's 48 bands are normalised from -60 dB as
  mpv's were, positions every 50 ms. Sound through system-audio-all.
- **The default**: `NormalizeAudioBackend("")` and unknown ids give
  "builtin" (was mpv); a saved choice of mpv, VLC, MPlayer or mpg123 is
  kept. `-audio-backend builtin`. Settings' window is 550 high (min 520)
  for five rows. **For the owner:** new installs play in-process; the
  external engines remain for formats the framework lacks (WMA, APE,
  Musepack, WavPack...).
- **ID3 for the library and now-playing** (`internal/tags`): files added
  (`musicTrack`) show their ID3 title, artist and album at once (MP3, MP2,
  MP1, .mpga, .aac; v1, v1.1, v2.2, 2.3, 2.4, joined as id3.Read joins
  them), the file name where there is none; the playing file's front cover
  (APIC; PIC in 2.2; JPEG or PNG by the standard library, at most 4096 x
  4096, kept at 256 px) is read off the UI thread at each load
  (`Shell.loadCover`, `Model.Cover`/`CoverPath`) and drawn at the right of
  the default layout's display (`standard/body.go`); the skins do not draw
  it.
- **R94 (framework bug, row in app-requests.md)**: codecs-container-id3
  reads an ID3v2.4 frame of 128 bytes or more by its plain size, so the
  cover and every frame after it are lost. Workaround `readV24` in
  internal/tags/tags.go (tagged R94): the tag read again with its frames'
  sizes written as plain integers. Drop it once the module is fixed. The
  built-in engine's now-playing tags come through the same reader (the
  Tags registry), so a 2.4 tag's frames after a big one are missing there
  until the fix; the library's are right.
- **R95 (framework gap)**: audio-playback cannot start a track at a time
  (Jump from the start; Seek before the track opens is ErrNoTrack; no
  Seeked for a seek before anything is heard). Workaround in builtin.go
  (tagged R95): a load at a position plays paused, Seek retried each 50 ms
  (5 s at most), then plays as asked; the engine reports UpdateSeeked
  itself.
- go.mod: nothing to drop (go-winio is mpv's Windows pipe, x/image the skin
  plans' BMPs, go-text through desktop-ui-text-complex; no audio or tag
  library was there).
- Tests: internal/playback/builtin_test.go (audiotest's sound system in a
  synctest bubble, no device: MP3 and AAC played to their end with
  positions, 48 bands, sound in the system; ID3 tags as metadata; a load at
  600 ms paused then played on; seek, replacement, a missing file, stop,
  Close), testdata/lame-mono22.mp3 and ffmpeg-mono22.aac copied from the
  framework's own test files (testdata/readme); internal/tags/tags_test.go
  (every ID3 version, v1 joined, PNG and JPEG covers scaled, the R94 case),
  internal/shell/tags_test.go (a folder's tracks with their tags),
  standard_test.go TestNowPlayingDisplayDrawsTheCover, the backend settings
  tests with five rows at the new minimum.
- Size, stripped, linux/amd64, CGO_ENABLED=0: 34.10 to 36.55 MB.
- Checked: vet on linux, windows, darwin, freebsd; `go test -race` of the
  root, internal/appicon, internal/tags/..., internal/players, the eleven
  faces but winamp, internal/skins/...; internal/playback with `-run
  'TestBuiltin|TestDiscovery'` and internal/shell with `-run` of the file
  tests (the rest of both start mpv or the shell's playback); the built-in
  engine was never run against a real sound device (tests only).
- Next: drop R94's and R95's workarounds when the framework answers; the
  owner on the external engines' future and on a ReplayGain setting.

## goframework round, 2026-11-12 (R94 and R95 adopted, workarounds gone)
- **R94** fixed in codecs-container-id3 9c85ebc (frameSize4's precedence:
  an ID3v2.4 frame of 128 bytes or more was taken at its plain size). The
  same bug was in codecs-audio-mp3's own ID3v2 skip (c85724a): an MP3 whose
  tag's size was 128 bytes or more had that tag unseen, its now-playing
  text unread through the Tags registry, and one behind a tag past 64 kB
  (a big cover) did not open at all in the built-in engine. `readV24` is
  gone from internal/tags/tags.go: `id3.Read` alone, the R94 case of
  tags_test.go (a 2.4 APIC then TALB, ID3v1 joined) passing on it.
- **R95** answered by audio-playback 61e7390: `JumpAt(index, at)` starts a
  track at a time (the stream opening on it), and a Seek right after a
  load seeks the track loaded though it has not opened; Seeked comes for
  the first track heard from a time. builtin.go: a load is
  `JumpAt(0, Position)` (then Pause when asked paused); OpSeek is one
  `Seek`; UpdateSeeked comes from the player's Seeked event, matched to the
  last seek's time and carrying its epoch (it comes while paused too, the
  stream playing silence from the new place). The paused-until-open
  start, the 50 ms Seek retries and the 5 s give-up are gone.
- Tests: internal/playback TestBuiltinSeeksBeforeTheTrackOpens (a seek
  right after a load: one UpdateSeeked of its epoch at 800 ms, heard from
  there); TestBuiltinLoadsAtAPositionPausedThenPlays and the rest pass as
  they were.
- Checked: vet on linux, windows, darwin, freebsd; `go test -race` of
  internal/tags/..., internal/playback with `-run
  'TestBuiltin|TestDiscovery'`, internal/shell with `-run
  TestLibraryTracksCarryTheirID3Tags`, internal/players/standard. Not run:
  the player, real sound devices, the rest of internal/playback and
  internal/shell (they start mpv or the shell's playback).
- Next: the owner on the external engines' future and on a ReplayGain
  setting.
