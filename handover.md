# Handover: media-player-music (as of 2026-11-13: the daemon split, the library, playlists, the TUI and the WUI; then F1, APE and the GUI's library window; then the skins redrawn and renamed; see the last three sections)

For whoever picks this up next, a Claude session or a person: what this
is, the rules the owner set, how to build and check it, where it
stands, and what comes next. Read this first, then
[README.md](README.md) (what the player does, the skins, the keys),
[docs/faces.md](docs/faces.md) (the layout, the app/framework boundary,
how it was verified), then this app's rows in
`~/go/src/goframework.io/app-requests.md` (R21, R101, R102, R114, R121-R123).

> **Since 2026-11-09 goframework-gaps.md is retired** (its last version is at
> 1da0430). What media-player-music needs from the framework is in the
> framework's one ledger, `/home/user/go/src/goframework.io/app-requests.md`.
> New needs go there (../CLAUDE.md, "The loop"), and each workaround is
> tagged in code with its ledger id (`// R21`).

## What it is

A music player in Go on goframework.io, in four programs (since 2026-11-13;
the owner: "no backward compatibility is needed"):

| program | path | what |
|---|---|---|
| media-player-musicd | cmd/media-player-musicd | the daemon: owns the queue, the track and its place, the volume, the equaliser, ReplayGain, the library and the session; plays on goframework's audio-playback (the built-in engine, internal/playback); serves `musicapi.Player` on app-spec (clienthost through app-as-service-w-plugins): the user's Unix socket (LocalSockets) and ws://127.0.0.1:50130/clients with the token in media-player-musicd.token (R112) |
| media-player-music | main.go, internal/shell, internal/players | the desktop UI: the default face and twenty-one skins ("faces"), each drawing the shell's model, a copy of the daemon's state; what a face changes is sent to the daemon |
| media-player-music-tui | cmd/media-player-music-tui, internal/tui | a cmus-like terminal UI on terminal-ui: library, queue, search, settings (EQ, ReplayGain, folders), now playing |
| media-player-music-wui | cmd/media-player-music-wui | the web UI: a remote control (player, queue, library, search, settings) in a browser; launch link and session (R112), the page's WebSocket passed to the daemon with its token |

The daemon tells its renderers four states (app-spec's newest-state
channel: SetState/OnState): `player.state` (State, at each change, with a
`Seq`), `player.queue` (the tracks, at each change of the queue),
`player.now` (position and the analyser's 48 bands, about twenty a second
while playing) and `player.library` (the library in short). Renderers ask
the library with Browse and Search. `-shot DIR` paints posed screenshots
headless with synthetic tracks, no daemon, and plays nothing.

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

## Build, test, vet (checked 2026-11-13)

go.mod names no goframework module and has no `replace` block: the
go.work that `app-workspace.py` writes wires every workspace module
(`/go.work` and `/go.work.sum` are in .gitignore). Run it again whenever
the framework gains a module (a missing one shows as "module lookup
disabled by GOPROXY=off"). Never run `go mod tidy`. Run Go only in a clean
environment that cannot reach the display or the desktop bus, and heavy
commands through capped.sh (GOOS, GOMEMLIMIT and GOFLAGS go **inside** the
`env -i`). Never launch the player or the daemon for real, never play
audio, never write into bin/.

```sh
C=~/go/src/goframework.io/tools/capped.sh
E="env -i HOME=$HOME PATH=$PATH GOPROXY=off DBUS_SESSION_BUS_ADDRESS=unix:path=/nonexistent GOMEMLIMIT=2GiB GOFLAGS=-p=4"
python3 ~/go/src/goframework.io/tools/app-workspace.py .     # writes go.work (not in git)
$C $E go build ./...
for os in linux windows darwin freebsd; do $C $E GOOS=$os go vet ./...; done
$C $E go generate ./musicapi        # after changing the API: the Go stub, its method table, and the WUI's TypeScript stub
cmd/media-player-music-wui/web/build.sh   # after changing the page: tsc + esbuild into cmd/media-player-music-wui/internal/frontend
# -race one package at a time (several big ones at once go past capped.sh's 10G):
for p in $($E go list ./... | grep -v /tools/); do $C $E go test -race -count=1 -timeout 60m $p; done
```

internal/shell takes about 25 minutes under -race (the popup matrix:
352 subtests), internal/players/winamp 10, internal/skins 8; the rest
seconds to a minute. A script that runs every package one at a time is
in the 2026-11-13 session's scratchpad (racesuite.sh); the loop above is
the same. No test reaches a sound device: the engine plays into
audiotest's sound system; the daemon in e2e has `MEDIA_PLAYER_MUSICD_SOUND=memory`;
the shell's and the TUI's tests run the daemon's service in their own
process on a fake engine.

- The native opt-in tests (`MUSIC_PLAYER_NATIVE_XDND_TEST`,
  `MEDIA_PLAYER_NATIVE_WINDOW_TEST`) need a real X11 display. Do not run
  them without the owner.
- `tools/probes/` holds standalone probe programs, ignore-tagged.

## Where the code is

| Path | What |
| --- | --- |
| `musicapi` | the API: `Player`, its states, the generated Go stub with its method table and drift test (`go generate ./musicapi`, which also writes the WUI's `musicapi.ts`) |
| `cmd/media-player-musicd` | the daemon's main: config (media-player-musicd.json, MEDIA_PLAYER_MUSICD_*; `sound: memory` for tests), one daemon a user (vfs-source-disk's Single), session.json through vfs-statefile, library.index, clienthost |
| `internal/service` | the daemon's player: `musicapi.Player` over internal/model's transport and queue, driving the engine (engine.go: loads, seeks, generations, the end of a track, CUE parts), files and playlists made tracks (files.go), what is kept (saved.go) |
| `internal/library` | the library: folders read (only changed files' tags again) and watched (vfs-watch), browse, search (data-search's Parse; case and accents folded), the gzip index |
| `internal/playlist` | M3U/M3U8 (#EXTINF), PLS, CUE read (Windows-1252 when not UTF-8, codecs-text-charsets); M3U8 and PLS written; fuzzed |
| `internal/tags` | a file's tags and length from every container (codecs-container-spec's Open: ID3, APE, Vorbis comments, MP4, Matroska); `tags/cover` the ID3 front cover for the desktop UI |
| `internal/link` | the Go renderers' connection: link.Start with the four states, the token for ws://, `Ensure` and `Daemon` (R115's ensure: starting media-player-musicd, with Retry) |
| `internal/model` | the model with no UI: Track, Playlist, Transport, commands, Equalizer; `internal/players` aliases its names for the faces |
| `internal/playback` | the one engine, `Builtin`, on audio-playback, with ReplayGain; `Engine` is its contract, which tests fake |
| `internal/players` | the faces' shared pieces (host, queue view, settings, skins list) and one package per face; `playertest` the test helpers |
| `internal/shell` | the desktop UI: face switcher, the model kept in step with the daemon (playback.go: ShowState/ShowQueue/ShowNow, syncPlayback), files and drops, Ctrl+S save playlist, settings window, tray |
| `internal/tui`, `cmd/media-player-music-tui` | the terminal UI |
| `cmd/media-player-music-wui` | the web UI: main, internal/server, internal/frontend (the built page), web/ (TypeScript sources, build.sh) |
| `internal/skins` | skin registration (embeds only the redrawn packs), plans (every skin drawn in Go; geometry_*.go tables), packs, fandomart (Fandom's own raster), sources (the originals and, in sources/oldplans, the old plans that cut them; built into nothing; await the owner's history rewrite) |
| `e2e` | the daemon as a program (apptest), two renderers |
| `internal/testenv` | tests kept off the owner's environment (R102) |

`media-player-music` (a binary at the root) and `out/` are build and shot
output, ignored by git.

## Where it stands

See "The daemon split, 2026-11-13" at the end for the latest commits. Older
history is in the dated sections below and in git.

## goframework modules

**Used now:** app-spec (protocol, kit/clienthost, transport.TokenFile),
app-as-service-w-plugins (the daemon), app-as-service (the WUI),
app-plugin (client, stub, apptest), app-client-typescript (the page),
networking-http-server (Launch, Page, Hosts, Origins, SecurityHeaders,
Forward), config-json, config-source-file, logging-config, cliflags (the
TUI), terminal-ui and -widgets, vfs, vfs-source-disk (UserPlaces, Single,
WriteFile, Write), vfs-statefile, vfs-watch, data-search (Parse only),
codecs-text-charsets, audio-playback (with `/all`), audio-dsp,
system-audio-spec (audiotest: tests and the daemon's `sound: memory`),
codecs-container-spec, -id3, -apetag, codecs-audio-ape (R101);
app-plugin's client/ensure and ensure/userservice (R115); desktop-ui and its widgets, skins,
skingen, rack, text-complex (tags), platform-all, themes-breeze/-luna/-win95,
graphics-paint2d, -gpu-gles; system-dbus in tests and probes.

**Could use, not yet:** media-session (F5: MPRIS, media keys; not built),
a pure-Go store with
full-text search (F9; the library keeps its own gzip index meanwhile),
vfs-trash (F3) for deleting from the library, logging in the UIs.

## Next steps, in order

1. **The owner's questions** (see "The daemon split" and the last
   section): what closing the desktop UI does to the music; a first run's
   folder; F9; whether the desktop UI should manage the library's folders.
2. A real first run on a desktop (the owner's: no UI on the host): the
   GUI, TUI and WUI starting media-player-musicd through ensure, the user
   service path; a browser run of the WUI's Retry.
3. MPRIS and media keys (F5), gapless (F14), WMA (R101: stopped on its
   sources, the owner's).
4. The adopted gaps: R114, R121, R122, R123 when the framework answers.
5. Re-shoot the pictures (the library window too); a user guide; start
   time and RSS; VMs on the six systems.

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
  for five rows. (Since 2026-11-12 the external engines are gone: the
  built-in engine is the only one.) **For the owner, then:** new installs play in-process; the
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

## The owner's decisions, 2026-11-12: one engine, ReplayGain
"There is no backward compatibility, this is a new app, previously it was a
test." What changed:
- **The external engines are gone**: mpv, VLC, MPlayer and mpg123, with
  everything of theirs: internal/playback's mpv*, vlc*, mplayer*, mpg123*,
  process*, spectrum* (mpv's FIFO analyser), ipc_*, command_*, catalog*
  (discovery) and their tests; the `Engine` interface lost `Capabilities`
  and `EngineOptions`; the update broker moved to broker.go. The shell's
  engine switching (`SetBackend`, `detectBackends`, `openBackend`,
  `backendError`, `backendSwitching`, `backendTransition`, `engineEpoch`,
  `lifecycleDone`), the "choose an installed backend" paths, and
  `Options.Backend` (the legacy mpv hook), `DetectBackends`, `OpenBackend`,
  `AudioBackend`; the `-audio-backend` flag; the Settings window's engine
  rows (players/backend_settings.go and its three tests); every face's
  capability gating (`Host.AudioBackend`, `Model.BackendLimited`,
  `BackendEqualizer/Balance/Spectrum`, `EqualizerAvailable`,
  `BalanceAvailable`, `SpectrumAvailable`, `FeatureTip`) and the tests of it;
  the menu labels "Audio backend…" (now "Playback…"/"Playback settings");
  tools/probes/backend_settings.go; go-winio (mpv's Windows pipe) and the
  indirect x/sys out of go.mod and go.sum (edited by hand, no tidy).
- **Old compatibility code removed**: `NormalizeAudioBackend` and the saved
  `audio_backend` field (an old value is ignored: unknown JSON); the
  settings migration from the per-face preference (`face`, `skins`) and
  the old skin names (`minim-classic`, `minim-silver`, `dark`,
  `breeze-night`); reading the settings from uitoolkit's folder until the
  first save (`style.ConfigSource`; now `SettingsPath()` only). Kept:
  `Themed` as an alias of the default UI (the menus pass it at run time),
  the ignore-tagged uitoolkit-era probes in tools/probes (developer probes,
  not the app's code paths).
- **Formats it does not play**: `playback.Formats` (in Settings),
  `Playable` (a folder scan takes only those), `UnsupportedFormat` (WMA,
  APE, Musepack, WavPack, TTA, AC-3, DTS, AMR, DSD, M4P, MIDI, trackers,
  Shorten, OptimFROG, TAK, CAF): an explicit file of one is refused with
  "song.wma: WMA files are not supported. The player plays MP3, AAC …"; a
  folder leaves them out and says how many and which formats; a file
  failing as ErrUnknownFormat or an unsupported codec gets the same
  message from the engine. Ledger R101 asks the framework for WMA and
  Monkey's Audio decoders (verdict: they pass the boundary test).
- **ReplayGain**: `players.Settings.ReplayGain` (`mode` "", "track",
  "album"; `preamp_db` -15..+15), saved; Settings window: Off / Track gain /
  Album gain radios and a Preamp slider (520 × 480, min 460 × 460); the
  shell sends `OpReplayGain` at start and on change; the engine makes each
  load's player with `replaygain.Settings` (audio-playback's
  `Settings.ReplayGain`, its Guard limiting past full scale). The tags'
  peak always holds the gain (no switch). A track without the chosen gain
  takes the other.
  - **R97** (codecs-container-id3's `Tags` gives no TXXX REPLAYGAIN_* and
    no RVA2) and **R98** (no APE tag reader in goframework): workaround
    `internal/tags/replaygain.go` (`ReplayGain(path)`: APE items, then RVA2,
    then TXXX, each replacing the last, parsed by audio-dsp/replaygain's
    `Parse`), used by `Builtin.replayGain` (internal/playback/builtin.go,
    tagged R97, R98): when it finds a gain, the player gets Mode Off with
    the computed gain as its Fallback; else audio-playback reads the
    stream's tags itself (Vorbis comments, MP4 freeform items).
  - **R100** (audio-playback takes ReplayGain only at New): no
    workaround; a change is heard from the next track (said in Settings).
- Tests: internal/playback/replaygain_test.go (MP3s tagged by the test with
  ID3v2.3/2.4 TXXX, RVA2, APEv2 + ID3v1, TXXX and APE together; a FLAC tone
  written by codecs-audio-flac with Vorbis comments; each mode, a preamp,
  +20 dB held by the peak at full scale; a WMA file's message), played into
  audiotest in synctest and compared by energy to 0.3 dB;
  internal/tags/replaygain_test.go (the reader, broken APE sizes);
  internal/players/settings_test.go and settings_panel_test.go (old fields
  ignored, ReplayGain saved and normalised; the panel by keyboard and
  accessibility at 1× and 1.75×, preferred and minimum size); the shell's
  tests moved from the mpv protocol fakes to `fakeEngine`
  (playback_test.go, spectrum, tray, files, session and the faces' shell
  tests), metadata_test.go (mpv's audio-params recovery) and
  playback_integration_test.go (real mpv) and the engine-switch tests
  deleted; new: unsupported files, ReplayGain reaching the engine and
  saved, and builtin_test.go (a two-track tagged queue through the shell
  with the real engine on audiotest, played to its end).
- Size, stripped, linux/amd64, CGO_ENABLED=0: 36.55 to 33.17 MB
  (36,544,672 to 33,173,664 bytes).
- Checked: vet on linux, windows, darwin, freebsd, with `-tags
  font_complex` and `font_emoji`, and the 53 probes; `go test -race` of the
  root, internal/appicon, internal/playback, internal/tags/...,
  internal/players and all twelve faces (winamp included, about 13
  minutes), internal/skins/...: pass. internal/shell: everything passes
  but four tests that fail at HEAD before this change too (checked in a
  worktree of 21c45ef): TestSnappedWindowsFollowWhilePlaybackIsPausedOrStopped,
  TestFandomSessionKeepsItsMainAndPlaylistSeparateFromEveryFamily,
  TestHalfLife2SessionKeepsAllWindowsSeparateFromOtherFamilies and
  TestSessionRestoresQueueSelectionSkinAndPositionWithoutAutoplay (a
  restored or followed panel window reads 0,0 headless: the rack.Group
  move, never run before because the package started mpv). goframework's
  footprint.py --check passes. Not run: the player, a real sound device.
- Next: the four shell placement tests (with rack.Group); R97, R98, R100,
  R101 when the framework answers (drop `internal/tags/replaygain.go`'s
  readers then); the owner on R101 (WMA, APE decoders).

## Shell placement tests fixed; tests kept off the owner's environment, 2026-11-12 (R102)
- **The four failing internal/shell tests** (TestSnappedWindowsFollowWhilePlaybackIsPausedOrStopped,
  TestFandomSessionKeepsItsMainAndPlaylistSeparateFromEveryFamily,
  TestHalfLife2SessionKeepsAllWindowsSeparateFromOtherFamilies,
  TestSessionRestoresQueueSelectionSkinAndPositionWithoutAutoplay) were
  stale, not the app: they moved windows with `w.Move`, a program's own
  request, which the headless desktop reports no move for (by design), so
  the racked faces' rack.Group never followed it and saved the panes where
  they were (0,0); and they moved them before the group had settled, when
  it puts a window back where its bond or saved place says. Checked first
  that the app is right: a desktop move after the settle is saved and
  restored at once (420,210 read back through a closed and reopened shell).
  Fix: `desktopMove` (docking_test.go: wait for the face to settle, then
  `Offscreen.SimulateDesktopMove` and playertest.Follow) and `awaitSettled`;
  the racked faces (winamp, fancy, fandom, halflife2) got
  `Settled() bool` (their group's), which the shell's tests ask through an
  interface; a playlist's SetSize is followed (two turns) before it is
  hidden; a reopened shell is settled before its positions are read; the
  docking test waits for the pulled-away playlist to come loose before the
  player moves (the group follows a batch of moves at once). Three more
  shell tests had the same `w.Move` on a racked face and passed only when
  the move landed inside the settle (whose adoption took it): they failed
  in a full run of the package (both with an empty and an odd home, so
  not the isolation: timing), and now use `desktopMove` too
  (TestSessionRetainsSeparateDefaultAndSkinArrangements,
  TestWMPSessionKeepsEachPlacementSeparateFromWinampAndDefault,
  TestImportedSkinsPreserveThePlayingQueueAndWindowGroup). The tests now
  catch a real restore bug: with RestoreGroup's `g.Place` taken out, three
  of them fail.
- Not changed, worth a look: TestFancySessionRestoresCompactAndAllPanelPlacementIndependently
  passes but proves little: it also uses `w.Move`, and compares captured
  placements before and after, which never held the moves.
- **Tests and the owner's environment**: nothing redirected HOME, so a test
  run without the handover's XDG_* variables read the owner's look.json,
  themes, skins and fonts (~/.config/goframework-desktop-ui, uitoolkit's)
  and the GOFRAMEWORK_DESKTOP_UI_* look variables (the video player's tests
  were seen drawing in "hangar"). `internal/testenv` (new): `Main(m)` /
  `Isolate(dir)`: HOME, XDG_CONFIG/CACHE/DATA/STATE_HOME, XDG_RUNTIME_DIR
  (and USERPROFILE, APPDATA, LOCALAPPDATA on Windows) in a temp folder
  removed after, XDG_CONFIG_DIRS and desktop-ui's look variables unset
  (`lookEnv`); a display given on purpose keeps its XAUTHORITY. Called by
  playertest.Main (every face and the shell), amarok's TestMain, and new
  TestMains in internal/players, players/vlc, internal/skins,
  skins/plans. Its own test checks every desktop-ui folder is inside and
  the look is the default. internal/playback, internal/tags,
  internal/appicon and skins/sources do not build on desktop-ui and were
  left alone (internal/playback was another session's work in flight).
  **R102** (ledger): desktop-ui has no one call for this; testenv's list is
  kept by hand (tagged R102).
- Proof: `go test -count=1 -v` of internal/testenv, internal/players/...,
  internal/skins/..., internal/shell with HOME at an empty folder and at
  one whose look.json (both folders) names hangar with odd corners, icons,
  decorations, renderer and fonts, and every look variable set (theme
  hangar, colour scheme dark, direction rtl, scale 2...): identical
  PASS/FAIL lists, "hangar" nowhere in the output.
- Checked: vet on linux, windows, darwin, freebsd; `go test -race
  -count=5` of the seven placement tests above: pass; `go test -race` of
  internal/testenv, internal/players and all twelve faces,
  internal/skins, internal/skins/plans: pass (one package at a time: four
  at once went past capped.sh's 10G and were killed); internal/shell:
  everything passes but TestSinglePreferenceAndScreenshotIsolation, which
  a4a7605 (ReplayGain's default track gain, another session's) broke: the
  settings file of a new install now holds `"replaygain": {"mode":
  "track"}` beside the skin, and the test wants the skin alone. Left to
  that work (is track, the default, meant to be written?). goframework's
  footprint.py --check passes (no program changed).

## the cheap sweep, 2026-11-12 (release-gaps.md)
- TestSinglePreferenceAndScreenshotIsolation passes again. The file's convention decides: a default is never written (every field is omitempty/omitzero; ReplayGain's doc says its zero value is the default, and TestSettingsReplayGainDefaultsToTrack wants no "replaygain" for none chosen). LoadSettings normalised the mode to "track", and the shell's next Save wrote it back. Fixed in the code, not the test: `Settings.Save` writes track gain as no mode (`ReplayGain.saved`); off, album and a preamp are still written. TestSettingsReplayGainDefaultsToTrack now also saves what it loaded and finds no "replaygain".
- Settings are written by vfs-source-disk's WriteFile (temporary file, synced, renamed, folder synced, 0600) in place of a temp+rename with no fsync. Not on vfs-statefile's debounced Saver: the shell saves at its own moments, and moving it is the daemon split's work.
- Version 0.0.1: `players.Version` (a var, ldflags-ready), `-version`, and the Fandom skin's About line. go.mod untouched (x/image stays until the probes drop it).
- This file's lines on uitoolkit ("the uncommitted work"), `HeldSize` (gone with R45) and the external engines corrected.
- Checked: vet on linux, windows, darwin, freebsd; `go test -race` of internal/players, internal/players/fandom and internal/shell (370 s), one at a time, all pass but one.
- **Not this change's: TestSkinMenuOffersTheSameChoicesOnEveryPopupBackend hangs** (killed at 4 and 40 minutes) at HEAD f9e1f15 too, with this change stashed. Its goroutine is runnable in graphics-paint2d's raster (RectCoverage under CPUDevice.fillAxisAlignedRect, from openAtScale at shell_test.go:36, the itunes subtest at 1x); paint2d's fills changed on 2026-11-11 evening (7ce99e6 "a run at a time", f179ea8 "an upright rectangle's..."). For the framework session to look at; the rest of internal/shell passes with `-skip` of it.

## The popup test's "hang", 2026-11-13 (goframework session)
- TestSkinMenuOffersTheSameChoicesOnEveryPopupBackend does not hang: it is 352 subtests (22 skins x 2 starting skins x 4 popup modes x 2 scales), each a whole shell opened and painted, and passes alone under -race in 1195 s (1073 s after graphics-paint2d 7437f2b), past go test's default 10-minute timeout; beside other packages it went past 40 minutes. The goroutine dump only ever caught it painting. paint2d's fills (7ce99e6, f179ea8) are faster than before them, not the cause. Hence `-timeout 60m` on internal/shell above. Whether to trim the matrix (1.75x for a few skins only) is the app's call; the test is not wrong.

## The daemon split, 2026-11-13: daemon, library, playlists, TUI, WUI
The brief (overnight 2026-11-12→13): the daemon on app-spec, the GUI its
client with every look working, a library, playlists, a TUI and a WUI, e2e.
The skin art is not part of it (the owner: redrawn and renamed later).

- **4ad6b37, the daemon** (step 1): see "What it is" and "Where the code
  is" above. The shell's model is a copy of the daemon's: `ShowState`
  (a state older than the one shown, by `Seq`, is let go; a state naming a
  newer queue fetches it), `ShowQueue`, `ShowNow` (positions and bands of an
  older state let go); a face's changes (a track chosen, a seek, the
  levels, the equaliser) are read whole, then sent (`syncPlayback`), the
  answer shown; commands go as the daemon's (`Command`). Calls are made on
  the UI thread with a 5 s bound (10 minutes for an Add, made on the file
  loader's goroutine): fine on the socket; over a slow ws:// a drag of the
  volume would feel it. Closing the desktop UI leaves the daemon playing
  (mpd-like; a question for the owner, below). The GUI's settings file
  keeps only the skin, the panels and the windows (no backward
  compatibility: the old queue and ReplayGain fields are ignored).
- **ea7e49b, the library** (step 2): internal/library, internal/tags reading every
  container; the API's Library, AddFolder, RemoveFolder, Rescan, Browse
  (with `Unknown` for the tracks whose tags do not say), Search; the
  `player.library` state, told at most every 250 ms while a scan runs. The
  index is the app's own (gzip JSON, about 40 bytes a track): goframework
  has no pure-Go store (F9, the owner's question) and data-search is SQL's
  (only its Parse is used). R114 (Vorbis comments' names) found, worked
  around in tags.go.
- **ea7e49b, playlists** (step 3): internal/playlist; Add opens M3U/M3U8/PLS/CUE in
  every UI (the GUI's file chooser, drops and command line; the TUI's
  `:add`; the WUI's paths); CUE tracks are parts of a file (`Start`/`End`
  in the model and the API: the engine is given the file's times, the
  transport keeps the track's; a part's end plays the next). Saving is the
  shared model's: `SavePlaylist` (M3U8 or PLS by the extension), Ctrl+S in
  every look, Fancy's Save button through `Host.SavePlaylist` (its own M3U
  writer gone), Fancy's Clear through `Host.ClearQueue` (the daemon's).
- **f0c210d, the TUI** (step 4) and **f23dc8a, the WUI** (step 5): see the table above;
  R121, R122 (terminal-ui-widgets) and R123 (the TypeScript stub's
  constants) found and tagged. The WUI's daemon starter (`/daemon`,
  `Unreachable`, R115 in the calculator) is not adopted: it belongs to F1.
- **Sizes**, stripped (`-trimpath -ldflags=-s -w`), linux/amd64, CGO_ENABLED=0:
  media-player-music (GUI) 33.17 → 35.03 MB (the app-spec client and the
  link; the engine and the decoders left it for the daemon), media-player-musicd
  12.94 MB, media-player-music-tui 9.07 MB, media-player-music-wui 8.59 MB.
- **Checked**: vet on linux, windows, darwin, freebsd, openbsd and netbsd;
  `go test -race` of every package, one at a time, on the tree of f23dc8a
  (internal/shell 1604 s, all pass), and on 4ad6b37's alone before it; internal/playlist fuzzed 60 s (1.7 M inputs). Not run: any
  program for real, a sound device, a browser, the VMs.
- **For the owner** (not decided here, overnight rule):
  1. Closing the desktop UI (Quit, Ctrl+Q) leaves the music playing in the
     daemon. Options: (a) keep playing, as mpd and cmus's daemon do (now);
     (b) stop when the last UI quits; (c) a setting. Recommended: (c), off
     by default as (a), once F1 starts the daemon on demand.
  2. The first run's library folder: the user's music folder
     (XDG_MUSIC_DIR, ~/Music) is added when there is no library yet. Keep,
     or ask in a first-run screen?
  3. F9's shape (a pure-Go store with full-text search) for a library past
     some 100 000 tracks: the in-memory index serves until then.

## F1, APE and the GUI's library window, 2026-11-13 (overnight)
- **ad01b8e, F1 adopted (ledger R115, app-plugin client/ensure)**: link's
  `Options.Ensure` (bool) and `Options.Daemon`: on the default address
  each UI calls ensure.Daemon before the first dial (the user's service
  through userservice.Starter, else media-player-musicd next to the
  program or on PATH, detached, its output in UserPlaces("media-player-music").Logs'
  media-player-musicd.log; ensure's default lock, by the daemon's name, so
  the GUI, TUI and WUI share one), and dials once it runs.
  `link.Daemon{Starting, Err, Retry}`; `musicapi.DaemonName`. GUI: every
  face shows "Starting media-player-musicd…" or why it could not be where
  it shows errors (Model.Error), and internal/shell/daemon.go opens a small
  window of the default look over the player with the reason, Retry
  (focused) and Close (Escape). TUI: the line under the views, Ctrl+R
  retries. WUI (cmd/media-player-music-wui/daemon.go): the calculator's
  starter (at start, again when a page's connection cannot reach the
  daemon, at most every 30 s), `server.Config.Starter`, GET /daemon for
  the page, POST /daemon (its origin, behind the session) for the page's
  Retry button beside the status chip. The page bundle was rebuilt by
  web/build.sh (the old source rebuilt the committed bundle byte for byte
  first). With `-daemon`/`--daemon`/a configured address nothing is started.
- **9a597e0, APE (R101)**: codecs-audio-ape imported in internal/playback
  (beside audio-playback/all) and internal/tags; ".ape" out of
  `unsupported`, into `Playable` and `Formats`; the settings panel and the
  README no longer say APE does not play; the README's Licence section
  reproduces Monkey's Audio's 3-clause BSD notice (the module's
  license-monkeys-audio). go.mod unchanged: as every suite app's, it names
  no goframework module (go.work replaces each at v0.0.6). testdata: the
  module's stereo-c1000..c5000.ape (testdata/readme).
- **511aa6b, the library window** (internal/shell/library.go): "Library…" (Ctrl+B)
  in every face's menu (Host.ContextMenu, which every face's menu key and
  appearance menu opens, and the faces' own playlist/File/Tools menus
  through `Host.LibraryItem`/`WithLibrary`); only with a daemon (the
  screenshot fixture has none, so the popup matrix test's rows are as
  before). A window of the default look: Artists | Albums | Genres |
  Years (Ctrl+1..4), a search field (Ctrl+F or /; the daemon's Search),
  three lists (groups, the group's albums, the tracks), Play (the tracks
  listed in the queue's place, from the one chosen) and Add to queue
  (Insert), a status line from the daemon's `player.library` state (now
  passed to the GUI: `Shell.ShowLibrary`), listed again when the library
  changes. Enter goes on to the next list or plays; Escape clears the
  search, then closes. Calls on the UI thread with the shell's 5 s bound
  (an Add 30 s). It does not manage the library's folders (the TUI and WUI
  do): a question for the owner.
- Tests (nothing starts a real daemon: ensure replaced): internal/link
  TestEnsureStartsTheDaemonThenDials, TestNoEnsureStartsNothing,
  TestEnsureOptions; internal/shell TestTheWindowSaysTheDaemonIsStartingOrWhyNot,
  TestTheLibraryWindowBrowsesSearchesAndPlaysByKeys,
  TestTheLibraryWindowsButtonsAndChanges, TestEveryFacesMenuOpensTheLibrary
  (all 22 looks); internal/tui TestTheScreenSaysTheDaemonIsStartingOrWhyNot;
  cmd/media-player-music-tui TestTheDaemonStartingTellsTheView; wui
  TestStarter, server TestTheDaemonAway, TestNoStarter; internal/playback
  TestBuiltinPlaysMonkeysAudio (each level into audiotest),
  TestBuiltinSeeksMonkeysAudioAndReadsItsTags; internal/tags
  TestMonkeysAudioTagsAndLength.
- **Checked**: vet on linux, windows, darwin, freebsd; `go test -race`
  of every package, one at a time, on 511aa6b's tree: all pass
  (internal/shell 1568 s, internal/players/winamp 567 s, internal/skins
  529 s); goframework's footprint.py --check passes. The library and
  daemon windows were looked at as headless PNGs (scratchpad), not on a
  display. Not run: any program for real, a sound device, a browser, the
  VMs, ensure against a real service manager.
- **For the owner** (not decided, overnight rule): should the desktop UI's
  library window also add and remove the library's folders (a folder
  chooser), as the TUI and WUI settings do? Recommended: yes, a "Folders…"
  button opening the file chooser in folder mode.

## The skins redrawn and renamed, 2026-11-13 (overnight)
release-plan.md "Skins (music and video players): redrawn and renamed".
Every skin's art is now the app's own, drawn in Go; the names are neutral.
The owner: no backward compatibility, so there is no rename map (a saved
old ID falls back to Default; old per-skin window placements are ignored).

| old | new | label | how |
|---|---|---|---|
| winamp | slate | Slate | ours (winamp.go), renamed |
| winamp-silver | silver | Silver | ours (winamp_silver.go), renamed |
| winamp-classified | dossier | Dossier | redrawn (panelart.go) |
| dosamp | textmode | Text Mode | redrawn (panelart.go) |
| windows-98 | bevel | Bevel | redrawn (panelart.go) |
| winamp-amber | amber | Amber | redrawn (panelart.go) |
| calculator | tally | Tally | redrawn (panelart.go) |
| sony-cdx-mp3 | headunit | Head Unit | redrawn (panelart.go) |
| ti-82 | graphing | Graphing | redrawn (panelart.go) |
| radar-amp | radar | Radar | redrawn (panelart.go) |
| quake-3-arena | forge | Forge | redrawn (panelart.go) |
| headspace | wingspan | Wingspan | redrawn (wingspan.go) |
| xbox | pod | Pod | redrawn (pod.go) |
| half-life-2 | gauge | Gauge | redrawn (gauge.go) |
| fandom | fandom | Fandom (was "Win Media Player - Fandom") | ours, relabelled; its raster copied to internal/skins/fandomart |
| itunes | aluminium | Aluminium | redrawn (aluminium.go) |
| series-9 | cobalt | Cobalt | redrawn (cobalt.go) |
| vlc | utility | Utility | redrawn (utility.go) |
| realplayer | console | Console | redrawn (console.go) |
| amarok | sidebar | Sidebar | redrawn (sidebar.go) |
| fancy | fancy | Fancy | ours, unchanged |

- **How**: each redrawn skin is a geometry table, plans/geometry_<new>.go,
  generated once from the old pack's skin.json by a scratchpad script
  (sprite names, sizes, slices, sampling, tint; layouts and slots; window
  frames and variants; parts; text-role names; numbers and names only, no
  pixels) and kept by hand from now on, plus a painter: redraw.go
  (build(), the slot helpers, the shared symbols), generic.go (a painter
  by sprite name), panelart.go (the nine compact-panel themes on one
  painter), and one file per other skin. Window silhouettes, curved
  sliders' `.map`/`.shape` sprites and keys' hit alpha are drawn from our
  own vector shapes. The old and new manifests were compared: every
  redrawn pack keeps its old one's sprites (sizes, slices, sampling),
  layouts, window, parts, text roles, metrics (scratchpad compare.py, all
  "same geometry"). Text colours, fonts (now Liberation/DejaVu/Nimbus
  names) and focus colours are the new art's.
- **Renames in code**: the skin IDs and labels (internal/players/skins.go;
  labels are now the packs' names, no "WinAmp -"/"Win Media Player -"
  prefixes; FormatSkinLabel only tidies white space), the layout names
  (winamp.* -> panel.*, xbox.* -> pod.*, ... in internal/skins/*/layout.go,
  panel.go and the faces' player.go), the faces' accessible names and
  messages that named a product ("Xbox music player" -> "Pod music player",
  "SRS WOW Effects" -> "Surround Effects", "TruBass" -> "Bass Boost", ...).
  The faces' Go packages and constants keep their old Go names
  (players.XboxSkin = "pod").
- **Moved, not deleted** (DELETE NOTHING): the plans that cut the old art
  (headspace, xbox, halflife2, itunes, series9, vlc, realplayer, amarok,
  winamp_classified, winamp_imported, import_bmp, their tests, and the
  skins package's tests of the old art) are in
  internal/skins/sources/oldplans/ with `//go:build ignore`. The old-named
  pack folders stay in internal/skins/packs/ but are not embedded (skins.go
  embeds the 21 new packs by name). The old screenshots stay in
  docs/screenshots; the new ones are beside them under the new names.
  All of that, and internal/skins/sources, goes with the owner's history
  rewrite.
- **Tests** (new): plans TestGeneratorReadsNoOriginals (the generator and
  every app package it imports: none is or names internal/skins/sources,
  none imports os/io/fs/path/filepath but cmd/generate), 
  TestNoProductNamesInAnySkin (every manifest, name and label, and the
  geometry tables), TestEveryRedrawnSpriteIsDrawn, utility_test.go (the
  pixel icons); skins TestEveryPackDrawsItsLayouts (every pack registers,
  every layout's face and every slot's art draws), and the updated
  TestBundledPacksLoadAndLint / TestGeneratedPacksMatchBundle (packs equal
  the generator's output, byte for byte). Face tests that read the
  originals were rewritten on the new art (players/xbox and
  players/halflife2: the curved sliders' maps; players/vlc: crisp icons at
  1x, 1.75x, 2x).
- `go run ./internal/skins/cmd/generate -out internal/skins/packs [-only a,b]`
  (-only is new) regenerates the packs.
- Rewritten panel tests (internal/players/winamp): TestDossierUsesItsPlaylistAndVisualizerColors,
  TestBevelPlaylistReadoutsUseThePacksInk, TestGraphingPlaylistReadoutsUseThePacksInk
  (the painted selection, analyser ramp and footer readouts follow the
  pack's text roles, read from the pack instead of the old fan skins' hex).
- **Checked**: gofmt; vet on linux, windows, darwin, freebsd; `go test
  -race` of every package one at a time (internal/shell 1765 s with
  -timeout 60m, internal/players/winamp 606 s, internal/skins 193 s): all
  pass (the three winamp tests above and shell's
  TestSinglePreferenceAndScreenshotIsolation, which named "winamp-silver",
  were fixed after the full run and rerun on their own); goframework's
  footprint.py --check passes. The old/new manifests compared (same
  geometry, all 21). Headless `-shot` screenshots of every skin, old and
  new, side by side, in the session's scratchpad (music-skins/side-by-side);
  the new ones are also in docs/screenshots. The GUI binary went from 44.8
  to 38.0 MB (unstripped, linux/amd64). Not run: any program on a display,
  the VMs.
- **For the owner**: the art review (side-by-side screenshots); then the
  history rewrite takes internal/skins/sources (with oldplans), the
  old-named folders in internal/skins/packs and the old screenshots in
  docs/screenshots (the 21 old names: winamp-strip.png, xbox-main.png, ...;
  fandom-*.png and fancy-*.png are the current art). The faces' Go
  package names (internal/players/xbox, ...) and constants (XboxSkin) still
  carry the old names: renaming them is a mechanical follow-up, left so
  this change stays reviewable.
