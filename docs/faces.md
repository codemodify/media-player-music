# The interface and skins

> The links below to `goframework-gaps.md` point at a file retired on
> 2026-11-09 (its last version is in this repository's history, before
> c64f45b; "UIToolkit" there is the toolkit desktop-ui grew out of). What
> the player needs from the framework now is in goframework's ledger,
> `goframework.io/app-requests.md`.

The player has one default interface and twenty-one optional skins. The default
uses ordinary toolkit controls in a single window. Eleven Winamp skins use
compact panels; Headspace, Xbox and Half Life 2 reproduce separate Windows
Media Player arrangements. Fandom adapts a supplied silver-player reference,
and iTunes recreates the brushed-metal Mac OS X player with a MiniPlayer.
Series 9 adapts Microsoft's original blue-and-silver Windows Media Player skin,
with embedded drawers and a compact player.
Classic VLC, RealPlayer 8 and Amarok 1.4 reproduce the older desktop
arrangements in separate resizable window families.
Fancy adapts a supplied blue-player reference into three shaped panels and a
compact main view.
This document explains their layout, the app/toolkit
boundary, and the v0.23.21 audit.

## Playback and screenshot fixtures

Playback is the player's own, in its process: `internal/playback`'s
`Builtin` engine on goframework's audio-playback (every format
`audio-playback/all` imports: MP3, AAC, FLAC, Ogg Vorbis and Opus, ALAC,
WAV, AIFF and AU, in MP4 and Matroska too), through the system's sound
(system-audio-all). Nothing is installed or started. Formats goframework
has no decoder for (WMA, APE, Musepack, WavPack...) do not play: a file of
one is refused with a message naming its format, and a folder leaves them
out and says how many. Since 2026-11-12 the external engines (mpv, VLC,
MPlayer, mpg123) are gone, with their discovery, settings and saved
`audio_backend` field (an old saved value is ignored).

Default's gear button, player context menus and Ctrl+comma open the
ordinary themed Settings window (520 × 480 logical pixels, at least
460 × 460): the formats that play, and ReplayGain: Off, Track gain or
Album gain, and a preamp from -15 to +15 dB. A track's gain comes from its
tags: ID3v2's TXXX `REPLAYGAIN_*` frames, RVA2 and APEv2 tags are read by
`internal/tags` (goframework's tag readers do not give them yet: ledger
R97, R98), Vorbis comments and MP4's freeform items by audio-playback
itself. The tags' peak keeps the gain from clipping. audio-playback takes
a player's ReplayGain as it is made, so a change is heard from the next
track (R100). Escape closes the dialog.

The first startup begins with an empty playlist. Later launches restore the
saved queue, selected track and playback offset, stopped until Play.
Positional music-file paths replace the queue and start the first file.
Ctrl+O and Eject choose a replacement playlist; Playlist → Add appends files. The model
tracks the real file, position, queue, volume, balance, shuffle/repeat and
equaliser values. Seeking and volume affect real playback. Balance and
the ten-band/preamp equaliser are audio-playback's. Every presentation
displays 48 frequency bands measured from that processed audio
(audio-playback's analyser, normalised from -60 dB). Playback generations
and epochs reject stale bands after a seek or track change; paused,
stopped or stalled levels decay to zero.

The whole playlist accepts file-manager drops, including its empty list,
heading and controls. For panel skins, an app-owned drop ancestor also
routes the toolkit caption's title band and controls to that same action;
the original caption keeps its dragging and close behavior. Drops append
local files or folders using the Copy
action, so the source files stay in place. Folder expansion runs off the UI
thread and adds recognized audio files and media containers, including MP4,
M4V, WebM and MKV, recursively in deterministic sorted
order. Requests commit in their
arrival order. A video file plays its audio only. An addition to an empty queue starts the first track; an
addition to an existing queue leaves its current playback intact.

Headless screenshots and presentation tests use invented tracks and a
synthetic clock-driven analyser. They play no sound. The
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
identify appearances; playback is the player's own whatever the appearance.
The reference-adapted `fancy` appearance is always displayed as **Fancy**, without
a player-brand prefix, including when a user pack provides another label.

All main windows are titled **Music Player**. The adapted artwork uses the
app’s [logo](../logo.png) in place of legacy Winamp marks, matching the native
window, taskbar and tray icons. Original source archives remain unchanged.

The ten bitmap-based skins preserve their 1× artwork and author separate
2× control and chrome images with edge-aware pixel enlargement. Speaker
logos at 2× come from the original logo rather than the small 1× stamp.
Small bitmap letters, digits, lamps and display patterns retain nearest
sampling. Faces with dense ASCII or LCD artwork retain their pixel rendering.
Silver keeps its existing independently rendered artwork.

## One default layout, twenty-one skins

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
| `fancy` — Fancy | Blue shaped player adapted from the supplied reference, with separate equalizer and playlist panels; no imported original archive | Fixed 420 × 220 main player or 420 × 40 compact view; fixed 420 × 124 equalizer; resizable playlist starts at 420 × 144, minimum 420 × 124 |

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
Fancy keeps the equalizer and playlist in separate owned utility windows; only
the main player appears in the taskbar. Its full and compact views use the same
main window and keep the current playback and position. Ctrl+Shift+M switches
between those views.
The [main player](screenshots/fancy-main.png),
[equalizer](screenshots/fancy-equaliser.png),
[playlist](screenshots/fancy-playlist.png) and
[compact player](screenshots/fancy-compact.png) previews use posed demo tracks.
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
library, playing track, file picker, Settings and appearances. The
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

The shell owns a single `players.Model`, playback engine and `players.Pulse`.
Presentation code receives that model through `players.Host`; it does not
make its own transport, queue, analyser, equaliser or playback process. Controls
call Host playback/file actions; the shell synchronizes the model with the
engine and supplies status/error text.

The shell uses `playback.Engine` commands and events (the built-in
engine; tests give a fake one). Load generations and epochs reject updates
from replaced tracks and superseded seeks.

Changing skin preserves track, position, queue order, volume, balance,
shuffle, repeat and equaliser values. A change between presentation families replaces the window composition
around that model.
Switching between Winamp skins keeps their controls and updates their layout and paint.
The active main window hosts the application's update timer. Changing skin
does not restart playback.

Closing the main window hides all player panels to the tray while music
and UI timers continue. Tray activation restores the main window and the
panels that were visible; temporary hiding does not change saved panel
preferences. Tray Quit, Escape and Ctrl+Q exit and close the sound stream.
Close exits normally if there is no shown tray host. If the host disappears
while the player is hidden, the windows return to the desktop.

Ctrl+K cycles Default and the optional skins in the table’s order.
Ctrl+Shift+K switches between Default and the last optional skin. The Skin
control, a context menu and Shift+F10 provide named choices. The compact
skin menu lists all twenty-two appearances. When a compact host constrains
the menu height, the toolkit scrolls the choices with keys or the wheel,
including its in-window fallback. UIToolkit v0.23.5 also fixes
[Home/End and initial-Up focus scrolling](../goframework-gaps.md#carried-over-from-uitoolkit),
keeping the focused choice visible.
No separate layout-selection command is needed.

The app saves its session in
`$XDG_CONFIG_HOME/goframework-desktop-ui/media-player-music.json`: queue order, selected
track and playback offset, ReplayGain settings, skin, equaliser/playlist visibility, and window
placement with snap relationships. Default stores its size and position
independently; all Winamp skins share their panel arrangement. Headspace
stores its own position. Xbox, Half Life 2, Fandom, iTunes, Series 9, VLC,
RealPlayer, Amarok and Fancy store their window arrangements
separately from the other families. The saved
queue resumes only after Play, and entries whose files no longer exist are
skipped. Known positions restore where the desktop supports them; an
desktop that cannot report positions preserves previously saved coordinates for a future
session. Resizable window sizes are clamped to their layout's minimum;
the Winamp main strip/equaliser, Headspace, Xbox main, Half Life 2 main and Fandom main
retain their fixed dimensions.
iTunes also remembers its full/MiniPlayer mode and each mode's size. Switching
modes keeps the main window, playback and tray item alive; opening
the library from the MiniPlayer returns to the full player.
Series 9 also stores its full/compact mode and full base size in its own family;
drawer visibility remains a shared model preference, including while compact or
temporarily hidden to the tray. Saved mode restores before window placement. Fancy stores its full/compact mode
in its own family together with all three panel positions and playlist size.

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

The application supplies the design. goframework.io's desktop-ui supplies reusable UI
machinery without knowing what a music player, playlist or equaliser is.

| Application owns | Toolkit owns |
| --- | --- |
| Model and playback, Settings, file commands, folder scanning, session data, keyboard policy and window composition | Event dispatch, drag/drop negotiation, widgets, focus and accessibility infrastructure |
| Layout algorithms and control-to-slot bindings | Layout primitives and named slot placement |
| Skin manifests, artwork, sprites and design generators | Skin parsing, state resolution, scaling and painting |
| Custom readouts, transport glyphs and playlist row paint | Drawing APIs, clipping, hit shapes and platform windows |

The app should declare its main anchor, member panes, relative layout and
snap/detach behavior. The toolkit should handle platform capabilities and
the physical windows or host surfaces that realize that declaration,
including a portable unsupported-case strategy. desktop-ui-rack's `Group`
does this now (ledger R21): see "Desktop positioning".

The skinned equaliser and playlist hide their minimize buttons through the
caption API and declare `RoleUtility` with Main as `Owner`. The toolkit
handles platform role, ownership and taskbar/pager policy. The app's former
C/X11 adapter and utility policy polling have been removed. UIToolkit v0.23.5
automatically refreshes existing children's native owner when Main is
recreated; the independent
[ownership lifetime check](../goframework-gaps.md#carried-over-from-uitoolkit)
passes. The app retains its small public `SetOwner` refresh defensively.
Native Wayland advertises ownership without explicit taskbar control; the
toolkit exposes that capability limit.

The playlists use toolkit URI-list drop dispatch and negotiate Copy, then
pass local paths to the background folder/file loader. Remote targets remain
separate in `DropEvent.Remote`. The shared host rejects remote-only and
mixed local/remote requests without reparsing toolkit data.

The toolkit's automatic compact chooser now creates an owned window with
visible listing and action buttons. The app keeps a separately tracked
chooser across skin changes and tray hiding, prevents duplicate opens and
transfers its owner to the new main window before closing the old one.
This independent dialog lifetime is application policy.

`internal/skins/packs` contains twenty-one generated packs. They are
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
edge pins also round-trip through the generator. v0.23.6 adds `Plan.Fonts`
for ordered UI and mono family preferences. The app now supplies imported
playlist families directly in its plans and delegates generation to the
toolkit; no manifest JSON adapter remains. This resolves
[item 17](../goframework-gaps.md#carried-over-from-uitoolkit).

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
inside one main window. Fancy declares `fancy.main`, `fancy.compact`,
`fancy.equalizer` and `fancy.playlist` for its three shaped panels and compact
main view, with layout and artwork authored from the supplied reference.
A shared `players.QueueView` composes the stock list
with queue-index mapping, browsing and activation policy, interrupted gesture
guards and copy-only local file drops. Each skin supplies its own row geometry
and paint; Amarok's search and sorting leave the underlying queue unchanged.

Imported skins retain their resizable playlist backgrounds on smooth sheets
and preserve the original background in the 1× artwork. This sampling
choice originally avoided the
[fractional panel-slice defect](../goframework-gaps.md#carried-over-from-uitoolkit),
which now passes independently in v0.23.5. The generator also preserves
[mixed-sheet sampling directly](../goframework-gaps.md#carried-over-from-uitoolkit),
so the app no longer repairs those manifest fields.

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
The app retains its pointer-gesture guard for track remapping, same-count
queue replacement and global-key interruptions. The recorded stock list
click-history defects are fixed; this remaining guard preserves the
player's track-identity policy.

## Desktop positioning

The four panelled faces (winamp, fancy, fandom, halflife2) declare their
panels as the members of a desktop-ui-rack `Group`: each panel's content,
size, silhouette (the skin's shape slot) and close policy, and for the
Winamp skins its frame role (`strip`, `equaliser`, `playlist`), then the
bonds between them (Fancy's and Winamp's stack, Fandom's playlist 46
pixels down the player's right edge, Half Life 2's equaliser 80 pixels up
it). The group realises them on whatever backend the toolkit chose; the
app no longer prefers X11.

Where a program may place its windows (X11, Windows, macOS, the offscreen
backend), each panel has a window of its own. The group opens them, puts
the declared bonds and the saved places in place past the window manager's
own placement, then follows the desktop: dragging the strip carries what
is snapped to it, dragging a satellite away detaches it with what hangs
from it, and bringing it near an edge snaps it back. Where a program may
not (Wayland), the panels stuck together share one shaped window, framed
per panel for its role; a panel's caption dragged away takes it out into a
window of its own, which docks back when dropped on another part. A
desktop that says it places windows and places none (sway as an X11
window manager) is shared the same way.

A saved layout is each panel's box and bond (`Dock.Along` included).
Restored, the resizable playlist gets its size back, each docked panel its
bond along its edge (a bond the group refuses, a cycle in a hand-edited
file, leaves the panel loose) and each loose panel its place, which the
group keeps while it settles. Sharing windows, a box is in the group's own
plane and is saved unpositioned. Closing a satellite hides it; showing it
brings it back where it was. Closing the player is the shell's close to
tray, through the group's first member.

Native popup surfaces already existed in v0.23.1 and remain available in
v0.23.5. A general menu is not universally confined to its parent window.
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
`-scale`, `-shot` and `-at`. Run `./media-player-music -help` for all choices.
desktop-ui chooses window systems and theme engines by import, not by build
tag: [internal/players/toolkit.go](../internal/players/toolkit.go) imports
desktop-ui-platform-all and the engines the player draws with (Breeze for
the default face, Windows 95 and Luna for the skins built over them), which
every face, the shell and main link, so tests see the same set as the
program. A look.json naming another pack shows the nearest one of these. All skin packs are embedded in every build.

```bash
go build .
go build -tags font_emoji .   # adds complex-script shaping and colour emoji
```

```text
Space         play / pause     Z / B          previous / next
V             stop             M              mute
Left / Right  seek             Up / Down      volume
S             shuffle          R              repeat: off → all → one
Ctrl+O        open music files
Ctrl+,        settings (ReplayGain)
Ctrl+K        next skin         Ctrl+Shift+K   Default / last optional skin
Ctrl+E        show/hide equaliser    Ctrl+L    show/hide playlist
Shift+F10     skin menu         Escape / Ctrl+Q  quit
```

Focused controls also consume their own navigation keys. The app's
transport shortcuts bubble from controls to the shared keyboard handler.

Screenshot mode creates headless windows with synthetic tracks and poses
the model at `-at`, which defaults to 97 seconds. It writes one PNG per open
window and exits without playing anything. The synthetic analyser is a pure
function of the fixture's clock position, so the same skin, scale and
position produce the same picture. Posing resets its peak markers
consistently; it is not a visualisation of live audio.

## Verification

```bash
tools/testenv.sh go test ./...
tools/testenv.sh go test -race ./...
tools/testenv.sh go test -tags font_complex ./...
tools/testenv.sh go test -tags font_emoji ./...
```

The helper removes the Wayland and X11 display and supplies a private
D-Bus session and private XDG directories. `font_complex` and `font_emoji`
are the player's own tags: they link desktop-ui-text-complex into the font
tests. Presentation
fixtures retain the invented library; playback/file hooks verify action routing
without decoding audio or launching a playback process.

The engine's tests (internal/playback) play into goframework's audiotest
sound system in memory, in synctest bubbles: no device, nothing heard.
They play MP3 and AAC to their end with positions, the format, metadata
and 48 bands; load at a position paused, seek before a track opens,
replace and stop loads; level tracks by ReplayGain from tags the tests
write (ID3v2.3 and 2.4 TXXX, RVA2, APEv2 before an ID3v1 tag, FLAC's
Vorbis comments) in each mode, with a preamp, and with the peak holding
a +20 dB gain at full scale; and refuse a WMA file with a message saying
what plays. internal/tags' tests read those tags directly, broken ones
included. The shell's tests drive a fake engine through the same
`playback.Engine` contract (loads, seeks waiting for a load, superseded
generations, errors stopping audio, ReplayGain reaching the engine and
saved, unsupported files refused), and one plays a two-track queue
through the shell with the real engine on audiotest. Settings tests use
actual toolkit input and accessibility actions on the ReplayGain controls
at 1× and 1.75×, at the preferred and the minimum size.

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
repeated accessibility activation. A fake-engine regression proves
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
that prefers Move, and recursive queue additions with a fake engine
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
[SHAPE ordering defect](../goframework-gaps.md#carried-over-from-uitoolkit)
in resizable rounded windows; v0.23.4 fixes it and the native recheck passes.

A screenshot check should cover Default and all optional skins at 1x and
1.75x, with every separate player window included. The README links the
current default window and each skin's main-window illustration.

## Toolkit review at v0.23.21

The dependency is UIToolkit v0.23.21 with paintengine2d v0.12.2; other
dependencies are unchanged. All 566 non-test Go files except version.go
are byte-identical to .20. No player or skin migration is needed. Shipping
retains `theme_engine_all,font_fallback`; minimal builds keep `font_fallback`.
The player, minimal and Windows/macOS cross-builds compile, with shipping
vet passing. Cross-builds do not establish native desktop runtime. All
twenty-one manifests/eighty-two artwork PNGs are unchanged; all 102 posed
panels at 1×/1.75× match the actual .20 shipping executable.

The fresh full shipping application suite passes on unchanged app sources
and tests, including Shell, presentation/Desk and pack/generator checks.
The reporter's deliberately failing fixtures are checked separately for
correct diagnostic status.

The [owner report](../goframework-gaps.md#carried-over-from-uitoolkit) retains fifty-five entries. All five
earlier reporter modes pass before the new `-repeat` guard is added.
The new guard verifies .21 invocation isolation with real `-count=2` tests:
a second silent attempt keeps its own silence instead of inheriting the
first reason. A synchronized parallel control exercises CONT and NAME and
retains pre-pause/post-resume context without cursor-test leakage. The same
guard fails on actual .20 after earlier controls pass. All six modes pass
on .21. No new issue was found in these bounded changed-tool checks.

All 164 production style files match .16/.20/.21. The .20 fifteen-test font
race and nine targeted iTunes tests retain their demonstrated scope; they
are not fresh .21 runs. Earlier fallback/cache/Prefix/application-race and
native proof retain their original scope as well. No app source/test change,
provider override or font promotion is made. Gap 54's .17 held-clock
coverage fix is unchanged. Portable grouping requirement 9 remains open.

## Earlier toolkit review at v0.23.20

The dependency is UIToolkit v0.23.20 with paintengine2d v0.12.2; other
dependencies are unchanged. All 566 non-test Go files except version.go
are byte-identical to .19. No player or skin migration is needed. Shipping
retains `theme_engine_all,font_fallback`; minimal builds keep `font_fallback`.
The player, minimal and Windows/macOS cross-builds compile, with shipping
vet passing. Cross-builds do not establish native desktop runtime. All
twenty-one manifests/eighty-two artwork PNGs are unchanged; all 102 posed
panels at 1×/1.75× match the actual .19 shipping executable.

The full shipping application suite passes on unchanged app sources and
tests, including Shell, presentation/Desk and pack/generator checks. The
reporter's intentionally failing fixture is checked separately for correct
failure-status propagation.

The [owner report](../goframework-gaps.md#carried-over-from-uitoolkit) retains fifty-five entries. Gap 55
retains its .19 setup/cleanup/context/status fixes. A new bounded guard
verifies .20 package isolation: same-named child tests in two real packages
keep their own reason or silence, with package headings and flat test rows.
A single-package format control also passes. The same guard fails on actual
.19 after its earlier positive controls pass. All four earlier modes pass
on .20 before the new mode is added. No new issue was found in these checks.

The fifteen-test font race suite and nine targeted iTunes tests pass freshly
without skipped cases, including six editor/IME/keycap controls. All 164
production style files match .16/.19/.20. Earlier .16 plain/ordinary-optional
suites, broader application/fallback/cache races and native lifecycle proof
retain their original scope for unchanged paths. No app source/test change,
provider override or font promotion is made. Gap 54's .17 held-clock
coverage fix is unchanged. Portable grouping requirement 9 remains open.

## Earlier toolkit review at v0.23.19

The dependency is UIToolkit v0.23.19 with paintengine2d v0.12.2; other
dependencies are unchanged. All non-test Go files except version.go are
byte-identical to .18. No player or skin migration is needed. Shipping
retains `theme_engine_all,font_fallback`; minimal builds keep `font_fallback`.
The player, minimal and Windows/macOS cross-builds compile, with shipping
vet passing. Cross-builds do not establish native desktop runtime. All
twenty-one manifests/eighty-two artwork PNGs are unchanged; all 102 posed
panels at 1×/1.75× match the actual .18 shipping executable.

The fresh full shipping application suite passes on unchanged app sources
and tests. Diagnostic fixtures that deliberately fail are checked separately
for correct error-status reporting.

The [owner report](../goframework-gaps.md#carried-over-from-uitoolkit) retains fifty-five entries. Gap 55
is resolved for the recorded cases: setup and cleanup logging preserve the
actual Skip reason, while failure status stays intact. Additional bounded
controls verify full log context for untraceable/ambiguous source sites,
silent SkipNow and quiet no-Skip trees, including a failed invocation.
All controls pass; no new issue was found in the changed-tool checks.

The fresh fifteen-test font race suite and nine targeted iTunes tests pass
without skipped cases, including six strict editor/IME/keycap controls.
Earlier .16 full plain/ordinary-optional, broader application races and
native lifecycle proof retain their demonstrated scope for unchanged
paths. No source/test adaptation or font promotion is made. Gap 54's .17
held-clock coverage fix is unchanged. Portable grouping requirement 9
remains open.

## Earlier toolkit review at v0.23.18

The dependency is UIToolkit v0.23.18 with paintengine2d v0.12.2; other
dependencies are unchanged. Runtime/font/API sources are byte-identical to
.17 except version.go. No player or skin migration is needed. Shipping
keeps `theme_engine_all,font_fallback`; minimal builds keep `font_fallback`.
The player, minimal and Windows/macOS cross-builds compile; shipping vet
passes. Cross-builds do not establish native desktop runtime.

All twenty-one manifests and eighty-two artwork PNGs are unchanged; all
102 posed panels at 1×/1.75× match the actual .17 shipping executable.
The nine targeted iTunes editor/full/Mini tests pass with no skipped children,
covering six strict IME/keycap cases and the retained caret, CJK, metadata,
selector and colour/cache paths at both scales.

The fresh full shipping application suite passes on unchanged app sources
and tests. Its result is separate from the diagnostic cleanup assertion
that still reproduces gap 55.

The [owner report](../goframework-gaps.md#carried-over-from-uitoolkit) retains fifty-five entries. Gap 55
is partial: earlier setup logging is now handled and failed-run status is
preserved, but later cleanup logging still replaces the actual Skip reason.
The original probe passes, the bounded cleanup extension fails after its
positive controls, and the new failure-status control passes. This remains
a diagnostic issue with no demonstrated player/runtime impact.

The retained fifteen-test font race suite passes freshly without skips,
including exact source-PNG and pairwise flag artwork checks. Earlier .16
full plain/ordinary-optional, application races and native lifecycle proof
retain their original scope for unchanged paths. Gap 54's .17 held-clock
test-coverage fix is unchanged. No source/test adaptation or font promotion
is made. Portable grouping requirement 9 remains open.

## Earlier toolkit review at v0.23.17

The dependency is UIToolkit v0.23.17 with paintengine2d v0.12.2; other
dependencies are unchanged. All non-test Go files except version.go are
byte-identical to .16, so no application, artwork or API migration is
needed. Shipping keeps `theme_engine_all,font_fallback`; minimal builds
keep `font_fallback`. The player, minimal and Windows/macOS cross-builds
compile, with shipping vet passing. Cross-builds do not establish native
desktop runtime.

The fresh full shipping application suite passes. All twenty-one manifests
and eighty-two artwork PNGs are unchanged; all 102 posed panels at
1×/1.75× match the actual .16 shipping executable.

The [owner report](../goframework-gaps.md#carried-over-from-uitoolkit) retains fifty-five entries. Gap 54
is fixed: both maximized-turn scene children now pass the same SDK race
command without skipping. New 55 records a bounded diagnostic issue in
tools/skips.sh: an earlier setup Log is reported instead of the actual Skip
reason. It still detects nested child verdicts correctly. Runtime impact
is unproven; the player does not use either flip API or the SDK skip tool.

Fresh checks retain all fifteen strict font race tests with no skips and
the six actual iTunes editor/IME/keycap cases plus full/Mini metadata,
complex caret, CJK and selector/colour/cache paths. Earlier .16 full plain,
ordinary-optional, focused application races and native lifecycle checks
retain their demonstrated scope; they are not represented as new .17
runs. Shipping flags and app/test sources are unchanged. Portable grouping
requirement 9 remains open.

## Earlier toolkit review at v0.23.16

The dependency is UIToolkit v0.23.16 with paintengine2d v0.12.2; other
dependencies are unchanged. The only runtime change keeps terminated
emoji tag sequences with their picture base. No application API or skin
migration is needed. Shipping retains `theme_engine_all,font_fallback`;
minimal builds keep `font_fallback`. Linux, minimal and Windows/macOS
cross-builds compile, with shipping vet passing. Cross-builds do not
establish native desktop runtime.

All twenty-one manifests and eighty-two artwork PNGs are unchanged. All
102 posed panels at 1×/1.75× match the actual .15 shipping executable.

Full plain, shipping fallback and ordinary optional-emoji application
suites pass on unchanged app sources and tests. Focused shipping races
pass all eight selected packages, including all four no-pulse Desk fixtures.
Guarded optional-font editor races pass separately. Existing editor/metadata
controls pass at both scales, including six strict IME/keycap cases,
full/Mini search, RTL caret, default scripts, CJK, selectors and colour/cache
behavior. These application passes are distinct from the SDK coverage
issue below.

The [owner report](../goframework-gaps.md#carried-over-from-uitoolkit) retains fifty-four entries. Gap 53
is fixed for the recorded source-supported flags: England/Scotland/Wales
no longer paint the black flag. The original strict guard and a stronger
independently decoded source-PNG/pairwise-country check pass .16 and fail
actual .15. Eight bounded malformed-tag/text-selector controls pass both
versions. All fifteen strict raw optional-font tests pass under the race
detector without skips; earlier font fixes retain their demonstrated scope.
Shipping font configuration is unchanged.

New 54 records a verification-coverage issue in the toolkit itself: both
maximized-flip scene children skip under the race detector while their
parent reports PASS. The separate held-clock direction check newly made
strict in .16 passes without a skip. This is not a demonstrated player
runtime failure; the player uses neither flip API. Public Flip clocks and
no-pulse rack deadline checks still pass. Portable grouping requirement 9,
tray contract 23 and earlier native lifecycle proof retain their scope.

## Earlier toolkit review at v0.23.15

The dependency is UIToolkit v0.23.15 with paintengine2d v0.12.2; other
dependencies are unchanged. No player API migration is needed. Shipping
keeps `theme_engine_all,font_fallback` and minimal builds keep
`font_fallback`. All 102 posed panels at 1×/1.75× match the actual .14
executable, and all twenty-one manifests/eighty-two artwork PNGs are
unchanged. Linux, minimal and Windows/macOS cross-builds compile, and
shipping vet passes. Cross-builds do not establish native desktop runtime.

Full plain, shipping fallback and ordinary optional-emoji application
suites pass. Focused shipping races cover all eight selected packages,
including all four unchanged no-pulse Desk fixtures; guarded optional-font
editor races pass separately. No app compatibility change, test scheduling
change or relaxed assertion is needed for this release.

The [owner report](../goframework-gaps.md#carried-over-from-uitoolkit) retains fifty-three entries.
Gap 52 is repaired for the recorded installed-source BMP emoji: bare
watch, lightning and star now match the colour-source/VS16 advance and
pixels. Gap 51's verified repair now also covers the release's alchemical,
chess and squared-text samples inside the old coarse range, with the
report retaining its alchemical source-raster qualification. Gap 53
remains open: England/Scotland/Wales tag flags still paint
the black flag. The unchanged strict artwork guard continues to fail
despite correct width/count/tint. No full-font promotion or application
substitution is introduced. Actual iTunes IME/keycap/source-search and
Shell metadata/script/selector checks pass at both scales.

The toolkit's new property tables replace the range heuristic used to
mark default emoji, while Extended_Pictographic separately handles joiner
context. Its public FlipPanel clock now reaches a framed Window.Flip's
temporary panel; a public probe fails actual .14 and passes .15 under the
race detector. The player uses neither flip API, so no app adaptation is
needed. Rack source is unchanged and retained no-pulse deadline controls
pass. Portable grouping requirement 9, tray contract 23 and earlier
native lifecycle proof retain their existing scope.

## Earlier toolkit review at v0.23.14

The dependency is UIToolkit v0.23.14 with paintengine2d v0.12.2; other
dependencies and shipping `theme_engine_all,font_fallback` remain unchanged.
Minimal builds keep `font_fallback`. All 102 posed panels at 1×/1.75×
match the actual .13 executable, and all twenty-one manifests/eighty-two
artwork PNGs are unchanged. The player, minimal variant and Windows/macOS
cross-builds compile. Cross-builds do not establish native desktop runtime.

Full plain, shipping and ordinary optional-emoji application suites and
shipping vet pass. Focused shipping races pass after correcting only a
test-clock/pump ordering issue in the four-family Desk fixture; the
identical corrected fixture still fails actual .13. Separate optional-font
races cover the actual editor composition and plain-text keycap controls.
The [verification details](../goframework-gaps.md#carried-over-from-uitoolkit)
distinguish the first instrumented fixture failure from the final passes.

The [owner report](../goframework-gaps.md#carried-over-from-uitoolkit) retains fifty-three entries.
Every recorded pending-move timer case in 10 now passes, including the
staggered earlier-deadline case that actual .13 still misses. The app
removes its pending flag, Desk.Moved observer and pending-only follow
branch in Winamp, Fandom, Half Life 2 and Fancy. Explicit Follow for
startup/restoration, resize/capture and non-notifying backends remains.
Actual player fixtures now require toolkit timers alone to settle the
requests, without Pulse/Pending hooks; the unchanged new Fandom fixture
fails actual .13 and passes .14. No atomic native move or drag/compositor
smoothness guarantee follows from these headless scheduling checks.

Plain-text keycap regression 50 is repaired in strict raw Font checks and
the actual iTunes source editor at both scales. Exact IME text, caret,
callback count, filter, paused playback and text-tinted pixels are verified;
the same editor diagnostic fails actual .13. Earlier positive composition,
script/title, selector, prefix and race controls retain their verified scope.
New 51 records a verified supplementary-text mixing fix already present
in .14. New 52 records the documented pre-existing default-presentation
limitation for some bare BMP emoji. New 53 is a .14 regression: supported
England/Scotland/Wales tag flags paint the same black flag. The final
strict raw optional-font suite fails solely this artwork check, despite
correct width/count/tint. No full-font promotion or provider workaround
is introduced.

New public FlipPanel.SetClock passes independent rendered-turn/callback
race checks; the app has no FlipPanel usage to migrate. Portable grouping
requirement 9 and resolved tray contract 23 remain unchanged. Fresh raw
rack/tray/primitive checks pass. Native key/capability/GPU-close sources
retain earlier proof without a fresh native desktop/GPU claim.

## Earlier toolkit review at v0.23.13

The dependency is UIToolkit v0.23.13 with paintengine2d v0.12.2; other
dependencies are unchanged. Shipping keeps `theme_engine_all,font_fallback`
and minimal builds keep `font_fallback`. No application API or artwork
migration is needed. All 102 posed panels at 1×/1.75× match the actual
.12 executable, and all twenty-one manifests/eighty-two artwork PNGs are
unchanged. The player, minimal variant and Windows/macOS cross-builds
compile. Full plain, shipping and ordinary optional-emoji application
suites, vet and focused shipping races pass; the latter include all four
panel families' actual Desk fixtures. Cross-builds do not establish native
Windows/macOS runtime.

The [owner report](../goframework-gaps.md#carried-over-from-uitoolkit) retains fifty entries. The old
shared-outline race in 47 and recorded qualified/no-VS16 keycaps in 48 now
pass unchanged controls. All five actual IME composition diagnostics pass.
New 49 records another independently verified cache-group race already
fixed by this release: public cached measurement/drawing during atlas
growth fails the actual .12 no-tag baseline and passes .13 no-tag,
fallback, complex and emoji controls, with stable pixels/width/snapshots.

New 50 is a .13 optional emoji regression. A generic enclosing keycap mark
after plain A or an explicitly text-selected digit becomes an untinted
colour cell; the leading letter/digit remains correctly tintable. Actual
.12 negative controls pass. The final strict raw Font suite fails only
this guard, despite the ordinary application suite passing. The player
keeps its current font configuration without a substitution or provider
override; earlier font and metadata fixes retain their verified scope.

All original lost-wake paths in 10 are repaired, including direct requests
followed by satellite closure. A pre-existing staggered-request delay
remains: a later request postpones an earlier deadline. The app keeps its
narrow pulse, with corrected comments and an actual fixed-clock test
across all four panel families. That test reconciles the expired first
request while preserving the later request's own patience. No semantic
production workaround is removed based only on the repaired closure paths.

Portable grouping requirement 9 and resolved tray contract 23 are unchanged.
Fresh raw rack/tray/primitive checks agree. Native key/capability and GPU
lifecycle code is unchanged, preserving earlier proof without a new native
desktop or human-drag smoothness claim.

## Earlier toolkit review at v0.23.12

The dependency is UIToolkit v0.23.12 with paintengine2d v0.12.2; the other
dependencies are unchanged. No application API or artwork migration is
needed. Shipping remains `theme_engine_all,font_fallback`; minimal builds
keep `font_fallback`. All 102 posed panels at 1×/1.75× match the actual
.11 build byte for byte, and all twenty-one manifests/eighty-two artwork
PNGs are unchanged. Full plain/shipping application suites and focused
shipping races pass. The player, minimal variant and Windows/macOS
cross-builds compile; cross-compilation does not establish native runtime.

The [owner report](../goframework-gaps.md#carried-over-from-uitoolkit) keeps forty-eight entries. The
qualified keycap cases in 48 now match one source-shaped glyph's advance,
untinted pixels and one painted cell. Seven supported qualified heart-
couple/kiss/BMP gender/profession sequences also pass against an actual
.11 negative baseline. Public Device.Blit counts expose zero-advance
overlays that width alone can miss. Actual iTunes family/rainbow/qualified
keycap/couple search checks pass with strict event and paused-state checks.

48 remains partial: no-VS16 `1⃣` still paints two cells with excessive
width and text tint, including in actual search. 47 still races on shared
outline scratch storage in a fresh focused run; its source is unchanged.
The complete optional suite fails the no-VS16 assertion even where its
concurrent test happens to run cleanly. No full-font flag promotion,
provider override or composition workaround is added. Earlier script,
selector, width, atlas and longest-prefix fixes retain their verified scope.

10's original wake cases remain repaired, while the direct-request
closing-satellite paths still fail and retain the narrow application pulse.
Portable grouping 9 and clarified tray contract 23 are unchanged. Fresh
raw rack/tray/primitive checks agree, while unchanged native key/capability
and GPU lifecycle code retains earlier proof without a fresh native claim.
No additional numbered issue is established; remaining cases and fixed
qualified forms are saved under the existing entries.

## Earlier toolkit review at v0.23.11

The dependency is UIToolkit v0.23.11 with paintengine2d v0.12.2; the other
dependencies are unchanged. No application API migration or artwork
regeneration is required. Shipping keeps `theme_engine_all,font_fallback`
and the minimal build keeps `font_fallback`. All 102 posed panels at
1×/1.75× match the actual .10 executable byte for byte; all twenty-one
manifests and eighty-two artwork PNGs are unchanged. Full plain/shipping
suites, vet and affected shipping race checks pass. The player and minimal
variant build, as do Windows/macOS cross-builds; native runtime on those
platforms is not established by cross-compilation.

The [owner report](../goframework-gaps.md#carried-over-from-uitoolkit) retains forty-eight entries. The
family-emoji regression (45) and longest-prefix omission (46) pass their
original raw/application controls. Independent installed-source shaping
also verifies rainbow/profession composition. The original occurrence,
text-selector, returned-width and colour-parser concurrency fixes remain
verified; default script providers and actual metadata/search paths pass.

New 47 records a pre-existing shared outline-buffer race during concurrent
shaped-font drawing. New 48 records supported keycaps that remain split,
with incorrect width and text tint, despite the release claim. Both
reproduce on actual .9/.10/.11 graphs. The complete strict optional-font
suite fails these two checks; a separate colour-only concurrency control
passes. Existing assertions remain strict, and actual iTunes family and
rainbow IME/search checks pass separately from the failing keycap diagnostic.
No application font workaround or full-font flag promotion is added.

The original movement-wake exceptions in 10 now pass, but direct Apply
or sequential Show requests still lose their only wake when its satellite
closes without an event-driven follow pass. The app retains its narrow
pending-geometry pulse, with corrected comments about that remaining path.
The explicit tray-name contract clarification in 23 is resolved and real
private-bus payload/lifetime checks agree. Portable grouping requirement 9
remains unchanged. GPU lifecycle and native key/capability sources are
unchanged; earlier native proofs remain scoped to their original runs.

## Earlier toolkit review at v0.23.10

The dependency is UIToolkit v0.23.10 with paintengine2d v0.12.2; other
versions are unchanged. No application API migration or asset regeneration
is required. The player keeps `theme_engine_all,font_fallback`, and the
minimal build keeps `font_fallback`. All 102 posed panels at 1×/1.75× match
the actual previous shipping build byte for byte. The twenty-one manifests
and eighty-two artwork PNGs are unchanged.

The [owner report](../goframework-gaps.md#carried-over-from-uitoolkit) now retains forty-six entries.
The original occurrence/FE0E/VS16 cases (42), returned-string width bounds
(43), and concurrent emoji-face race (44) now pass. The default provider
list now covers the tested installed Arabic/Hebrew/Devanagari/Thai fonts;
actual default-provider metadata checks pass across all twenty-two
appearances and both scales without an application override. iTunes
full/compact/search caret, editing, selector and whitespace controls pass.

Two issues keep the full optional font configuration out of the player
build. New 45 records a .10 regression: previously composed family emoji
become four pictures; unchanged raw and actual-search controls compare
the real .9/.10 graphs. New 46 records pre-existing longest-prefix
omission despite a fitting independently measured Arabic character.
Thus 41 remains partial through 45, and 39 remains partial through 46.
Passing width bounds and concurrency do not certify the complete optional
font suite; the joined-family assertion still fails without being weakened.

Digit-row/keypad accelerator and cloud/badge primitives pass public checks.
An owned native X11 window verifies Ctrl+0–9 row/keypad delivery and
retained shortcuts without changing host input. The app uses typed key
constants and needs no enum-value migration. Grouping cases 9/10 are
unchanged and retain the pending-geometry pulse. GPU lifecycle source is
unchanged, preserving the earlier actual native 36/37 proof without
claiming a fresh .10 GPU test.

## Earlier toolkit review at v0.23.9

The dependency is UIToolkit v0.23.9 with paintengine2d v0.12.2; the other
dependencies are unchanged. No application API migration or asset
regeneration was required. The player keeps `theme_engine_all,font_fallback`
and the minimal build keeps `font_fallback`. Every one of the 102 posed
panels at 1×/1.75× is byte-identical to the previous shipping executable;
the twenty-one manifests and eighty-two artwork PNGs remain unchanged.

The [owner report](../goframework-gaps.md#carried-over-from-uitoolkit) now retains forty-four entries.
Actual native .8/.9 comparisons verify 36/37: normal Window.Close dispatches
GPU cleanup once with its own EGL context during pumping, after Run, and
after another Application is created. Repeated close produces no duplicate.
The grouping code and remaining cases in 9/10 are unchanged; the app keeps
its pending-geometry pulse.

The original complex-script lender defect (38), emoji snapshot identity
defect (40), and skin-tone composition defect (41) now pass. Actual
Arabic/Hebrew metadata and colour emoji regressions cover all twenty-two
appearances at both scales; iTunes full/compact/search caret, selection,
IME and sequence checks pass. Script tests explicitly name covering
installed Noto providers; default-list coverage for every installed script
font is not claimed. These are normally compiled optional-font regressions,
replacing the former failing custom-tag diagnostic.

The optional tags remain outside the shipping build. New 42 records emoji
presentation leaking between equal runes and overriding a text selector;
43 records returned contextual prefixes/fitted strings exceeding their
width; 44 records shared emoji-face races between independent fonts.
Item 39's original Hebrew caret/wrap cases pass, but its general fitting
contract remains partial through 43. Both contextual fitting and shared
emoji-face mutation predate .9. Raw public probes retain the remaining
negative assertions separately from passing shipping checks.

## Earlier toolkit review at v0.23.8

The dependency is UIToolkit v0.23.8 with paintengine2d v0.12.2. Its new
typesetting dependency also updates x/image/x/text. The player keeps
`theme_engine_all,font_fallback` and the README's minimal build keeps
`font_fallback`. All twenty-one manifests and eighty-two artwork PNGs are
unchanged; all 102 posed panels at 1×/1.75× are byte-identical to the actual
previous shipping build. No application API migration or rendering
workaround was needed.

The [owner report](../goframework-gaps.md#carried-over-from-uitoolkit) retains forty-one entries. Runtime
fallback-provider replacement (35) is now fixed for both cached hits and
misses, old/new strings, and measurements. Startup CJK rendering (34)
remains correct with a covering installed font. The grouping code and
remaining cases in 9/10 are unchanged; the player retains its small
pending-geometry pulse.

Per-surface GPU-close registration corrects the constructor-routing cause
of 36, but normal Window.Close unregisters before native teardown. New
entry 37 preserves actual .7/.8 native GPU comparisons and the diagnostic
Surface.Close-first control. The player has no custom GPU-close consumer.

New complex/emoji tags imply fallback and compile without application API
changes. They are reviewed but remain outside the player build: 38 records
complex-script lender exclusion; 39 records RTL caret/prefix/wrap failures
with real primary Hebrew glyphs; 40 records lost colour identity after an
emoji atlas grows; 41 records skin-tone composition split across faces.
Installed source-font controls and raw public reproductions accompany all
four. Existing per-glyph fallback still does not supply shaping/bidi, and
original Winamp bitmap readouts keep their source-art alphabet.

## Earlier toolkit review at v0.23.7

The dependency is UIToolkit v0.23.7 with paintengine2d v0.12.2. The rebuilt
player enables `theme_engine_all,font_fallback`; the README's minimal build
also enables `font_fallback`. With a covering font installed, native-font
song/artist labels borrow missing glyphs. No CJK font is bundled, and
`GOFRAMEWORK_DESKTOP_UI_SYSTEM_FONTS=0` (`UITK_SYSTEM_FONTS` then) disables installed-font lookup. Original Winamp bitmap
readouts keep their source-art alphabet; fallback is not bidi or shaping.

The [owner report](../goframework-gaps.md#carried-over-from-uitoolkit) now retains thirty-six entries.
The demonstrated CJK title collapse (34) is fixed in the tagged build,
including same-row title updates across all twenty-two appearances and
iTunes full/compact/search controls. The manifests and all artwork are
unchanged; all 102 posed panels at 1×/1.75× are byte-identical to the prior
build. Existing grouping gaps 9/10 remain unchanged with the same pulse
fallback.

New entry 35 reproduces stale provider/measurement caches after changing
fallback families. The player keeps its startup provider list. Entry 36
reproduces a native GPU closing-callback regression when another Application
is constructed; the player's single Application is unaffected. Single-App
own-context, pumping and post-Run GPU-close controls pass. These findings
retain raw public reproductions and baseline comparisons.

## Earlier toolkit review at v0.23.6

The dependency is UIToolkit v0.23.6 with paintengine2d v0.12.2. The
[report for the toolkit owner](../goframework-gaps.md#carried-over-from-uitoolkit) retains thirty-four
numbered findings, including the earlier source and reproduction history.
Public probes confirm that Main rejoin/link preservation (7), font-preference
authoring (17) and interrupted list gestures (29) are now fixed.

The player uses the new ButtonDraw API for its empty-label custom controls
and explicitly follows desktop reduced-motion preferences to preserve its
previous behavior. Native Desk callbacks now provide move batching; the four
panel families remove their former rejoin and eager-follow safeguards.
Startup/session placement and a minimal pending-geometry pulse remain app
policy. The font JSON bridge is gone: all twenty-one manifests are emitted
directly by skingen, with identical decoded contents and all eighty-two
artwork PNGs unchanged. Across twenty-two appearances at 1×/1.75×, all 102
posed panels remain byte-identical between the actual old and new toolkit/
paintengine pairs.

Portable grouping (9) remains declined. Pending-move reconciliation (10)
works for ordinary Apply and event bursts, but refused re-show and closing
the timer-owning window still need the retained fallback. Newly recorded
item 34 documents a pre-existing missing-glyph fallback limitation: distinct
CJK song titles can paint as identical boxes despite an installed covering
font. Raw probes reproduce it in both v0.23.5 and v0.23.6; it is separate
from the fixed font-authoring API. Entry 23 still requests a minor explicit
tray-icon-name documentation clarification. The earlier macOS attribution
remains withdrawn, with no local macOS runtime claim.

## Earlier toolkit review at v0.23.5

At that review, the [report for the toolkit owner](../goframework-gaps.md#carried-over-from-uitoolkit)
retained thirty-three items with independent public probes, expected and actual
behavior, severity and versioned source references. The dependency was
UIToolkit v0.23.5 with paintengine2d v0.12.0. Application safeguards are not
counted as proof of a toolkit fix.

Nine previously outstanding defects now pass independent checks: raster-only
tray icon priority, fractional resized panel slices, popup focus scrolling,
mixed-sheet sampling serialization, horizontal list hit testing, ownership
refresh after parent recreation, cached GPU sampling changes, proportional
placeholder fonts and MenuBar sprite dispatch. Earlier registration,
replacement, callback, accessibility, geometry and utility-window fixes
also remain verified. The macOS move-reversion attribution was withdrawn
upstream after the unchanged old version passed on the same machine; neither
a local macOS runtime result nor a verified fix is claimed.

The app now delegates VLC menu-bar painting and iTunes/Amarok placeholder
painting to the toolkit, supplies its tray raster without the unresolved
icon-name workaround, and removes the mixed-sheet JSON repair. The manifest
bridge carries font preferences only. Twenty regenerated manifests preserve
effective sampling and every other field; all eighty artwork PNGs and the
preserved source archives are unchanged. All ninety-six captured panels in
the plain-build headless matrix across twenty-one appearances at 1×/1.75×
are pixel-identical before removing the workarounds. That comparison
isolates UIToolkit by holding paintengine2d at v0.12.0 in both versions;
the GPU sampling check separately compares paintengine2d v0.11.0/v0.12.0.
After cleanup, only Amarok's two placeholder captures change to the stock
muted gray, with the same font metrics and bounds. A separate all-engine
comparison uses the actual former v0.23.4/v0.11.0 pair with its old app
painters: VLC and both iTunes panes stay pixel-identical; Amarok changes
only within that placeholder. The ordinary plain/all-engine font difference
predates this upgrade because the optional Win95 base is absent in plain
builds. The gap report records the final tests, race checks and cross-builds.

Five entries remain open or partial: longer Rack rejoin paths and stationary
snap geometry (7), a portable grouping fallback (9), movement batching and
expiry wake-up (10), font-preference authoring (17), and interrupted
list double-click history (29). The remaining list reproduction is a
click → wheel → click where the wheel cannot move the list: the second
click still activates. Actual scrolling and the other tested interruption
boundaries now reset click history. Entry 23 separately requests a minor
clarification of explicit tray icon names versus raster priority.

The app retains its every-wheel/model-remapping gesture policy, guarded Rack
following, defensive native-owner refresh and explicit viewport guards.
Those retained guards do not keep independently fixed items open. Portable
logical-window grouping remains a declined requirement: ordinary Wayland
windows render without XWayland, but arbitrary separate-window placement and
taskbar requests are unavailable on the tested native backend. The temporary
X11 preference remains. Programmatic native checks do not establish continuous
human drag smoothness.
