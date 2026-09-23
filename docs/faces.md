# The three faces

One player, three ways of showing it. This is the long form: what each face
is for, how the skins place their controls, what a desktop may refuse to do,
how the pieces fit together and what is tested.

Most of this began life as uitoolkit's `docs/players.md`, where the three
faces were three separate demo applications. What was the toolkit's stayed
there; what was the application's is here.

## They play nothing

**Nothing here is decoded and nothing is played.** There is no audio stack,
no video stack and no media dependency of any kind in this repository or in
the toolkit under it, and there is not going to be one — adding PipeWire,
ALSA or FFmpeg to make a demo honest would make the project dishonest
instead.

So the model underneath is a clock that counts, a list of invented tracks, a
band of numbers that move the way an analyser does, and an equaliser whose
ten faders filter nothing. Every face says so where it cannot be missed:
Minim in its title bar and its playlist's footer, Marquee on its display,
Lantern in its status bar and its About box, and every menu row that would
need a media stack if you picked it says exactly that.

Every title, artist and album in the invented library was made up for
`internal/players/players.go`. No skin, bitmap, icon, font, name or mark
belonging to any real player is in this repository, as art or as a fixture.
The skins are uitoolkit's, generated from paths and whole pixels by its
public `skingen` package; the transport glyphs are drawn from paths in
`internal/players/ui.go`; the display alphabets and clock digits the panels
print in were set by hand on their grids for the toolkit.

## One model, three faces

The application owns the model: the transport, the queue, the equaliser and
the analyser (`players.Model`). A face is handed it — never its own copy —
through `players.Host`, which is also how a face reaches the few things only
the application can do: run a transport command with the clock behind it,
answer the keys every face shares, change the skin, change the face, and
offer both in a menu.

That is what makes a face switch a change of face rather than a restart.
`shell.Shell.SetFace` opens the new face's windows **before** closing the
old face's, for two reasons: the application ends when its last window goes,
and a player that blinked out of existence between two faces would not be a
player changing its face. Nothing of the model is touched on the way
through.

There is **one clock for the whole application**, not one per face and not
one per window (`players.Pulse`): three windows waking separately would
advance the same transport three times a frame and drop the analyser's peak
markers three times as fast. A wake delayed past a few periods is clamped,
so a machine that was asleep does not come back to a row of bars with no
markers on them.

## Minim — the compact face

```bash
go run . -face minim
go run . -face minim -only strip           # the strip on its own
go run . -face minim -skin breeze-night    # the same face, no skin
go run . -face minim -scale 1.75
```

A strip with a phosphor display, a seek bar and a transport row, plus an
equaliser and a playlist that **snap flush to its edges and travel with it**.
Drag any of the three within ten pixels of another's edge and it sticks;
drag the strip and the stack follows; drag one out of the middle and it
takes whatever hangs from it and leaves what it hung from.

275 by 116 is what this shape was at 1×, and here it is a *design* size
rather than a frozen one: at 1.75 the window is 481 by 203, the silhouette
is redrawn at that size rather than stretched, and the art is the 2× sheet
drawn at a whole multiple because the skin declares itself `pixelated`.

### Three skins, one widget tree

```bash
go run . -skin minim-classic   # the base-skin look of the era
go run . -skin minim-silver    # the rounded silver-and-blue one
```

- **`minim`**, its own: a pixel front panel dressing ordinary widgets.
- **`minim-classic`**: slate-blue bevelled chrome, title bands with a gold
  groove either side of the words, a black display with thin green segments
  and a small green analyser, an orange volume bar and a green balance bar,
  grey transport keys, and an equaliser of eleven yellow faders. A pixel
  skin, exact at 1× and 2×.
- **`minim-silver`**: silver chrome under a navy title band, a blue
  dot-matrix display with big pixel digits, glossy round keys that brighten
  under the pointer, capsule toggles beside blue lamps, an equaliser whose
  header is a tab with its name on it and no title band at all, and windows
  whose four corners are round with the desktop showing beyond them. Drawn
  from paths, exact from 1 to 2.

The last two are *panels*: pictures of each whole window with holes where
the keys go, which is how a player of this shape was built. **Each skin says
where its holes are.** It states three fixed layouts — `minim.strip`,
`minim.equaliser`, `minim.playlist` — whose named slots the player binds its
controls to with `widget.Slots`. The face carries no rect of its own for
either panel; it reads every one it uses from the skin it is wearing,
including the ones it prints into: the clock's digits, the lamps, the
scrolling title, the playlist's rows. The rects are generated from
uitoolkit's `skingen/panel`, the same numbers the art is drawn round, so the
art and the layout cannot disagree about where a key is — and a panel
somebody else draws for this face puts its keys wherever its own art has
them.

A key's look in every state is its slot's art (`style.DrawSkinSlot`); the
digits, lamps, thumbs and bitmap capitals are sprites the face paints by
name (`style.DrawSkinSprite`). Each window is given a role — the name of its
layout, `app.Window.SetFrameRole` — and that is how Minim Silver's equaliser
gets its tab: the skin states a caption for the `minim.equaliser` role and
the other two windows keep the navy band. The windows keep the player's
titles in every skin; the panels print them in capitals because their
caption text roles say `"case": "upper"`.

The widget tree is the **same one in every skin**. A switch moves and
repaints the controls and hides the few one face has and another does not:
the panels have their own pause key, an eject key and a balance slider, and
no mute key. The focus, the tab order and the accessibility tree carry
across a switch, and a control that disappears hands the keyboard to the
obvious next one rather than to nothing.

The switch is reachable every way a control should be:

| | |
| --- | --- |
| the skin key | in the strip's own chrome: the column of option letters in the panels (it reads SKIN), the last toggle in Minim's own row. It steps to the next skin, and its accessible name says which one it is on |
| **Ctrl+K** | from any of the three windows: minim → minim-classic → minim-silver → minim |
| **Ctrl+Shift+K** | drops the skin for `breeze-night` and puts back the one it dropped |
| a menu | a right-click anywhere on the face of any window, or the menu key (Shift+F10), lists the skins by name with the one being worn ticked, and "No skin" |

The panels' extra keys do what is true in a player that plays nothing.
Eject stops and goes back to the first track. AUTO chooses a preset for each
track as it comes up. PRESETS lists the presets. MISC opens the application
menu — the faces, and this face's skins. SEL shows the playing track. ADD
and REM say plainly that there is nothing to add or remove. LIST OPTS jumps
to the first track or closes the list.

## Marquee — the face that folds

```bash
go run . -face marquee
go run . -face marquee -compact            # open folded
```

A cabinet with a display, a queue and a curved transport shelf. Ctrl+M, or
the button at the right-hand end of the shelf, folds it into a flat stadium
and back.

The fold is the only place in the application where an **app** sets a
silhouette of its own. A skin declares one window shape — here the brow
across the top and the dome under the cabinet — and the stadium is the
app's, set with `SetShapeFunc`; the toolkit gives the app's shape precedence
over the look's exactly so that a window which knows it is a different shape
can say so. Passing `nil` hands the window back to its look.

It keeps **one widget tree** for both shapes rather than swapping two. That
is the behaviour and not a saving: the focus stays where it was, the
accessibility nodes keep their ids, and a control the keyboard was on is
still the control the keyboard is on after the window has changed size and
outline. Folding hides what does not fit, which takes those controls out of
the tab order and out of the accessibility tree — which is what "not in this
mode" has to mean.

A right-click anywhere on the cabinet's face opens the application menu.

## Lantern — the face that drops its skin

```bash
go run . -face lantern
go run . -face lantern -skin breeze-night  # start with the skin dropped
```

A main window cut to its skin's outline, with a playlist window anchored to
its right-hand edge. **Ctrl+K drops the skin**, and that is the argument of
the whole feature in four lines:

```go
ap := style.LookAppearance(app.Look())
ap.Name = "breeze-night"          // or "lantern"
app.SetLook(style.WithAppearance(app.Look(), ap))
```

It is the call Settings makes for any theme. What comes back is the same
widget tree in another look — the same tab order, the same accessibility
tree, the same keyboard — and a window that is a rectangle again, because
the outline belonged to the skin and never to the app.
`TestDroppingTheSkinKeepsTheApp` pins exactly that: the tab order compared
as a cycle, the tree compared by role and by every control's name, and the
one thing that is *supposed* to differ — the status line, which names the
pack — asserted separately.

Its skin is deliberately partial. It binds seventeen parts and leaves tabs,
splitters, switches and tree disclosures to `breeze-night`, because a skin
is always a partial override and a demo that only ever showed a complete one
would be hiding the common case.

## The keyboard

One table for every face, so learning the keys in one is learning them in
all (`players.CommandFor`):

```
Space  play / pause          ←  →   seek five seconds
Z  B   previous / next       ↑  ↓   volume
V      stop                  M      mute
S      shuffle               R      repeat: off → all → one
Escape quit                  Ctrl+Q quit
```

The application adds Ctrl+F (next face), Ctrl+Shift+F (the face before),
Ctrl+K (next skin) and Ctrl+Shift+K (drop the skin). Each face adds its own:
Minim the menu key, Marquee Ctrl+M, Lantern Ctrl+L.

Bare letters as shortcuts are safe here because no face hands one to a text
field. There is exactly one field in the three — Lantern's playlist filter —
and `players.Typing` asks the focused component what role it gives the
accessibility tree before any letter is dispatched. Typing "Beacon" into the
filter does not skip six tracks.

Every control is on the keyboard, including the ones drawn as glyphs:
`players.GlyphButton` is a `widgets.ToolButton` — a name, a focus ring, an
accessibility node, Return and Space — that happens to be painted with a
mark on it, or with a skin's art.

## The one thing a desktop may refuse

The snapping is not this application's: it is uitoolkit's `rack` package,
which any application can use for satellite windows of its own.

Windows that stick to one another need two things of a desktop: being told
where a window is, and being able to put one somewhere. **X11 has both** —
clients place their own windows — and so does the offscreen backend, which
is why every snapping test runs without a compositor.

**A Wayland toplevel has no position.** A client is never told where its
windows are and cannot ask for one; that is the protocol, not an omission in
the toolkit. So on Wayland the arithmetic still runs — the rack snaps, the
model is the same — and the windows simply cannot follow. The faces say
which of the two they got rather than pretending: Minim in its strip,
Lantern in its status line ("playlist not placeable here").

Two more things only real hardware showed, both about *when* rather than
*what*:

- **A window manager places a window when it maps it**, and the application
  is told afterwards — so a rack built when its windows are made is built
  around windows that are all still at the origin. `rack.Desk.Adopt` takes
  the real boxes in first, and each face attaches its panes once that
  answers yes.
- **A move is a request.** X11 answers one a frame or two later, so reading
  the position straight back says the window is where it was and looks
  exactly like a drag. `rack.Desk` waits for a move it asked for to arrive
  and gives up after a few looks, so a window manager that refuses one —
  KWin clamps a window that would hang off the screen — is not argued with
  forever.

## How it is put together

```
main.go                      the one command: -face, -skin, -only, -compact, -scale, -shot
internal/shell/              the application: the Face abstraction, the switch, the clock
  face.go                    what a face is, and the three that are
  shell.go                   one model, one clock, the shared keyboard, what is remembered
internal/players/            the model, and the pieces of interface every face shares
  players.go  spectrum.go    the transport, the playlist, the analyser, the equaliser
  host.go                    what a face is given, and the skin and face switching
  settings.go                the face and the skins, remembered between runs
  ui.go                      the glyphs, the transport key, the fader, the pulse, the keys
  minim/  marquee/  lantern/ one package per face: its look and its layout, and no more
  playertest/                what the faces' tests do to a window
```

The line between the shared package and each face is worth stating:
**anything a player of any era would have drawn the same way** is shared —
the transport marks, the analyser, the clock that drives them, the keyboard
— and everything about *how a particular player looked* is its own, because
that is the subject of the demo and sharing it would be sharing the thing
being demonstrated.

Three pieces are worth knowing about.

**`players.GlyphButton`** is a `widgets.ToolButton` with a mark painted on
its face, a label, and a way in for a skin that draws the key as a picture
of its own. It used to be a component of its own — two hundred and forty
lines of presses, keys, focus, hover and accessibility that a button already
had — because a button could not be painted by its app.
`widgets.ButtonPainter` and `widgets.ButtonShaper` are what it was waiting
for.

**`players.Fader`** is a `widgets.Slider`, stood on end by `Vertical` and
painted by a skin through `SliderPainter` where the art is a thumb riding a
printed groove. What is left on top of it is what is genuinely the app's:
the label, the reading a screen reader says out loud ("+4.5 dB", not "4.5"),
a step and a page of the app's choosing, Home meaning the top of a control
that stands on end, and the increment and decrement a screen reader can ask
for, which the slider advertises and does not itself answer.

**`internal/players/minim/tracks.go`** is the one component here that is not
a stock widget, and it says why in its own comment.
`widgets.ListView.RowGeo` takes everything it needs about *where* the parts
are from the same skin slots it uses, and if geometry were all of it that
file would be a hook and nothing else. What a list view has no hook for is
what a row is *painted* in: rows go through `LookAndFeel.DrawListRow`, a
skin passes that to the pack underneath it, and the panel skins are drawn
over `win95` — so a stock list inside the panel's printed well paints win95
rows in the look's font instead of eleven-pixel green on black. Three things
would retire the file: a row painter (or a row font and ink a layout can
state), a second column so a track's length can sit at the right-hand end of
its row, and a scroll thumb painted from the skin's own `list.thumb` sprite.

## The analyser, and why a screenshot is reproducible

The bars are a pure function of the transport's position: a few sines at
incommensurable rates per band under a falling tilt, and nothing else. Two
consequences, both deliberate.

A screenshot of the player posed at 1:37 (`-shot`) is the *same picture*
every time it is taken, so one run can be compared with the one before it.
And the same second of playing looks the same whether the window repainted
twelve times in it or sixty, which matters because the faces run at
different rates and all of them run slower under a screen recorder.

Only the peak markers and the fall to silence have memory. The scrolling
title takes its phase from the same position and slides back and forth with
a rest at each end rather than looping: a loop is what this era did and is
one line of arithmetic, but two thirds of every still frame of one is a
title cut in half with the next copy beside it.

## Tests

```bash
tools/testenv.sh go test ./...
```

The model is tested without opening a window at all — the transport's wrap
at the end of a track, the seek bar's fraction, the shuffle that is a
permutation beside the list rather than of it.

Each face is then tested headless for the four things a skinned app has to
keep: every control on the keyboard, `a11y.Check` clean in the skin *and*
with the skin dropped, the silhouette at 1, 1.25, 1.5, 1.75 and 2, and a
themed fallback that is a whole app. Minim's switch is tested at every stop:
each skin paints all three windows at the five scales with the display where
the layout says, the silver windows are round at their corners and nowhere
else, the skin key and Ctrl+K go all the way round, the menu lists the skins
by name, every panel control is on Tab, and the choice is remembered.

The silhouette is checked as **fractions of the window** rather than as
pixels or as a golden image. That is exactly what a shape stated in design
pixels and resolved against the window buys — the same numbers at every
scale, which a bitmap stretched to fit would not give — and a failure names
the row that is wrong instead of handing back two pictures.

The application itself is tested for the thing the faces cannot test: that
changing face keeps the player exactly as it was. The transport, the queue,
the position, the volume, the shuffle and the equaliser's curve are compared
across three switches; the old face's windows are checked closed and the new
face's open; the clock is checked still running and still moving the head;
the face key is checked in every window of the face it starts in; and the
face and the skin are checked to survive into what the next run would read.

## What still stands

Things this application wanted and the toolkit could not do, kept because
they are the useful output of writing one. Most of the original list has
been fixed in uitoolkit — `widgets.Button`/`ToolButton` gained `Painter` and
`Shaper`, `widgets.Slider` gained `Vertical`, `Painter` and `Travel`,
`widgets.ListView` gained `RowGeo`, window snapping became the public `rack`
package, a press bubbles, a window opens with something focused, key events
carry the character as well as the key, `SizingFixed` means a window whose
size is its design. These are what is left:

- **A menu is drawn inside the window it opens from.** The strip is 116
  design pixels tall, so its skin menu is four rows and nothing else — no
  separator, no shortcut column — and the face switch has to live in the
  playlist window's menu instead. A menu that could leave its window (a
  Wayland `xdg_popup`, an X11 override-redirect window) would lift that
  limit for every compact app.
- **A list view's rows cannot be painted by the app.** See `tracks.go`
  above: `RowGeo` places them, nothing colours them.
- **`widgets.Slider` has no step or page of the app's**, no tooltip, no
  formatted reading for a screen reader, and no `AccessibleAction` for the
  increment and decrement it advertises. `players.Fader` adds all four in
  about eighty lines, which is the right size for the gap but not a gap an
  app should have to fill.
