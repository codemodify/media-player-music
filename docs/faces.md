# The interface and skins

The player has one default interface and twenty optional skins. The default
uses ordinary toolkit controls in a single window. Eleven Winamp skins use
compact panels; Headspace, Xbox and Half Life 2 reproduce separate Windows
Media Player arrangements. Fandom adapts a supplied silver-player reference,
and iTunes recreates the brushed-metal Mac OS X player with a MiniPlayer.
Series 9 adapts Microsoft's original blue-and-silver Windows Media Player skin,
with embedded drawers and a compact player.
Classic VLC, RealPlayer 8 and Amarok 1.4 reproduce the older desktop
arrangements in separate resizable window families.
This document explains their layout, the app/toolkit
boundary, and the v0.23.4 audit.

## Playback and screenshot fixtures

Interactive playback uses an external **mpv**, **VLC**, **MPlayer** or
**mpg123** executable; none is bundled. Detection checks `PATH` first,
then common Homebrew and app-bundle locations on macOS and application
installation directories on Windows. mpv uses Unix socket IPC on Linux/macOS
and a named pipe on Windows. VLC uses an authenticated local HTTP interface;
MPlayer and mpg123 use their remote-control protocols.
The app owns this playback integration; the UI toolkit does not decode
music or drive a playback process.

Default's gear button, player context menus and Ctrl+comma
open the ordinary themed Settings window. Installation is shown by vector
green checks and red crosses; the selected radio is distinct. Missing
executables cannot be chosen, and Refresh rescans discovery. Finding a
program does not guarantee its control interface starts successfully:
startup errors appear separately, and the selected backend can be chosen
again to retry. Settings opens at 600 × 470 logical pixels, with a minimum of
560 × 440; its four rows remain usable at 1× and 1.75×. Escape closes the dialog.
The mpg123 row is explicitly labelled MP3/MPEG only. Unsupported containers
are rejected with a short explanation, and track failures also appear in
Settings and the application log.

The `audio_backend` session field preserves the selected program, even if
it is missing on a later launch. The initial default is mpv. A successful
backend change keeps the queue, track, position, paused/playing state and
volume; a failed replacement leaves the existing engine in place.

mpv supports the player's balance and ten-band/preamp EQ, plus the live
visualizer on Linux/macOS. Windows mpv playback and filters use the pipe
adapter, but its metadata-FIFO visualizer is unavailable. VLC, MPlayer and
mpg123 currently provide basic transport, seeking and volume only. Their
unsupported EQ and balance controls are disabled, with explanations in
Settings/tooltips; model values survive so returning to mpv restores them.
VLC 3's reported duration and clock are coarse, and seeking is coarse too.
mpg123 accepts MPEG audio (MP1/MP2/MP3), while the other programs' format
support depends on their installed builds.
VLC retains its native volume scale, which follows the same cubic curve as
mpv. A separate latest-value lane prevents rapid slider motion from
accumulating obsolete HTTP commands.

The first startup begins with an empty playlist. Later launches restore the
saved queue, selected track and playback offset, stopped until Play.
Positional music-file paths replace the queue and start the first file.
Ctrl+O and Eject choose a replacement playlist; Playlist → Add appends files. The model
tracks the real file, position, queue, volume, balance, shuffle/repeat and
equaliser values. Seeking and volume affect real playback. With mpv,
balance and the ten-band/preamp equaliser use its FFmpeg audio filters.
On supported platforms, every presentation displays 48 frequency bands
measured from that processed audio.

The analyzer splits a passive branch inside the same mpv process and
measures RMS levels through logarithmically spaced bandpass filters from
32 Hz to 16 kHz. The original audio branch reaches the output unchanged.
Frame times and playback generations reject stale samples after a seek or
track change; a bounded 20 Hz stream keeps visual updates from delaying
playback commands. Paused, stopped or stalled levels decay to zero. The
analyzer uses mpv's existing FFmpeg filters and a private metadata FIFO,
with no separate decoder process or FFmpeg executable.

The whole playlist accepts file-manager drops, including its empty list,
heading and controls. For panel skins, an app-owned drop ancestor also
routes the toolkit caption's title band and controls to that same action;
the original caption keeps its dragging and close behavior. Drops append
local files or folders using the Copy
action, so the source files stay in place. Folder expansion runs off the UI
thread and adds recognized audio files and media containers, including MP4,
M4V, WebM and MKV, recursively in deterministic sorted
order. Explicit files use the selected backend's format support. Requests commit in their
arrival order. Backends play their audio with video disabled. An addition to an empty queue starts the first track; an
addition to an existing queue leaves its current playback intact.

Headless screenshots and presentation tests use invented tracks and a
synthetic clock-driven analyser. They launch no audio backend and play no sound. The
screenshots in the README show those deterministic fixtures.

Winamp and Winamp Silver generate their artwork, bitmap alphabet and clock
digits from app-owned drawing code. The nine imported skins adapt original
Winamp bitmap artwork into the same app-defined skin and layout format.
Their unchanged archives, credits and notices live in
[skin sources](../internal/skins/sources/README.md). The menu prefixes Winamp
skin names with `WinAmp -`, removes any repeated
Winamp name, and calls the original skin `WinAmp - classic`. Skin IDs and
source credits retain their original names. The toolkit renders the supplied
art and controls. The Windows Media Player skins appear separately as
`Win Media Player - Headspace`, `Win Media Player - Xbox`,
`Win Media Player - Half Life 2`, `Win Media Player - Fandom` and
`Win Media Player - Series 9`, with the saved
and command-line IDs `headspace`, `xbox`, `half-life-2`, `fandom` and `series-9`. Their pack labels retain user
overrides under the same `Win Media Player -` prefix.
The Apple appearance is listed as `Mac OS X - iTunes`, with the saved and
command-line ID `itunes`; user labels retain the separate `Mac OS X -` prefix.
The classic desktop choices are `VLC - Classic`, `RealPlayer - Classic` and
`KDE - Amarok 1.4`, with IDs `vlc`, `realplayer` and `amarok`. These names
identify appearances; the selected audio backend remains independent.

All main windows are titled **Music Player**. The adapted artwork uses the
app’s [logo](../logo.png) in place of legacy Winamp marks, matching the native
window, taskbar and tray icons. Original source archives remain unchanged.

The ten bitmap-based skins preserve their 1× artwork and author separate
2× control and chrome images with edge-aware pixel enlargement. Speaker
logos at 2× come from the original logo rather than the small 1× stamp.
Small bitmap letters, digits, lamps and display patterns retain nearest
sampling. Faces with dense ASCII or LCD artwork retain their pixel rendering.
Silver keeps its existing independently rendered artwork.

## One default layout, twenty skins

| Choice | Layout | Windows |
| --- | --- | --- |
| `default` — Default | Ordinary themed controls, playback and equaliser on the left, playlist on the right | One resizable 920 × 560 window |
| `winamp` — WinAmp - classic | Slate-blue chrome, gold title grooves, a black display with green segments, orange volume and green balance bars | Fixed strip and equaliser 275 × 116; resizable playlist starts at 275 × 232 |
| `winamp-silver` — WinAmp - Silver | Silver chrome, navy title bands, blue dot-matrix display, round keys and capsule toggles | The same dimensions and playlist resizing, with rounded corners and a tab caption on the equaliser |
| `winamp-classified` — WinAmp - Classified | Artwork adapted from Winamp3 Classified v5.5 | The same three panel windows and resizable playlist |
| `dosamp` — WinAmp - DOSamp | Original DOSamp artwork | The same three panel windows and resizable playlist |
| `windows-98` — WinAmp - Windows 98 | Original Windows 98 artwork | The same three panel windows and resizable playlist |
| `winamp-amber` — WinAmp - Classic Amber | Original Classic [CM] Amber artwork | The same three panel windows and resizable playlist |
| `calculator` — WinAmp - Calculator | Original Calculator artwork | The same three panel windows and resizable playlist |
| `sony-cdx-mp3` — WinAmp - Sony CDX-MP3 | Original Sony CDX-MP3 artwork | The same three panel windows and resizable playlist |
| `ti-82` — WinAmp - TI-82 | Original TI-82 v1.1 artwork | The same three panel windows and resizable playlist |
| `radar-amp` — WinAmp - Radar Amp | Original Radar Amp artwork | The same three panel windows and resizable playlist |
| `quake-3-arena` — WinAmp - Quake 3 Arena | Original Quake 3 Arena v2 artwork | The same three panel windows and resizable playlist |
| `headspace` — Win Media Player - Headspace | Original Windows Media Player Headspace artwork, with equaliser and playlist wings around the central player | One shaped 760 × 394 window |
| `xbox` — Win Media Player - Xbox | Original Windows Media Player Xbox artwork, with an equaliser inside the main display | Fixed 350 × 216 main window; resizable playlist starts at 448 × 316, minimum 438 × 316 |
| `half-life-2` — Win Media Player - Half Life 2 | Original Windows Media Player Half Life 2 artwork, with an orange round player and separate equaliser and playlist | Fixed 339 × 300 main window; 430 × 239 equaliser; resizable 393 × 226 playlist |
| `fandom` — Win Media Player - Fandom | Reference-adapted silver oval player, attached Settings wing and separate playlist; original skin identity and archive unverified | Fixed 490 × 330 combined main window; resizable 190 × 246 playlist |
| `itunes` — Mac OS X - iTunes | Original iTunes 4-era brushed-metal appearance, library table and MiniPlayer | Full player starts at 800 × 560, minimum 640 × 400; MiniPlayer starts at 380 × 62, horizontally resizable from 150 to 1000 pixels; fixed 430 × 230 equaliser |
| `series-9` — Win Media Player - Series 9 | Original Microsoft 9 Series Default (Corona) artwork, with a sliding playlist and equaliser, plus a compact mode | One resizable main window with a 346 × 344 minimum base; playlist adds 250 pixels of width and equaliser adds 124 pixels of height; fixed 346 × 103 compact player |
| `vlc` — VLC - Classic | Original VLC 0.8.6 wxWidgets transport icons and classic gray beveled desktop chrome | One resizable 500 × 430 window, minimum 440 × 300, with embedded playlist and equalizer |
| `realplayer` — RealPlayer - Classic | RealPlayer 8 gray-and-blue desktop arrangement with original Windows transport sprites | One resizable 720 × 520 window, minimum 606 × 500, with embedded playlist and equalizer |
| `amarok` — KDE - Amarok 1.4 | Original KDE 3-era blue transport icons, silver chrome, track context and searchable playlist table | One resizable 920 × 620 window, minimum 740 × 480, with embedded equalizer |

Dimensions are logical design pixels. Display scale changes their physical
size; artwork, window outlines and hit testing follow that scale. The
Winamp playlist keeps its footer controls on the bottom edge and expands its row
viewport. Its minimum is 275 × 232; the saved size survives skin changes
and restarts.

Default contains all three sections in its one window, with no menu bar.
Buttons, shortcuts and context menus provide its actions, and its status
line uses the ordinary theme. Equaliser and Playlist controls show
or hide the corresponding sections. Winamp skins provide those sections as
separate windows. Headspace opens and closes its wings within its one window.
Xbox keeps its equaliser in the main display and opens a separate playlist.
Half Life 2 uses a round main window with two utility panels.
Fandom embeds its Settings wing and equaliser in the main window and opens a
separate utility playlist. Its artwork is a project adaptation of the supplied
reference, rather than an import of a verified original archive.
iTunes embeds the library table in its full player and switches that same
native main window into the MiniPlayer. Its floating equaliser is an owned
utility window, so only the main player appears in the taskbar.
Ctrl+Shift+M switches full/MiniPlayer mode; Ctrl+F returns to the full player
and focuses library search. The MiniPlayer keeps its 62-pixel height while its
width can change; widths below 300 hide the LCD and seek control while keeping
transport and volume available. Each mode remembers its own size and retains the current
main-window position.
Series 9 embeds both drawers in its one native main window. Full/compact changes
keep that window and its current position. Compact mode temporarily suspends the
drawers while preserving their requested visibility, and the full player retains
its resized base dimensions independently of the drawer dimensions.
Ctrl+Shift+M switches full/compact mode; opening a drawer returns to the full
player. Ctrl+E and Ctrl+L toggle the equaliser and playlist.
All appearances retain the same model and commands.

Default and Winamp provide six transport controls — Previous, Play, Pause,
Stop, Next and Eject — with seeking, volume, balance, shuffle, repeat,
Equaliser, Playlist and Skin controls. Their equaliser has On, Auto and
Presets, a preamp and ten frequency-band faders. The Winamp playlist has
Add, Remove, Select, Miscellaneous and List options, plus six small transport controls.

Headspace keeps its four original transport buttons: Previous, combined
Play/Pause, Stop and Next. Its display provides seeking; the equaliser wing
contains volume, balance and ten bands. The equaliser menu adds On, Auto,
Presets, Preamp and Reset. The playlist header opens its menu for opening or
adding music, removing the current track, shuffle and repeat. The lower app
logo opens appearances and Settings. Wing buttons and their handles open or
close the equaliser and playlist without creating separate windows.

Xbox retains its original Previous, combined Play/Pause, Stop, Open and Next
controls, curved seek and volume sliders, and visualization selector. The
equaliser replaces the visualizer within the main display; its preset menu
provides On, Auto, Preamp and Reset. Its playlist has shuffle and repeat
controls, closes without exiting the player, and resizes from its original
grip. The app logo and menu key provide appearances and Settings.

Half Life 2 has separate Play and Pause buttons, curved seek and volume
controls, mute, a spectrum toggle and its original inner and outer shutters. Its
equaliser has ten bands, balance, On, preset cycling and reset; the preset
menu adds Auto and Preamp. Original crossfade and SRS/WOW controls remain
visible with disabled actions because the supported playback programs do
not provide those effects. Its playlist has a Now Playing dropdown, Title,
Artist, Album, Type and Length columns, and selected/total duration status.
The dropdown, headers and context menus offer Open, Add, Remove,
shuffle, repeat, appearances and Settings. Closing either satellite hides
it, and only the main window appears in the taskbar. The spectrum appears
within the circular display; original separate visualization, video and web
views are retained as source material. The original glass overlay paints once
above the dynamic inner controls and passes pointer events through.

iTunes has Previous, combined Play/Pause and Next controls, seeking, volume,
shuffle, repeat and a floating equaliser. The full player's Name, Time, Artist
and Album columns can sort the displayed rows; Search Library filters title,
artist and album without changing the playback queue. The sidebar opens the
library, playing track, file picker, backend Settings and appearances. The
original Music Store entry remains disabled because this app does not provide
that service. The MiniPlayer keeps the shared transport controls and can
return to the full library without reopening the main window.

Play starts or resumes the chosen track; Pause toggles pause/resume; Stop
returns to its beginning. Ctrl+O and Eject open the file picker. Auto chooses
a preset when the track changes. Select shows the playing track. Add appends
music files and Remove removes the current playback track. A single
playlist click selects a row without changing the playing track or clock.
Double-click, Enter and the accessibility default action play the selected
row. Arrow keys browse without starting playback, and the highlighted row
survives ordinary UI refreshes. Explicit activation can resume a selected
paused track.

## Changing skins preserves player state

The shell owns a single `players.Model`, playback backend and `players.Pulse`.
Presentation code receives that model through `players.Host`; it does not
make its own transport, queue, analyser, equaliser or playback process. Controls
call Host playback/file actions; the shell synchronizes the model with the
backend and supplies status/error text.

The shell uses `playback.Engine` commands and events. Each executable adapter
owns its native protocol, and discovery lives in `internal/playback`. Load
generations and engine identities reject updates from replaced tracks or
processes. Backend switching starts a candidate before closing the active
engine, retains it on startup failure, and joins process cleanup on exit.

Changing skin preserves track, position, queue order, volume, balance,
shuffle, repeat and equaliser values. A change between Default, Headspace, Xbox,
Half Life 2, Fandom, iTunes, Series 9 and the Winamp family replaces the window composition around that model.
Switching between Winamp skins keeps their controls and updates their layout and paint.
The active main window hosts the application's update timer. Changing skin
does not restart the external audio process.

Closing the main window hides all player panels to the tray while music
and UI timers continue. Tray activation restores the main window and the
panels that were visible; temporary hiding does not change saved panel
preferences. Tray Quit, Escape and Ctrl+Q exit and release the audio backend.
Close exits normally if there is no shown tray host. If the host disappears
while the player is hidden, the windows return to the desktop.

Ctrl+K cycles Default and the optional skins in the table’s order.
Ctrl+Shift+K switches between Default and the last optional skin. The Skin
control, a context menu and Shift+F10 provide named choices. The compact
skin menu lists all eighteen appearances. When a compact host constrains the
menu height, the toolkit scrolls the choices with arrow keys or the wheel,
including its in-window fallback. A toolkit
[Home/End focus-scrolling bug](../uitoolkit-gaps.md#25-popup-homeend-and-initial-up-focus-rows-outside-the-viewport)
remains recorded; ordinary arrow and wheel scrolling reach every choice.
No separate layout-selection command is needed.

The app saves its session in
`$XDG_CONFIG_HOME/uitoolkit/media-player-music.json`: queue order, selected
track and playback offset, audio backend, skin, equaliser/playlist visibility, and window
placement with snap relationships. Default stores its size and position
independently; all Winamp skins share their panel arrangement. Headspace
stores its own position. Xbox, Half Life 2, Fandom, iTunes, Series 9, VLC,
RealPlayer and Amarok store their window arrangements
separately from the other families. The saved
queue resumes only after Play, and entries whose files no longer exist are
skipped. Known positions restore where the desktop supports them; an
unsupported backend preserves previously saved coordinates for a future
session. Resizable window sizes are clamped to their layout's minimum;
the Winamp main strip/equaliser, Headspace, Xbox main, Half Life 2 main and Fandom main
retain their fixed dimensions.
iTunes also remembers its full/MiniPlayer mode and each mode's size. Switching
modes keeps the main window, playback backend and tray item alive; opening
the library from the MiniPlayer returns to the full player.
Series 9 also stores its full/compact mode and full base size in its own family;
drawer visibility remains a shared model preference, including while compact or
temporarily hidden to the tray. Saved mode restores before window placement.

The restorers clamp saved sizes to each layout's minimum and wait for a
ready window before applying bounded placement requests. The toolkit now
keeps initial frame positions stable and updates layout after exact native
resizes; the explicit resize-layout compensation has been removed. A
window-manager refusal or later user move remains the new placement.

With no preference the app starts in Default. Legacy classic and silver
preferences are translated to `winamp` and `winamp-silver`; an obsolete or
unknown choice falls back to Default. Screenshot runs neither restore nor
save the live session.

## The app and toolkit boundary

The application supplies the design. uitoolkit supplies reusable UI
machinery without knowing what a music player, playlist or equaliser is.

| Application owns | Toolkit owns |
| --- | --- |
| Model, backend discovery and playback, Settings, file commands, folder scanning, session data, keyboard policy and window composition | Event dispatch, drag/drop negotiation, widgets, focus and accessibility infrastructure |
| Layout algorithms and control-to-slot bindings | Layout primitives and named slot placement |
| Skin manifests, artwork, sprites and design generators | Skin parsing, state resolution, scaling and painting |
| Custom readouts, transport glyphs and playlist row paint | Drawing APIs, clipping, hit shapes and platform windows |

The app should declare its main anchor, member panes, relative layout and
snap/detach behavior. The toolkit should handle platform capabilities and
the physical windows or host surfaces that realize that declaration,
including a portable unsupported-case strategy. The current X11 preference
below is a temporary workaround for a missing toolkit feature.

The skinned equaliser and playlist hide their minimize buttons through the
caption API and declare `RoleUtility` with Main as `Owner`. The toolkit
handles platform role, ownership and taskbar/pager policy. The app's former
C/X11 adapter and utility policy polling have been removed. Main's native
recreation still leaves existing children pointing to the old owner, so a
small public `SetOwner` refresh remains until
[the ownership lifetime defect](../uitoolkit-gaps.md#30-parent-recreation-leaves-utility-children-owned-by-a-destroyed-window)
is fixed. Native Wayland advertises ownership without explicit taskbar
control; the toolkit exposes that capability limit.

The playlists use toolkit URI-list drop dispatch and negotiate Copy, then
pass local paths to the background folder/file loader. Remote targets remain
separate in `DropEvent.Remote`. The shared host rejects remote-only and
mixed local/remote requests without reparsing toolkit data.

The toolkit's automatic compact chooser now creates an owned window with
visible listing and action buttons. The app keeps a separately tracked
chooser across skin changes and tray hiding, prevents duplicate opens and
transfers its owner to the new main window before closing the old one.
This independent dialog lifetime is application policy.

`internal/skins/packs` contains fifteen generated packs. They are
embedded in the executable. `skins.Register` passes each embedded filesystem directly to
`style.RegisterSkinFS` before preferences or looks are resolved. Registration
retains the filesystem; no extracted directory or shutdown cleanup is needed.
The binary therefore works outside this checkout. The user's own skin
packs take precedence, and the app never installs its packs into the
user's toolkit skin directory.

`internal/skins/plans` contains generated and imported bitmap plans; the app's
`internal/skins/panel` defines their design geometry. The toolkit's public
`skingen` types, renderer and manifest writer generate the packs:

```bash
go generate ./internal/skins
```

### Layouts belong to the app

The default interface implements `Measure` and `Arrange` in its own
components. Other applications can use those same APIs for responsive,
overlapping, radial or other layouts; a skin manifest is not a general
constraint language.

The panel skins declare fixed `winamp.strip` and `winamp.equaliser`
layouts, and a responsive `winamp.playlist` layout. Named slots describe
the positions of controls and readouts. The player binds components with `widget.Slots`
and reads rectangles through `style.SkinSlotRect`. The app's generators use
the same geometry to draw the wells and grooves and to write their slots,
so artwork and control placement share one design. Playlist slots use the
runtime's stretching and edge anchors. v0.23.4's `SlotSpec` authors those
fields directly, so the app's JSON anchor rewrite has been removed. Shape
edge pins also round-trip through the generator. Font family preferences
still require a small manifest adapter because `skingen.Plan` lacks the
runtime's font preference schema; this remaining authoring request is in
[item 17](../uitoolkit-gaps.md#17-skin-generator-cannot-emit-supported-responsive-slot-anchors).

Headspace declares its separate `headspace.main` layout. Its central controls,
equaliser and playlist wings belong to that presentation rather than the
Winamp window arrangement. Xbox declares `xbox.main` and `xbox.playlist` for
its player display and responsive satellite. Half Life 2 declares
`halflife2.main`, `halflife2.equalizer` and `halflife2.playlist` for its
separate window family. The shell validates each family's required layouts
before changing the active skin. Fandom declares `fandom.main` and
`fandom.playlist` for its combined Settings/main window and responsive satellite.
iTunes declares `itunes.main`, `itunes.mini` and `itunes.equalizer`; full and
compact views swap the main window's content and native size limits, leaving
its identity, playback, tray ownership and utility parent unchanged.
Series 9 declares `series9.main`, `series9.transport`, `series9.equalizer`,
`series9.playlist` and `series9.compact`. The app places the original embedded
subviews in one native window and remembers the full base size without counting
drawer extensions twice. Its original transparent animation reservation is not
included in the application's desktop bounds.
The classic desktop appearances declare `vlc.main`, `realplayer.main`,
`realplayer.playlist`, `realplayer.equalizer` and `amarok.main`. They use
app-owned responsive layouts and source control artwork, with their sections
inside one main window. A shared `players.QueueView` composes the stock list
with queue-index mapping, browsing and activation policy, interrupted gesture
guards and copy-only local file drops. Each skin supplies its own row geometry
and paint; Amarok's search and sorting leave the underlying queue unchanged.

Imported skins place their resizable playlist backgrounds on a nonpixelated
sheet. This avoids the
toolkit’s [fractional panel-slice scaling defect](../uitoolkit-gaps.md#24-resized-pixelated-panel-slices-lose-fractional-display-scaling);
the original background is retained in the 1× artwork. The manifest
adapter also writes explicit smooth-sheet overrides to avoid the generator’s
[mixed-sheet policy loss](../uitoolkit-gaps.md#26-skin-generator-loses-nonpixelated-overrides-in-mixed-sheet-plans).

The app separately chooses the sampling policy for each type of artwork.
The bitmap skins provide higher-detail 2× chrome on smooth sheets and keep
small glyphs and digits on nearest-sampled sheets. Optional `backdrop.*`
slots restore the original display patterns before live readouts are painted;
their geometry and art belong to the skin. Packs without these slots need
no change. Scale-specific drawing uses the public authoring context's scale,
so the generated manifests and images remain reproducible through `skingen`.

A compatible external panel can rearrange controls using those slot names,
including edge anchoring and stretching. The app still decides which
components exist, what they do, their keyboard order and their accessible
names. A missing slot is reported to the app; every required control needs
a slot in a compatible pack.

The equaliser's window receives the `winamp.equaliser` frame role. Winamp
Silver uses that role for its tab caption while the other two windows keep
title bands. Window titles come from the app. Named sprites provide digits,
letters, lamps and thumbs through `style.DrawSkinSprite`; slot art supplies
a key's state through `style.DrawSkinSlot`. Named `player.*` text roles
supply row, selection, readout, equaliser and analyser colors, so imported
packs can define those colors without adding another app palette. Imported
main readouts retain their source bitmap glyph colors; optional `info.font.*`
masks let the playlist readouts use its own original foreground and background
colors. Printed EQ labels are adapted to the app’s actual ISO frequencies
and ±12 dB range. Imported playlist font preferences retain the original
family names with installed-font fallback; the app does not bundle those
fonts. Source archives remain unchanged.

### Appearance hooks retain widget behavior

`players.GlyphButton` configures `widgets.ToolButton`: the app supplies a
label, action, glyph and optional skin art. `Painter`, `ContentPainter` and
`Shaper` let it customize appearance while the widget handles focus,
presses, keyboard activation and the focus ring.

`players.Fader` configures `widgets.Slider` with range, formatted reading,
step, page and action. `Vertical`, `Painter` and `Travel` support a fader
whose thumb rides a printed groove. Imported packs can provide optional
`volume.frame.N`, `balance.frame.N` and `eq.frame.N` sprite sequences.
The app selects a frame from the current value before painting the thumb;
packs without frames retain their printed backgrounds. The toolkit owns dragging, keyboard
steps and accessible increment/decrement actions. Model refreshes inside
`OnInput` or `OnChange` no longer produce nested user-input callbacks;
the app's callback-suppression guard has been removed.

The panel playlist configures `widgets.ListView`. `RowGeo` supplies its
viewport, groove, row height and thumb size. `RowPaint` supplies its font,
colors and right-aligned track duration. `ScrollPaint` supplies thumb art.
Selection, keyboard navigation, scrolling, outer-component focus and
accessible viewport bounds use the toolkit. `OnActivate` commits keyboard
and accessibility actions; the app supplies playback and context-menu actions.
Only the interrupted pointer-gesture safeguard remains app-owned until
the toolkit fixes its click history.

## Desktop positioning

The panel windows use the toolkit's public `rack` package to snap and move
as a group. X11, including XWayland, and the headless backend expose
positions and allow windows to be placed. The app adopts the desktop's
actual boxes after mapping, then attaches the equaliser and playlist to
the strip. Moving a window is a request; the rack waits for the desktop's
response.

Dragging the main strip carries its snapped group. Dragging a satellite
away detaches it from Main; panes attached to that satellite travel with
it. Bringing it back near an edge rejoins the group. Moving Main to a loose
satellite also rejoins it. The toolkit's old path-reversal hang is fixed,
but rejoining a longer foreign chain still drops intermediate links and a
small snap gap moves stationary foreign panes. The app retains a guarded
public Rack pass to preserve the component and snap Main to it. It also
retains a final reentrant movement notification. Group following uses
movement events while playback is playing, paused or stopped.

The toolkit now uses a 250 ms acknowledgement deadline. Its callbacks still
run a pass per event rather than coalescing a complete batch, and no expiry
wake-up follows a final move ignored within that interval. The ordinary
70 ms app pulse therefore still checks unresolved geometry until the
request is acknowledged or its deadline expires. Settled event-capable
windows need no position polling. Native event-driven movement, detachment
and rejoining pass once startup placement is settled; these programmatic
checks do not establish continuous-drag visual smoothness.

The executable currently prefers available X11 at Linux startup, including
XWayland, even when it starts in Default. This is a temporary workaround to
retain group movement after switching to a skin. It should be removed when
the toolkit supplies portable logical-window grouping and its fallback
strategy; choosing X11/XWayland is not a permanent application responsibility.
Explicit `UITK_BACKEND` choices are honored, headless screenshots use
offscreen, and the native toolkit fallback remains available without X11.
On a Wayland-only system the skins still render and their controls work;
their separate native windows cannot move as a snapped group. There is no
mandatory XWayland dependency.

Native Wayland ordinary toplevels expose no global position and cannot be
placed by the client. That protocol/capability restriction prevents this
grouped positioning; it is not a toolkit defect. Layer-shell placement and
interactive toplevel dragging are separate capabilities and do not provide
arbitrary positioning of these ordinary application windows. The model and
panel controls still work. Default's single-window layout needs no satellite
positioning.

The missing feature is a higher-level grouping abstraction, rather than a
different implementation of Wayland `Move`. The toolkit already provides
`rack` for separate windows and `dock` for panes within a host, but does not
choose or migrate between representations for a declared skin-window group.
A portable strategy might keep docked logical panes in one physical host,
or use suitable compositor cooperation where available, while retaining the
app's artwork, geometry and behavior. That strategy belongs in the toolkit.

Native popup surfaces already existed in v0.23.1 and remain available in
v0.23.4. A general menu is not universally confined to its parent window.
Where a popup is unavailable, disabled or refused, the toolkit uses an
in-window layer. The player’s skin menu uses the toolkit’s constrained, scrollable choice
list on either path.

## Build, keyboard and screenshots

```bash
go build .
./media-player-music
./media-player-music track.mp3 album.flac
./media-player-music -skin default
./media-player-music -skin winamp
./media-player-music -skin winamp-silver
./media-player-music -skin dosamp
./media-player-music -skin windows-98
./media-player-music -skin itunes
./media-player-music -skin winamp -scale 1.75 -shot /tmp/winamp-175 -at 1m37s
```

The command accepts `./media-player-music [flags] file1.mp3 file2.flac` and
auto-plays the first provided file. Without files, use Eject/Open/Add from
the restored queue or drop music files/folders onto the playlist. A first
launch has an empty queue. The public options are `-skin` with any ID in the table above,
`-scale`, `-audio-backend`, `-shot` and `-at`. Run `./media-player-music -help` for all choices. A plain build uses an ordinary dark fallback where an era
base is unavailable. Era engines are a toolkit build configuration:

```bash
go build -tags theme_engine_all .
# Or only the bases used by these layouts:
go build -tags 'theme_engine_win95,theme_engine_luna,theme_engine_breeze' .
```

All skin packs are embedded with either configuration. Engine tags select
which ordinary base themes are compiled in; they are not a packaging defect.

```text
Space         play / pause     Z / B          previous / next
V             stop             M              mute
Left / Right  seek             Up / Down      volume
S             shuffle          R              repeat: off → all → one
Ctrl+O        open music files
Ctrl+,        audio backend settings
Ctrl+K        next skin         Ctrl+Shift+K   Default / last optional skin
Ctrl+E        show/hide equaliser    Ctrl+L    show/hide playlist
Shift+F10     skin menu         Escape / Ctrl+Q  quit
```

Focused controls also consume their own navigation keys. The app's
transport shortcuts bubble from controls to the shared keyboard handler.

Screenshot mode creates headless windows with synthetic tracks and poses
the model at `-at`, which defaults to 97 seconds. It writes one PNG per open
window and exits without starting an audio backend. The synthetic analyser is a pure
function of the fixture's clock position, so the same skin, scale and
position produce the same picture. Posing resets its peak markers
consistently; it is not a visualisation of live audio.

## Verification

```bash
tools/testenv.sh go test ./...
tools/testenv.sh go test -race ./...
tools/testenv.sh go test -tags theme_engine_all ./...
tools/testenv.sh go test -race -tags theme_engine_all ./...
```

The helper removes the Wayland and X11 display and supplies a private
D-Bus session and private XDG directories. The default and all-engine
configurations exercise different base-theme availability. Presentation
fixtures retain the invented library; playback/file hooks verify action routing
without decoding audio or launching a playback process.

Engine and controller integration tests start the installed mpv with
`--ao=null`, so decoding, transport and filter checks are silent. Controller
tests inject that real engine into a headless shell; this differs from the
synthetic presentation fixtures. These integration tests skip when mpv is
missing, and Unix IPC tests also skip on unsupported platforms. The shell
integration checks accepted balance, preamp and band filters through mpv's
audio-filter property, along with the running clock and playback state.
Measured spectrum tests distinguish low and high tones, reject stale
seek/track packets, check pause and silence, and preserve visualization
across all appearances. Backend tests verify channel preservation and
byte-identical decoded output with and without analysis. Protocol tests
exercise all four backend adapters; installed VLC, MPlayer and mpg123 have
silent playback checks too. Settings regressions use actual toolkit input
and accessibility actions for selecting, refreshing and retrying programs,
test availability marks, persistence and retained DSP values, and verify
the preferred and minimum layout at 1× and 1.75×.

Live backend checks were run on Linux. The Windows application and macOS
audio package cross-compile; native playback still needs validation on those
systems. MPV tests use v0.41.0. Older IPC versions that omit returned playlist
entry IDs cannot always distinguish failed loads during rapid replacement;
the adapter avoids attributing an old failure to the new track.

The skin tests parse and lint the bundled packs, decode their artwork, and
regenerate manifests and PNGs for comparison with the embedded bundle.
Registration tests cover direct embedded and unrelated in-memory packs,
user overrides, replacement with a fresh look, and startup without a writable
temporary directory. The panel tests exercise placement, paint, hit shapes,
keyboard order and accessibility at fractional scales. An external panel
fixture rearranges real controls with anchored and stretched slots.

Shared-model tests cover transport wrapping, seeking, shuffle and repeat.
Presentation-switch tests check state preservation and window composition.
Playlist tests exercise selection, scrolling, inset accessible rows,
outer-component focus, single-click selection, double-click activation and
repeated accessibility activation. A recording-backend regression proves
selection sends no playback commands and double-click loads exactly one
track in stopped, paused and playing states across all appearances.
Headless URI-drop tests negotiate
URI Copy delivery across the playlist, including empty lists, reject remote
file authorities and incompatible offers, and route decoded paths to the
app's loader. Folder tests cover recursive sorted expansion and background
request ordering. Caption-drop tests cover the two original skins and resized playlists,
preserve dragging and pointer/accessibility Close, and exercise the normal
shell timer after caption replacement. Session tests cover queue/offset restoration without
autoplay, missing files, independent default placement, shared skin placement
and retained detached or hidden panels. Structural accessibility checks are supplemented by actual menu action
dispatch tests for both in-window and simulated native popup surfaces.

The optional `TestNativeXDNDPlaylistFolderDrop` uses a separate Xlib sender
to transfer an actual `text/uri-list` selection to Default, Winamp and
Winamp Silver on X11 at 1.75x. It checks the native drop advertisement after
frame recreation and finds a viewable target through the root window tree.
It verifies Copy negotiation, including a source
that prefers Move, and recursive queue additions with a recording backend
that plays no audio. Enable it with `MUSIC_PLAYER_NATIVE_XDND_TEST=1` and an
X11 display; `MUSIC_PLAYER_NATIVE_DROP_FOLDER` chooses an existing folder.
The README includes the command. A locked desktop can leave these windows
unmapped; the test then skips visible target discovery. Direct messages to
a known unmapped XID can test protocol transfer and loading but do not
establish that a file manager can discover that target. Regular tests keep
native windows disabled.

The optional utility-window test also verifies taskbar policy through KWin's
own window state when its read-only scripting verifier is available. It
covers Main retaining its taskbar entry, both panels remaining excluded,
child and parent recreation, public mapped-state setters and hide/re-show.
The verifier inspects only its test windows and unloads its own script.

The optional `TestNativePlaylistResizeAndSavedSize` opens owned X11 windows
at 1.75x and checks both skins' native minimum-size hints, enlarged row
viewport, footer artwork, utility-window state and restored playlist size.
Enable it with `MEDIA_PLAYER_NATIVE_WINDOW_TEST=1` and an X11 display;
it also requires `xprop` for read-only inspection of those test windows.
These checks exposed a separate native
[SHAPE ordering defect](../uitoolkit-gaps.md#18-x11-shaped-resize-bands-violate-the-declared-rectangle-ordering)
in resizable rounded windows; v0.23.4 fixes it and the native recheck passes.

A screenshot check should cover Default and all optional skins at 1x and
1.75x, with every separate player window included. The README links the
current default window and each skin's main-window illustration.

## Toolkit review at v0.23.4

The [report for the toolkit owner](../uitoolkit-gaps.md) records all thirty-three
items with per-item status, independent public probes, expected and actual
behavior, severity and release source references. App safeguards are not
counted as proof of a toolkit fix.

Verified fixes cover slider normalization, hidden grid spans, wrapped-list
focus/accessibility, popup accessibility actions, compact chooser hosting,
remote URI classification, utility-window policy, child role preservation,
initial X11 frame positioning, resize geometry, SHAPE ordering and move
coordinates. Registered skin enumeration and live artwork replacement now
work. Generated slot anchors and shape pins round-trip directly. The
toolkit-only upgrade left all 51 bundled manifest/art files unchanged;
subsequent application branding changes regenerated the packs with the
speaker logo and **Music Player** captions.

Both playlists adopt explicit activation and context APIs. The app removes
its disabled-input guard, duplicate raw-URI parser, native utility code and
post-generation anchor rewrite. It retains only the verified pointer
interruption guard, guarded group rejoining, owner identity refresh,
background/sheet/font adapters and unique tray icon name.

The remaining defects include longer rack path reversal, stationary snap
geometry, incomplete event batching/deadline wake-up, stale ownership after
parent recreation, list horizontal hit testing and interrupted double-click
history. Fractional resized pixelated backgrounds, mixed sheet serialization,
font preference authoring, tray raster priority and popup focus jumps also
remain open. The release notes additionally confirm macOS requested moves
reverting; that platform was not locally tested in this Linux review.

Portable logical-window grouping remains a declined design requirement.
Ordinary native Wayland windows render without XWayland, but separate windows
cannot follow as a group and taskbar requests are unavailable. The temporary
X11 preference remains. Continuous native drag smoothness needs visual
confirmation from the rebuilt player.
