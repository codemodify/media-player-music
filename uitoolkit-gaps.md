# uitoolkit v0.23.4: integration gaps

Review target: `github.com/codemodify/uitoolkit v0.23.4` (tag commit
`d5510dbee047eb28d5093fba9411057c286223e2`), checked on 2026-10-04.
The Go module index and GitHub tag agree. This review executes independent
public toolkit probes as well as player integration checks; app safeguards
are not treated as proof that an underlying defect is fixed. Resolved items
remain for tracking and stable links.

The app supplies artwork, skin manifests, controls, layouts, music actions
and session policy. Default has one window. Eleven Winamp skins use three
panel windows; Headspace has one window, Xbox and Fandom have two, and
Half Life 2 has three. iTunes uses one Main window for full/MiniPlayer modes
and an owned equalizer. Series 9 uses one Main window with embedded playlist
and equalizer drawers plus a compact mode. Classic VLC, RealPlayer and Amarok
each use one resizable Main with embedded sections. The toolkit owns widget
behavior, skin interpretation,
rendering, input, accessibility and native window mechanics. Imported art
retains its original notices. See [the architecture](docs/faces.md).

The release addresses many of the original defects, but does not yet meet
every requirement. The remaining player safeguards cover group rejoining,
interrupted double-clicks, owner recreation, tray icon priority, fractional
playlist backgrounds and generator font/sheet settings. Portable grouping
remains a declined design requirement. Native interactive drag smoothness
still needs visual testing; programmatic move tests do not establish it.

| Item | v0.23.4 result |
| --- | --- |
| 1 Embedded skin registration | Resolved; startup fs.FS registration reverified |
| 2 Nested slider callbacks | Resolved; one input reports the normalized value |
| 3 List wrapper focus and accessible geometry | Resolved; public composition reverified |
| 4 Popup accessibility action | Resolved; enabled/disabled execution checked |
| 5 Hidden grid gaps | Resolved; spans fit in both axes |
| 6 List activation/context policy | API adopted; helper defects remain in 28/29 |
| 7 Main root and multi-pane rejoin | Partial; no hang, but links and stationary geometry break |
| 8 Nearest snapping anchor | Resolved; nearest-anchor policy retained |
| 9 Portable grouping fallback | Open requirement; toolkit design decision remains |
| 10 Movement notifications and update loop | Partial; clock/reentrant fix works, batching/deadline wake-up absent |
| 11 Compact-host file chooser | Resolved; automatic owned dialog fits |
| 12 Utility ownership and taskbar policy | API adopted; owner recreation defect remains in 30 |
| 13 Role preservation across X11 recreation | Resolved; child policy survives recreation |
| 14 Remote file URI authority | Resolved; Remote stays separate from local Paths |
| 15 Initial X11 frame position | Resolved; before-frame position remains stable |
| 16 Programmatic X11 resize geometry | Resolved; exact resize updates layout |
| 17 Generator authoring fields | Partial; anchors/shape pins fixed, fonts still absent |
| 18 X11 shape rectangle ordering | Resolved; native bounding/input shapes install without BadMatch |
| 19 X11 move-event coordinates | Resolved; callback agrees with Position |
| 20 Registered skins missing from enumeration | Resolved; documented override duplicates remain |
| 21 Registered skin replacement paints stale assets | Resolved; reused/fresh looks paint replacement |
| 22 Disabled list pointer input | Resolved; stock input is consumed without action |
| 23 Tray raster icon priority | Open; generic IconName still exported over a raster |
| 24 Fractional resized panel slices | Open; original mismatch reproduced |
| 25 Popup focus jumps do not scroll | Open; Home/End/initial Up still focus invisible rows |
| 26 Generator loses smooth sheet overrides | Open; mixed plans still lose false overrides |
| 27 macOS requested positions revert | Upstream-confirmed; not locally tested on Linux |
| 28 List RowAt ignores horizontal viewport | New public-API reproduction |
| 29 List double-click history survives interruptions | New public-API reproduction; app gesture guard retained |
| 30 Recreated parent leaves stale utility ownership | New native reproduction; public owner refresh retained |
| 31 GPU image cache retains first sampling filter | New rendering-dependency reproduction; not established as app blur cause |
| 32 Text-field placeholder ignores explicit proportional font | New public CPU rendering reproduction; app placeholder workaround retained |
| 33 MenuBar bypasses its bound bar skin part | New public CPU rendering reproduction; app menu-bar painter retained |

## 1. Embedded skin files cannot be registered for rendering

**Resolved; independently reverified in v0.23.4.**

`RegisterSkinFS` registers an `fs.FS` directly for lookup and painting at
1x and 1.75x. An independent `fstest.MapFS` probe verified loading,
sprite painting, user override precedence and restoration after removing
an override. The player registers its twenty embedded packs directly;
there is no extraction, search-path mutation or shutdown cleanup.
Enumeration and replacement were rechecked separately in items 20 and 21.

Source: [style/skin_register.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/style/skin_register.go).

## 2. Slider programmatic updates re-enter OnInput

**Resolved in v0.23.4; both normalization paths executed.**

A Right key on a 0..100 slider at 50, with `OnChange` normalizing 51 to
60, now produces changes `[51,60]`, exactly one input `[60]`, and final
value 60. Normalization inside `OnInput` produces one input `[51]` and
final value 60. The old `[60,51]` input sequence no longer reproduces.
`notifyChange` clears the user-input mark while executing callbacks and
reads the final control value before delivering input.

The player uses the stock callback contract without a suppression guard.

Sources: [widgets/oninput.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/widgets/oninput.go),
[widgets/slider.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/widgets/slider.go).

## 3. List wrappers lose row focus; custom viewports lose accessible geometry

**Resolved; public composition and accessible geometry reverified.**

A component embedding `ListView` and calling `Init(outer)` receives focused,
active row state when the outer component has focus. A custom row viewport
`(10,20,120,60)` with row height 20 reports its first accessible row at
`(10,20,120,20)`; scrolling 20 moves it to `(10,0,120,20)` and marks it
offscreen. The independent probe uses no player geometry repair.

Both player playlists retain the toolkit's focus state and accessible row
geometry. The app paints its current-track marker and selection colors.

Sources: [widget/active.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/widget/active.go),
[widgets/a11y.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/widgets/a11y.go).

## 4. PopupMenu advertises an accessibility action it cannot execute

**Resolved; advertised popup actions execute.**

Independent probes invoke enabled popup rows through
`Window.AccessibleAction` exactly once and dismiss the menu, on both an
in-window layer and a simulated native popup surface. Disabled rows neither
advertise the default action nor execute it. This verifies public toolkit
execution, not a real screen reader or desktop popup interaction.

Source: [widgets/a11y.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/widgets/a11y.go).

## 5. Hidden Grid tracks still subtract a gap from flexible space

**Resolved in v0.23.4; hidden-track spans checked in both axes.**

With tracks `Auto`, `Auto`, `Share(1)`, an 8-pixel gap and the middle child
hidden, both the trailing child and a visible three-track spanning child
end at 400 in a 400-pixel extent. The old 408-pixel span no longer
reproduces. The vertical probe requests `VAlignStretch` so its assertion
measures the track extent rather than a centered child's natural height.
The stock solver and span placement now use the same participating-gap rule.

Source: [widgets/grid.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/widgets/grid.go).

## 6. List selection and activation need separate application hooks

**Activation/context API resolved in v0.23.4; new helper defects remain.**

`ListView.OnActivate`, `Activate`, `RowAt` and `ContextKeepsSelection` now
exist. Independent routed input verifies that a single click selects
without activation, a double-click activates once, Return activates once,
and repeated accessibility default actions each activate the already
selected row. A context press can retain the existing selection.

Both player playlists now use `OnActivate` for their playback command,
stock Return/default accessibility activation, and the toolkit's context
selection policy. Selection alone leaves playback running. The remaining
app pointer-gesture guard prevents the stale click-history defect in item
29; it commits a validated gesture through public `Activate`, preventing
duplicate playback. `RowAt` is not adopted because of item 28.

Reproduce the stock contracts and remaining defects with
[the public list probe](tools/probes/list_activation.go):

```bash
tools/testenv.sh go run ./tools/probes/list_activation.go
```

Sources: [widgets/list.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/widgets/list.go),
[widgets/a11y.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/widgets/a11y.go).

## 7. Rack rebonding reverses the documented Main root

**Partially fixed in v0.23.4; the hang is gone, group links still break.**

The original multi-pane rejoin completes instead of looping forever.
However, joining Main to the end of a foreign chain of three or more panes
clears intermediate reversed links. The retained
[public Rack probe](tools/probes/rack_rejoin.go) needs no app or windows:

```bash
tools/testenv.sh go run ./tools/probes/rack_rejoin.go
```

A three-pane foreign chain becomes parents `[-1,2,-1,0]`; an intermediate
pane becomes a separate root and the old component no longer follows Main.
Four panes reproduce the same loss. Even a two-pane chain with a 2-pixel
snap gap moves the stationary foreign group by 2 pixels instead of
snapping the dragged Main to it. The exact-edge two-pane control passes.

`hangFrom` captures the old path before walking, which fixes the hang, but
clears `p.To` during each iteration, overwriting the link reversed on the
previous iteration. Reflow also snaps the foreign component rather than
the dragged Main when their bond has a gap.

Expected: retain every reversed link and side branch, keep Main the root,
and snap the moving pane while leaving the foreign component stationary.
The player's guarded public Rack pass remains necessary. It chooses a
foreign anchor without native moves, preserves the stationary component,
and restores the correctly oriented bonds. The app also preserves a final
reentrant movement notification rather than discarding it.

Source: [rack/rack.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/rack/rack.go)
(`hangFrom`, `reflow`).

## 8. Rack chooses the first qualifying anchor instead of the nearest

**Resolved; v0.23.4 recheck; original public probe passes.**

With Main at X=0, a nearer anchor at X=8 and a satellite moving to X=108,
stock `Rack.MoveTo` chooses anchor index 1 and leaves the satellite at
X=108. The farther first-inserted anchor no longer wins. No app snapping
arithmetic was added.

Source: [rack/rack.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/rack/rack.go)
(`rebond`).

## 9. Portable logical-window grouping has no fallback abstraction

**v0.23.4 recheck:** the release notes explicitly decline this high-level
API and assign unsupported-platform composition to the application. The
user's requirement remains unmet; this is a toolkit design decision, not
a claim that ordinary Wayland windows can be arbitrarily positioned.

**Medium severity; missing high-level toolkit feature.** The app should
declare logical skin windows, their main anchor, member content/layout,
and snap/detach behavior. The toolkit should realize that group through
available platform mechanisms and an unsupported-case strategy. v0.23.4
has separate-window `rack` and in-host `dock` primitives, but no unified
group declaration or capability-driven fallback between representations.

Public capability check: create three ordinary native Wayland windows with
`Application.NewWindow`, add them main-first to `rack.NewDesk`, and query
`Desk.Places()` and `Window.Position()`. Placement is unavailable and the
positions are unknown; `Desk.Follow` cannot carry the separate windows as a
group. The windows can still render and receive input. `dock.Host` and
`dock.Panel`, with `app.DockWindows` for floating panels, provide useful
building blocks, but the application must explicitly construct that
different host composition. These APIs do not automatically preserve a
declared skin-window group across backend capabilities.

Expected: the same app declaration has a supported grouping strategy on
native desktops without requiring an X11 server or XWayland. Requested API:
accept the main anchor, members, app-owned skins/layouts and snap/detach
behavior; handle native capability checks, physical representation and
unsupported-case policy inside the toolkit. Report any remaining limitation
explicitly. The app should not need platform-specific backend selection to
obtain that behavior.

Ordinary Wayland toplevels cannot simply be made arbitrarily positionable.
A solution may need a different physical representation, such as one host
containing the docked logical panes, or suitable compositor cooperation
where available. These are design directions; the toolkit does not currently
provide that automatic strategy. Retain the app's pane artwork and layout while
the toolkit owns host/window lifecycle, input, focus and accessibility.

The executable's current X11 preference on Linux, including XWayland when
available, is a **temporary workaround to remove once the toolkit supplies
portable grouping**. Explicit `UITK_BACKEND` overrides and headless mode
remain supported. Without X11/XWayland, native Wayland rendering still
works; motion of the separate-window group is unavailable. There is no
mandatory XWayland dependency and no request to patch an impossible native
Wayland `Move` operation.

Sources: [rack/desk.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/rack/desk.go)
(`NewDesk`, `Add`, `Places`, `Follow`),
[dock/host.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/dock/host.go)
(`Host`, `Dock`, `SetWindowOpener`),
[dock/float.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/dock/float.go)
(`WindowOpener`, `FloatPanel`),
[app/dockfloat.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/app/dockfloat.go)
(`DockWindows`), [app/app.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/app/app.go)
(`Options.Backend`, `NewWindow`). This finding comes from the public API
and capability implementation review, separately from the arithmetic
defects reproduced above.

## 10. Group following has no movement notification or managed update loop

**Partially resolved in v0.23.4; event batching and deadline wake-up remain.**

Movement callbacks continue to drive `Desk.Follow`. New `SetClock` and a
250 ms acknowledgement deadline replace call-count patience: frozen-clock
probes survive 80/100 `Follow` calls without prematurely aging a pending
request. The new `again` loop also preserves a final reentrant movement.

The retained [public event probe](tools/probes/rack_events.go) queues four
initiator moves in one `PumpOnce`; it observes four changed follow passes
and four distinct follower placements. An independent X11 probe queues 20
moves and observes 22 clock reads. `followFromEvent` still executes
synchronously for each callback: reentrant retry is not event-batch
coalescing. Follower native moves remain sequential, with separate X11
flushes, so atomic or visibly smooth group movement is not established.

A final move arriving inside the pending 250 ms interval can also remain
ignored after the deadline. The offscreen probe advances the clock 251 ms
and performs three idle pumps; only an explicit `Follow` reconciles the
refused satellite and detaches it. With the real native clock, after 400 ms
a child at `(700,150)` still has rack box `(100,200)` attached to Main;
explicit `Follow` corrects it. No expiry wake-up is scheduled.

```bash
tools/testenv.sh go run ./tools/probes/rack_events.go
# On an isolated X11 desktop:
go run -tags theme_engine_all ./tools/probes/rack_pending_move.go
```

The player's movement callback keeps its item 7 safeguard and now retains
reentrant final moves. Its ordinary pulse still follows unresolved
geometry until the toolkit deadline is consumed; settled event-capable
windows need no position polling. The toolkit should coalesce a real
geometry-event batch and schedule pending-expiry reconciliation itself.
Continuous native drag smoothness requires user-visible testing.

Sources: [rack/desk.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/rack/desk.go),
[platform/x11_linux.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/platform/x11_linux.go).

## 11. File-dialog fallback cannot fit a compact skin window

**Resolved in v0.23.4; automatic compact hosting verified.**

Stock `ShowFileDialog` from a 275×116 host creates an owned 680×500 dialog
window through `Window.OpenDialogWindow`. It hosts the chooser body rather
than nesting a second caption card. Independent public probes at 1x and
1.75x find nonempty path, listing, Open and Cancel controls entirely within
the dialog. Role and owner match the requester. Frame cancellation calls
`OnCancel` once, closes the dialog and retains the requesting control's
focus; this focus check is headless, not a native WM claim.

The app keeps its separately tracked chooser across skin replacement and
tray hiding and prevents duplicate dialogs. That is application lifetime
policy rather than a sizing workaround. Its owner is declared through the
new public API and follows presentation replacement.

Sources: [widgets/filedialog.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/widgets/filedialog.go),
[app/window.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/app/window.go)
(`OpenDialogWindow`).

## 12. Utility windows have no owner or taskbar-visibility API

**New policy APIs adopted; ownership lifetime remains incomplete.**

v0.23.4 provides `RoleUtility`, `WindowOptions.Owner`, `SkipTaskbar`,
`OwnedSurface`, `TaskbarSurface`, `FrameOwner` and `FrameSkipTaskbar`.
The player declares EQ/playlist utility ownership before the first frame.
All application C/X11 writes and utility policy polling have been removed.

Independent isolated X11 probes find utility type, skip-taskbar/pager
states and the correct transient owner. Child recreation and ordinary
hide/show preserve these properties. Parent-only native recreation leaves
existing children pointing to the destroyed owner (item 30), so the app
retains only a public `SetOwner` refresh when Main's native identity changes.

Wayland provides a parent relationship but does not advertise
`FrameSkipTaskbar`: ordinary xdg-shell has no utility-role/taskbar request.
The API reports this limitation instead of promising unsupported behavior.
The separate requirement for portable grouping remains item 9.

Sources: [platform/host.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/platform/host.go),
[platform/x11_linux.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/platform/x11_linux.go),
[app/window.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/app/window.go).

## 13. X11 visual recreation loses the declared window role

**Resolved in v0.23.4; child recreation preserves its declared policy.**

The independent X11 utility probe forces a visual/frame change that
recreates the child native window. Its declared utility role, skip states
and current owner survive. This fixes the old role loss; a parent-only
recreation is a separate relationship defect, documented in item 30.
The player no longer repairs a recreated child's native hints itself.

Sources: [platform/x11_linux.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/platform/x11_linux.go),
[app/shape.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/app/shape.go).

## 14. URI-list decoding treats remote file authorities as local paths

**Resolved in v0.23.4; local and remote targets stay distinct.**

The public URI decoder returns remote file and HTTPS URIs unchanged in
`DropEvent.Remote` rather than turning them into local paths. Empty file
authority, `localhost` and the current hostname decode into local `Paths`;
percent-escaped spaces and comments are handled correctly.

The shared player host consumes `Remote` and removes its duplicate raw-URI
parser. Remote-only and mixed local/remote drops are rejected without
queue changes; accepted local paths are copied before asynchronous folder
loading. Folder recursion and supported media extensions remain app policy.

Source: [widget/drop.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/widget/drop.go).

## 15. First client-frame layout shifts a requested X11 position

**Resolved in v0.23.4; initial frame position independently verified.**

A stock hidden Breeze window at 1.75x moved to `(120,75)` before its first
client-frame layout still reports `(120,75)` after layout, with nonzero
frame margins. The previous shadow-width shift no longer reproduces.
The toolkit reissues the initial request once the real frame margin is
known, and stops treating it as pending once the desktop or user moves it.
The player retains bounded saved-session restoration and WM-refusal policy.

Reproduction: [public geometry probe](tools/probes/window_geometry.go)
on an isolated X11 display, built with `-tags theme_engine_all`.

Sources: [app/move.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/app/move.go),
[platform/x11_linux.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/platform/x11_linux.go).

## 16. Programmatic X11 resize leaves window layout geometry stale

**Resolved in v0.23.4; exact programmatic resize updates geometry.**

The stock X11 probe requests 700×450 from a 520×340 window, without calling
`RequestLayout`. After native events, both `Size` and `WindowRect` report
the new dimensions. The previous backing-buffer/event suppression no
longer leaves stale layout or hit-test geometry.

The Default and skinned placement restorers remove their explicit
`RequestLayout` compensation. Saved minimum sizes and desktop-refusal
handling remain application policy.

Reproduction: [public geometry probe](tools/probes/window_geometry.go).
Source: [platform/x11_linux.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/platform/x11_linux.go).

## 17. Skin generator cannot emit supported responsive slot anchors

**Partially resolved in v0.23.4; font preference authoring remains open.**

`SlotSpec` now exposes `StretchX`, `StretchY`, `FromRight` and `FromBottom`;
`ShapeRect` exposes its two edge pins. Stock write/load/render probes verify
responsive slots and pinned shapes at 1x/1.75x, including enlarged layouts.
The app now authors playlist anchors directly through these fields and
removes the post-generation anchor rewrite. All 51 bundled manifest/art
files remain byte-identical. The resize regression now uses stock
`skingen.Write`, so an app repair cannot hide a generator failure.

`skingen.Plan` still has no font preference field, although skin JSON
accepts `fonts.ui` and `style.FontPrefs` supports it. Imported archive
playlist families and installed-font fallbacks therefore require a small
manifest adapter. Expose the supported preference schema in the public
plan and round-trip it without application JSON rewriting. This is a low
severity authoring request, not a runtime font renderer defect.

The remaining [plans.Write](internal/skins/plans/write.go) rewrite carries
font preferences and item 26's explicit smooth-sheet settings only.

Sources: [skingen/skingen.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/skingen/skingen.go),
[style/skin.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/style/skin.go),
[style/skin_pack.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/style/skin_pack.go).

## 18. X11 shaped resize bands violate the declared rectangle ordering

**Resolved in v0.23.4; actual bounding and input shapes install.**

The X11 SHAPE implementation now submits arbitrary combined silhouette
and resize-band rectangles with `Unsorted`, rather than falsely declaring
`YXBanded`. Independent read-only native queries at 1x/1.75x, minimum and
enlarged sizes, find nontrivial bounding and input regions (145, 223, 265
and 405 rectangles) even when the frame list backtracks in Y. No `BadMatch`
diagnostic occurs. The old rejected request/full rectangular fallback no
longer reproduces. Native Silver resize and saved-size checks also pass.

This verifies native shape installation, not pointer interaction at every
silhouette pixel. The app uses the toolkit shape API without changing
module sources or suppressing errors.

Sources: [platform/x11_linux.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/platform/x11_linux.go)
(`ui_shape_rects`), [platform/shapeorder.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/platform/shapeorder.go),
[app/shape.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/app/shape.go),
[app/shapeband.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/app/shapeband.go).

## 19. X11 move-event coordinates disagree with Window.Position

**Resolved in v0.23.4; move notifications use visible-window coordinates.**

The independent stock X11 geometry probe at 1.75x compares every callback
argument with `Window.Position` during moves to `(420,250)`, `(0,0)` and
`(-120,-75)`. They agree in all cases; nonzero frame margins no longer
introduce the old shadow-relative offset.

Reproduction: [public geometry probe](tools/probes/window_geometry.go).
Sources: [app/move.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/app/move.go),
[platform/x11_linux.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/platform/x11_linux.go).

## 20. ListSkins omits registered application skins

**Original omission resolved in v0.23.4; duplicate IDs are documented policy.**

Independent registration now makes `ListSkins` include the app-owned ID
and label; replacement updates that registration rather than adding an
extra app entry. The player's registration tests check all eleven packs
through the public enumeration API.

An override of the same ID still produces both app and user entries.
Source documents built-in → app → user ordering, with the winner last.
A picker requiring unique IDs must collapse by ID while retaining that
final entry. This is a standing enumeration policy, not the old missing
registration bug. The player's explicit choice catalogue remains app policy.

Source: [style/skin_pack.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/style/skin_pack.go)
(`ListSkins`).

## 21. Re-registering a skin can keep or restore stale artwork

**Resolved in v0.23.4; old and fresh looks paint replacement artwork.**

An independent probe registers red then blue PNG artwork under one ID,
using identical filenames, equal byte sizes and zero filesystem timestamps.
Painting through the already-used look first now yields blue; a fresh look
also yields blue at 1x and 1.75x. A green user override paints through the
same old look after invalidation; removing it restores blue. This checks
the order that previously repopulated caches with stale artwork.

The skin generation appears in both the look memo key and decoded asset
cache key. The immutable startup bundles need no application reload adapter.

Sources: [style/skin_pack.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/style/skin_pack.go),
[style/skin_assets.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/style/skin_assets.go).

## 22. Disabled ListView still selects rows and invokes context actions

**Resolved in v0.23.4; disabled stock controls consume input safely.**

Independent routed left/right presses on a disabled `ListView` preserve
selection and all select/context/activation callback counts. Disabled
Return and accessibility default activation also do nothing. The stock
pointer handler consumes the press, preventing enabled ancestors from
interpreting it as their own context menu or drag.

Both player playlists remove their disabled-input consumption adapter and
delegate to the toolkit. The retained
[public list probe](tools/probes/list_activation.go) checks this stock path.

Sources: [widgets/list.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/widgets/list.go),
[widgets/a11y.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/widgets/a11y.go).

## 23. Linux tray raster icons can be masked by a generic icon name

**Still open in v0.23.4; real private-bus SNI properties checked.** `StatusIcon` documents priority
as `Image`, then `Path`, then `Name`. However, an image-only icon still
exports SNI `IconName="application-default-icon"` alongside its `IconPixmap`.
`statusIconName` substitutes that name whenever `Name` is empty, without
checking whether the caller supplied an image or path. `SetIcon` repeats
the same behavior. The raster itself is exported correctly.

A private session-bus probe needs no native windows or desktop host.
Both image-only creation and `SetIcon` export the generic name plus a
32×32 raster. Minimal public setup:

```go
logo := paintengine2d.NewImage(32, 32)
logo.Clear(paintengine2d.RGB(1, 0, 0))
item, _ := a.NewStatusItem(platform.StatusItemOptions{
    ID: "raster-icon-probe",
    Icon: platform.StatusIcon{Image: logo},
})
defer item.Close()
```

Read `IconName` and `IconPixmap` from the item's
`org.kde.StatusNotifierItem` interface at `/StatusNotifierItem`. The source
exports the generic name plus a 32×32 pixmap. Expected: a supplied raster
does not also request an unrelated generic theme icon. The
[StatusNotifierItem specification](https://specifications.freedesktop.org/status-notifier-item/latest/status-notifier-item.html)
recommends hosts prefer a supplied icon name; this can cause the generic
theme icon to mask the app's raster. This report does not claim that a
generic icon was observed on screen in an image-only native probe.

The player works around this by supplying `Name="media-player-music"`
alongside its embedded logo image. No installed theme icon currently has
that name, so the host can use the supplied pixmap. A matching installed
logo is also a valid fallback. The toolkit should leave `IconName` empty
when `Image` or `Path` supplies the icon, and use its generic fallback only
when no pictogram was provided.

Sources: [platform/status.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/platform/status.go)
(`StatusIcon` contract and `statusIconName`),
[platform/status_linux.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/platform/status_linux.go)
(SNI property export, `iconPixmapsOf`, and `SetIcon`).

## 24. Resized pixelated panel slices lose fractional display scaling

**Still reproduced through public APIs in v0.23.4.** At fractional display scales,
`DrawSkinLayout` paints pixelated panel artwork at its proper scale only
when the panel has its original dimensions. Once resized, its nine-slice
edges use a whole-number multiplier while its anchored slots keep the true
display scale. Background artwork then disagrees with the controls it is
supposed to surround.

The retained [standalone probe](tools/probes/panel_slice_fractional.go)
registers an in-memory skin with synthetic blue artwork and a red footer,
both 1× and 2× sheets, a 273×211 layout, slice margins `[0,148,38,124]`, and
a bottom-anchored 38-pixel footer slot. It uses no player code or imported
skin assets:

```bash
tools/testenv.sh go run ./tools/probes/panel_slice_fractional.go
```

At 1.75×:

| Logical panel size | Painted footer height | Resolved footer slot height | Footer top mismatch |
| --- | --- | --- | --- |
| 273×211, original | 66 device pixels | 66.5 device pixels | Within pixel rounding |
| 420×320, resized | 38 device pixels | 66.5 device pixels | 28.5 device pixels |

The resized footer begins at device Y=522; its slot begins at Y=493.5.
The same failure occurs at 1.25× and 1.5×. At 1× and 2× the artwork and
slots agree. A nonpixelated control fixture uses a 67-pixel footer at
1.75× and agrees with the slot.

`panelDraw` takes its fractional panel path only at the sprite's original
size. Its resized fallback to `skinDraw` floors the asset target and
texel multiplier for pixelated art; sliced destination margins consequently
use that integer multiplier. That control-face policy should not make
resizable panel artwork disagree with the layout's design coordinates.
The toolkit needs panel-slice edges that retain the actual display scale
and round to the device grid.

The imported packs initially worked around this by putting their resizable
playlist backgrounds on nonpixelated sheets while keeping the other art
pixelated. A later app sharpness change also gives chrome and controls
separate higher-detail 2× artwork; bitmap readouts retain nearest sampling.
The playlist backgrounds still use the nonpixelated path to avoid this
geometry defect. Revisit that policy when the toolkit supports correct
fractional panel slices. The importer
also cleared fixed controls from the background before resizing; that was
an independent app correction, not this toolkit defect.

Sources: [style/skin_paint.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/style/skin_paint.go)
(`panelDraw`, integer asset/texel selection and slice margins),
[style/skin_layout.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/style/skin_layout.go)
(`SkinSlotRect` and `DrawSkinLayout`).

## 25. Popup Home/End and initial Up focus rows outside the viewport

**Still reproduced in v0.23.4; medium severity keyboard usability defect.**
A long `widgets.PopupMenu` correctly constrains its height and supports
scrolling, but some keyboard focus changes do not bring their target into
view. Return executes the invisible item's action. This affects any long
choice list in a small host, including the player's twelve-appearance
selector when it uses an in-window popup.

The retained [standalone probe](tools/probes/menu_focus_scroll.go) creates a
275×116 headless window and twelve ordinary menu items through public
APIs. It runs independently of the player and skin assets:

```bash
tools/testenv.sh go run ./tools/probes/menu_focus_scroll.go
```

At 1x, the constrained viewport is 108 device pixels high. End or an
initial Up selects the last item, but leaves `OffsetY=0` and that row at
Y=292..318, outside the viewport. After repeated Down keys scroll to the
last item, Home selects the first item but leaves `OffsetY=218` and its
row at Y=−212..−186. The same failures reproduce at 1.75x. In each case
Return invokes the selected item and dismisses the menu, despite the
item remaining invisible. Repeated Down navigation brings the last row
fully into view at both scales; wheel scrolling also works in the app.

Expected: every keyboard focus change scrolls the focused row into the
constrained viewport before it can be activated. In `KeyPress`, direct
focus assignments for Home, End and the initial Up/Down branches omit
`ensureItemVisible`; `moveFocus` already calls it. Apply the same visibility
step to those direct assignments, and cover these keys in a constrained
menu regression. The player retains the working arrow/wheel paths and
does not add a separate custom selector.

Source: [widgets/menu.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/widgets/menu.go)
(`KeyPress`, lines 1036–1076, versus `moveFocus`, lines 1177–1189).

## 26. Skin generator loses nonpixelated overrides in mixed sheet plans

**Still reproduced in v0.23.4 in both sheet orders; medium severity authoring defect.**
`skingen.Write` changes a plan that mixes pixelated and smooth sheets into
a manifest whose sheets all load as pixelated. This is separate from item
24's rendering defect: it loses the explicitly requested sheet policy before
rendering starts.

The retained [standalone probe](tools/probes/skingen_mixed_sheets.go) uses
only public `skingen` and `style` APIs. It creates two ordinary sheets,
one with `Pixelated=true` and one with `Pixelated=false`, writes the plan,
then decodes it through `style.LoadSkinFS`:

```bash
tools/testenv.sh go run ./tools/probes/skingen_mixed_sheets.go
```

The generated manifest sets `design.pixelated=true`. Its pixelated sheet
includes an explicit true override, but the smooth sheet omits the
`pixelated` key. The loader correctly inherits the design default for an
omitted key, so the intended smooth sheet incorrectly loads with
`Pixelated=true`.

Expected: each sheet's loaded policy matches the plan. Emit the boolean
explicitly for every sheet, or retain true per-sheet overrides without
promoting the design default. Cover mixed plans and both sheet orderings
with a public write/load regression. The player's `plans.Write` adapter
already adds explicit `pixelated:false` keys to smooth sheets, making the
responsive playlist workaround in item 24 effective. Remove this rewrite
when the generator preserves that policy itself.

Sources: [skingen/skingen.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/skingen/skingen.go)
(lines 351–362), [style/skin.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/style/skin.go)
(`loadSheets`, lines 785–792).

## 27. macOS requested window positions revert

**High severity on macOS; upstream-confirmed, not locally executed here.**

The v0.23.4 release notes report a confirmed failure of
`TestAppKitGeometryRoundTrips`: an application move takes effect, then
about 100 ms later the window returns to its creation frame. The notes
state that this predates the release and prevents application positioning
and rack snapping. This affects the player's saved placement and grouped
skin windows on macOS. This Linux review does not claim to reproduce it.

Expected: accepted moves remain in effect until another application,
window-manager or user action changes the position. Validate position
round trips and moving/detaching all members of a skinned group on macOS.
A source-level suspected custom-MIME pasteboard issue is also mentioned
upstream, but is not promoted here to a confirmed player defect.

Source: [v0.23.4 release notes](https://github.com/codemodify/uitoolkit/blob/v0.23.4/release-notes.md#known).

## 28. List RowAt ignores the horizontal row viewport

**New v0.23.4 public-API defect; medium severity for custom list geometry.**

With `RowGeo.Rows=(10,20,120,60)`, scrollbar `(140,20,10,60)` and row
height 20 in a 200×100 list, `RowAt(0,30)`, `(135,30)` and `(151,30)`
all return row 0 although none is inside the row viewport. Y-outside
points correctly return −1. The same Y-only test also appears in stock
pointer selection. A gesture handler using this API can act on a row when
the pointer is on a border, gap or blank area beside it.

Reproduce with [the public list probe](tools/probes/list_activation.go):

```bash
tools/testenv.sh go run ./tools/probes/list_activation.go
```

Expected: return −1 for every point outside `Rows`, while separately
excluding the scrollbar and applying the scroll offset to a valid row.
Check both axes against the actual custom viewport. The app retains its
verified row-hit path and does not adopt this new helper yet.

Source: [widgets/list.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/widgets/list.go)
(`RowAt`, `MousePress`).

## 29. List double-click history survives interrupted gestures

**New v0.23.4 public-API defect; medium severity unintended activation.**

One primary press, an interruption, then a primary press on the same row
within the toolkit interval activates it as a double-click. Separate
probes reproduce this after pointer exit, focus loss, wheel scrolling,
secondary/context press, arrow navigation, Ctrl-click, and a disabled
press followed by re-enabling. Each emits one activation instead of zero.
There is no player wrapper in the reproduction.

Run [the public list probe](tools/probes/list_activation.go). Expected:
an interrupted gesture cannot combine with a later single click. Reset
click history on these boundaries and on relevant content replacement;
cover ordinary double-click activation separately.

The player keeps its existing pointer-interruption guard while using
`OnActivate`, public `Activate`, stock Return/accessibility activation and
context policy. It suppresses stock activation only during the guarded
pointer dispatch, so an accepted app gesture issues playback exactly once.
Remove this guard once the toolkit owns correct gesture boundaries.

Source: [widgets/list.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/widgets/list.go)
(`MouseExit`, `MousePress`, `MouseWheel`, `KeyPress`, click history).

## 30. Parent recreation leaves utility children owned by a destroyed window

**New native X11 reproduction in v0.23.4; medium severity ownership lifetime.**

Create a main window and an owned `RoleUtility` child, then change only
the main window's visual/frame so the toolkit recreates its native X11
window. The main ID changes (observed `400002→400013`); the child's ID
stays `400006`, but its `WM_TRANSIENT_FOR` still points to destroyed
`400002`. Calling public `child.SetOwner(main)` corrects it to `400013`.
Child-only recreation and ordinary hide/show are successful controls.

The retained [public owner probe](tools/probes/utility_owner.go) uses no
player adapter and inspects actual properties on an isolated X11 desktop:

```bash
go run -tags theme_engine_all ./tools/probes/utility_owner.go
```

Expected: toolkit owner relationships survive either participant's native
recreation. Refresh existing owned surfaces after an owner's native ID
changes; do not make each application track native identities. The player
retains only a public `SetOwner` refresh keyed to Main's `PortalParent`,
while all former C/X11 property writes and policy polling are removed.

Sources: [platform/x11_linux.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/platform/x11_linux.go),
[app/window.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/app/window.go)
(`SetOwner`, native recreation).

## 31. GPU image cache retains the first sampling filter

**Open; independently reproduced in paintengine2d v0.11.0, the rendering
dependency used by UIToolkit v0.23.4. This is not established as the player's
blur cause.**

Draw one unchanged checkerboard image with `FilterBilinear`, then draw the
same image with `FilterNearest`. The second draw should contain only the
source's black and white pixels. In the public windowless GPU probe it
instead contains 144 blended pixels in a 12×12 interior. Starting with
nearest yields zero blended interior pixels.

```bash
tools/testenv.sh go run ./tools/probes/gpu_image_filter.go
```

`GPUDevice.Blit` passes the requested filter to `uploadImage`, but a texture
cache hit returns before applying that filter. Cache identity and freshness
cover the image and its contents, not sampling state. Update texture
sampling when a cached image's requested filter changes.

This is a visible output-correctness defect. Callers needing both filters
can use a separate image per filter as a temporary workaround. The player's
skin images use consistent per-sheet filters, so no app workaround is
required for this finding.

Sources: [GPUDevice.Blit](https://github.com/codemodify/paintengine2d/blob/v0.11.0/gpu_linux.go#L1220),
[uploadImage cache and sampler state](https://github.com/codemodify/paintengine2d/blob/v0.11.0/gpu_linux.go#L2031).

## 32. Text-field placeholder ignores an explicit proportional font

**Open; independently reproduced in v0.23.4.**

Passing an explicit 11-point proportional face to `DrawTextField` renders
typed text at 11 points, but an empty field's placeholder uses the look's
default 16-point muted face. The same placeholder rendered with explicit
11- and 16-point faces has byte-identical alpha pixels. The public CPU probe
measures typed text at 65×10 versus 94×15 pixels; the 11-point placeholder
also occupies 94×15. A monospace control case honors the requested size.

```bash
tools/testenv.sh go run tools/probes/field_placeholder_font.go
```

`mutedFor` preserves the supplied size only when its family is the look's
monospace family; other faces return the cached default muted face.
`baseDrawTextField` uses that result for an unfocused placeholder. Muting
should retain the requested face's metrics while changing its color. The
larger placeholder can clip or misalign in a compact field where typed text
fits. The reproduction uses a frameless field so frame pixels cannot hide
the difference. Text-area code also calls this helper, but was not separately
executed for this finding.

The iTunes search field temporarily paints its placeholder through public
`DrawTextIn` with the original 11-point face and delegates an empty placeholder
to the toolkit. The stock editor still owns typed text, caret, selection and
clear actions; its source-derived Aqua frame is retained.

Sources: [mutedFor](https://github.com/codemodify/uitoolkit/blob/v0.23.4/style/look.go#L327),
[baseDrawTextField](https://github.com/codemodify/uitoolkit/blob/v0.23.4/style/look.go#L462).

## 33. MenuBar bypasses its bound bar skin part

**Open; independently reproduced in v0.23.4.**

The public skin contract lists `bar` as the face for menu, tool and status
bars. A registered skin with a valid solid-magenta `bar` part loads correctly,
and public `DrawSkinSprite` paints that part. The stock `MenuBar` instead
paints its dark fallback background. At both 1× and 1.75×, an unobstructed
background pixel is `{66 66 66 255}` instead of `{255 0 255 255}`.

```bash
tools/testenv.sh go run tools/probes/menu_bar_skin.go
```

The probe creates only a stock menu bar and an in-memory skin through public
toolkit APIs, with no player code. `SkinEngine` supplies `underFor("bar")`,
but the base menu-bar painter calls `paintBezel` directly and bypasses the
skin face's paint hook. Authored menu backgrounds therefore disappear even
when the part is valid. Tool and status bars were not separately exercised.

VLC temporarily wraps public `LookAndFeel.DrawMenuBar` to paint its authored
gray bar. The stock menu widget still owns pointer routing, keyboard behavior
and accessibility. The toolkit should honor the bound `bar` face in its
normal menu-bar painter so applications can remove this wrapper.

Sources: [public bar contract](https://github.com/codemodify/uitoolkit/blob/v0.23.4/style/skin.go#L360),
[skin menu-bar dispatch](https://github.com/codemodify/uitoolkit/blob/v0.23.4/style/engine_skin.go#L746),
[base menu-bar painter](https://github.com/codemodify/uitoolkit/blob/v0.23.4/style/look.go#L631).

## Window grouping capability on native Wayland

**Protocol/capability limitation, not a toolkit defect.** Ordinary native
Wayland `xdg_toplevel` windows have no client-visible global position and
cannot be moved to an arbitrary desktop coordinate. The toolkit correctly
reports no position/move capability, so `Desk.Places()` is false and a rack
cannot physically carry neighbouring ordinary toplevels. This does not ask
for an impossible Wayland `Move` implementation. Layer-shell placement and
`xdg-toplevel-drag` serve different surface/drag use cases.

X11, including XWayland, exposes the position/move capabilities required
by this rack; individual window-manager refusals still need to be respected.
The executable currently prefers available X11 on Linux as a temporary
workaround for the missing portable grouping API in item 9. Explicit
`UITK_BACKEND` overrides, headless rendering and the native fallback without
X11 are preserved. This workaround should be removed when the toolkit can
choose an appropriate grouping strategy; platform selection belongs to the
toolkit. On Wayland-only systems, separate-window rendering works while
group motion remains unavailable.

Sources: [platform/host.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/platform/host.go)
(`SurfaceMoves`), [platform/wayland_linux.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/platform/wayland_linux.go)
(`wlSurface.Position`, `Move`), [platform/x11_linux.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/platform/x11_linux.go)
(`x11Surface.Position`, `Move`),
[platform/backend.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/platform/backend.go)
(`Default`, `Select`), [rack/desk.go](https://github.com/codemodify/uitoolkit/blob/v0.23.4/rack/desk.go)
(`Places`).

## Player verification at v0.23.4

The final full default and all-engine suites pass, as do all-engine vet
and both builds. Focused widget, playlist, session/tray and group races
pass in both build configurations; the complete skin race suite and plain
playback race suite also pass. The updated real-mpv spectrum boundary
check passes ten tagged race runs. An initial outdated RoleDialog test
expectation was updated to assert utility role, Main ownership and taskbar
policy across all eleven skins. A separate spectrum fixture now waits for
the replacement file before assigning its analyzer generation; this fixes
a deterministic old-track observation, with silence assertions unchanged
and no production analyzer change.

Independent public probes cover the report's fixed and remaining widget,
generator, registration and rendering findings. Real private-bus SNI
properties confirm item 23. Isolated virtual KWin/XWayland checks verify
utility ownership/taskbar policy, child and parent recreation, mapped public
setters, hide/show, minimum/enlarged playlist geometry, saved size, native
shape installation and event-only group movement/detachment/rejoining
after startup placement settles. Native Wayland separately confirms no
ordinary-window placement and ownership without taskbar-control capability.
These tests inspect only their own windows and configuration; macOS remains
an upstream-reported limitation, not a local native test.

Native external XDND tests pass for Default, Winamp and Winamp Silver at
1.75x, including target discovery, frame recreation, Copy/Move-preferred
sources and recursive queue additions. The same native Winamp test against
`/home/user/Music/net` adds all nine recognized files for each action
negotiation. A recording backend plays no audio and changes no user queue.
This verifies the native selection protocol, not a manual file-manager drag.

At the toolkit-only upgrade, all twelve appearances received fresh
1x/1.75x screenshots, including 420×320 playlists. All 88 optional-skin
panel/resize snapshots were byte-identical to v0.23.3, and all 51 generated
manifest/art files remained unchanged.
Real mpv playback and measured visualization continue across all twelve
appearances. A Windows 98 screenshot launch outside the checkout confirms
embedded runtime assets.

That executable records uitoolkit v0.23.4 in its build metadata. At that
review, the local player restarted and restored Winamp Classified, stopped,
at 1.75x with its saved session. Its SNI item reports Active and the
app-specific icon name. Interactive drag smoothness still requires visual
confirmation; retained safeguards and remaining findings are documented
above rather than treated as resolved.

### Subsequent application branding changes

The eleven packs were subsequently regenerated with the application's
speaker logo replacing legacy Winamp marks, exact-case **Music Player** main
captions, and readable amber ink on Amber's dark title plates. Optional menu
names now use **WinAmp -**; skin IDs and original source archives are
unchanged. These are application artwork and naming changes, separate from
the unchanged-art toolkit comparison above.

Fresh validation covers all twelve appearances at 1x/1.75x, including resized
playlists (90 PNGs), original-logo pixels, reproducible packs, title case and
fit, source integrity, compact menu selection and accessibility. No new
toolkit defect or additional runtime workaround was found in this follow-up;
the thirty toolkit items above remain the branding review's findings. The
subsequent skin sharpness audit adds the separate renderer finding in item 31.

### Subsequent application skin sharpness changes

The ten bitmap-based packs now author separate edge-aware 2× chrome and
transport artwork instead of enlarging every sprite as identical blocks.
Speaker logos use the original logo at 2×. Tiny bitmap letters, segmented
digits, lamps, printed equalizer labels and text-bearing controls retain
nearest sampling. Skin-authored optional backdrop slots preserve source
display patterns beneath live readouts. Dense DOS, Radar and TI-82 main
artwork also keeps its original pixel rendering. Silver is unchanged.

The original 20 1× PNGs and the complete Silver pack remain byte-identical.
Fresh headless captures cover all twelve appearances at 1×, 1.75× and 2×,
including resized playlists (135 PNGs); all twelve Silver captures match
the preceding app build byte for byte. Pixel regressions cover protected
readout and EQ textures, external backdrop slot placement, and fractional
display sampling. Panel placement, resizing, captions and skin menus pass
in both build configurations. Generated packs remain reproducible through
the public authoring API, and both executables build.

The public sampling probe confirms that the previous bitmap controls already
used nearest sampling; their low-resolution sources and fractional pixel
widths limited clarity. The sharpness improvements belong to the application's
artwork. The independent GPU cache
defect discovered during this review is retained in item 31. This review
does not claim pixel-exact compositor output from the native desktop.

### Subsequent Headspace verification

Headspace uses one fixed 760×394 window with transparent source artwork and
four masks matching the equalizer and playlist wing states. An initial
rectangular fallback exposed an intentional API restriction: `SkinSlotShape`
derives control hit shapes and declines images larger than 512² device pixels.
The presentation now paints its declared mask through `DrawSkinSlot` and
passes the resulting image to `Window.SetShapeFunc` and
`platform.NewShapeImage`. These public window APIs support the complete
silhouette; this is a control-helper limitation, not a missing window-shape
API or an additional numbered defect. Sources:
[control mask limit](https://github.com/codemodify/uitoolkit/blob/v0.23.4/style/engine_skin_shape.go#L34),
[slot helper contract](https://github.com/codemodify/uitoolkit/blob/v0.23.4/style/skin_layout.go#L276),
[window image mask](https://github.com/codemodify/uitoolkit/blob/v0.23.4/platform/shape.go#L186).

Focused headless tests verify all four states at 1×, 1.75× and 2×, including
zero-alpha empty corners, input-region membership, control visibility,
transport actions, backend limits, hidden-wing focus and saved placement.
Playlist guards retain the application protections for the horizontal row
gutter in item 28 and interrupted gestures in item 29. URI-drop tests verify
local file/folder decoding, remote rejection and refusal of Move-only sources.
The plain and all-engine race suites pass.

The reusable native X11 probe in `tools/probes/headspace_shape.go` passes at
1× and 1.75× for every wing state. Actual bounding/input regions exclude
empty corners and match each wing's visibility; the capture corner remains
transparent and `Frame.Alpha` is true. Run it with
`go run -tags theme_engine_all tools/probes/headspace_shape.go`. The probe
immediately hides its own window and checks installed native regions rather
than simulating pointer input on the user's desktop.

### Subsequent Xbox verification

The original 2002 Microsoft/The Skins Factory Xbox archive now supplies a
fixed 350×216 main view, an inset equalizer, and a resizable 448×316 playlist.
PNG/GIF artwork uses magenta color keys. The transport GIF palette quantizes
its transparent pink to `#FF22FF`; the app importer recognizes that palette
entry and verifies every remaining source pixel. This was an artwork import
issue, not a toolkit transparency defect. The original archive and all 132
entries remain unchanged in `internal/skins/sources/xbox`.

Both windows use the unrestricted public window image-mask APIs described
above. Mapped transport polygons use ordinary shaped buttons. Original
grayscale position images guide the curved seek and volume controls: an app
slider wrapper transforms pointer coordinates and retains the toolkit's
dragging, user-input notifications, keyboard operation and accessibility.
The resizable playlist uses the original frame pieces through a declared
nine-slice background. Playlist row and double-click guards continue the
protections documented in items 28 and 29.

Headless checks cover shapes, original geometry, transport/file hooks,
curved input at 1×/1.75×/2×, equalizer focus and backend limits, file/folder
Copy drops, playlist resizing, activation and saved placement. The full
plain test suite, pack lint and reproducible generation pass. Focused Xbox
race suites pass in plain and all-engine builds, including playlist context
actions and hidden-window placement after the startup retry timeout. Linux plain
and all-engine builds, Windows amd64 and macOS arm64 cross-builds, and
`go vet ./...` pass; these cross-builds do not claim native Windows/macOS UI
verification.

`tools/probes/xbox_shape.go` checks actual X11 bounding/input regions and
window properties at 1× and 1.75×, including a playlist resized to 610×430.
Main corners remain outside the native input shape. Both captures retain
zero-alpha corners on ARGB32 surfaces. The playlist's native input includes
the toolkit's intentional resize bands over its small transparent corner
cuts, while its painted mask retains the original silhouette. Native regions
match the published `Frame.Shape`. The playlist remains a utility window
with skip-taskbar/skip-pager hints and ownership by the current main surface
before and after resizing. The probe hides and cleans up its own windows,
without changing the live player or its session.

No additional numbered toolkit defect was found during this integration;
all existing entries and outstanding requests remain saved in this report.

### Subsequent Half Life 2 verification

The original Valve/The Skins Factory WMP skin is now imported as
`half-life-2`, using the public toolkit with a fixed 339×300 main window,
a fixed 430×239 equalizer and a resizable playlist starting at 393×226.
The archived WMZ and all 145 extracted files retain their original bytes.
The original archive omits its two referenced playlist dropdown images;
the app supplies the dropdown, metadata columns and duration footer.

Focused UI, source-fidelity, placement, session and popup tests cover
normal and fractional display scales. Playback uses the shared backend
hooks; unsupported SRS, TruBass, WOW and crossfade actions remain disabled.
All three views route copy drops through the shared file/folder loader.
Playlist clicks browse, while double-click, Enter and accessibility
activation play. Resizing retains the original frame and a readable
five-column viewport. Saved placement keeps this three-window family
independent of Default, Winamp, Headspace and Xbox.

The integration corrected application authoring mistakes: a union mask
claimed empty pixels in the open shutter state, right/bottom anchors used
the wrong edge margins, EQ fills covered their source thumbs, and the
opaque Lambda control hid the spectrum. Runtime now uses each shutter's
actual alpha mask, correct source control positions, visible thumbs, and
an optional spectrum in the main display. Original glass paints once
above the inner controls through a passive overlay. Focus moves to the
visible Play/Pause control and to the shutter button when covers close.
These are application fixes, not additional toolkit defects.

UI and playlist races passed, together with shell family/session/cycling
and compact popup activation races in both plain and all-engine builds.
Skin generation, source pixel fidelity, archive integrity and byte-for-byte
bundle reproduction passed. `tools/probes/halflife2_shape.go` checks native
X11 bounding/input regions, ARGB32 transparent captures and current utility
ownership at 1× and 1.75×, including both shutter covers and a 610×430
playlist. The probe uses only its own hidden windows and does not start
audio or read the live session. Programmatic grouping checks pass; this
does not establish pointer-drag smoothness on every desktop.

No new numbered toolkit issue was identified. The existing outstanding
entries remain saved, including portable grouping, Rack following,
utility ownership after native recreation and stock-list input behavior.
Half Life 2 uses the same public-API application guards as the other skins.

The full plain application suite passed after the final playlist checks.
Plain and all-engine Linux builds, Windows amd64 and macOS arm64 cross-builds,
and all-engine vet passed. A screenshot launch of the rebuilt executable at
1.75× produced all three panels without audio or live-session access. Final
native shape/owner checks and the 1×/1.75× artwork previews passed. Windows
and macOS runtime behavior was not exercised here.

### Subsequent Fandom verification

The supplied silver-player reference is now adapted as `fandom`. Its original
skin identity, author and archive remain unverified; the project preserves
its generated artwork and prompts separately from historical WMP archives.
Main is fixed at 490×330 with an embedded Settings wing and inset EQ. The
owned utility playlist starts at 190×246 and can resize. The app provides
layouts, controls, playback hooks and source artwork through public APIs.

This integration found an application painting issue: a shaped window still
starts with the look's opaque background, so a binary extent mask alone made
soft source edges opaque. The root painters erase that fill under the current
damage clip using `BlendDestOut`, then paint the original art once. Binary
extent masks avoid applying the source alpha twice. Hiding the Settings wing
updates both its painted content and native input mask. No toolkit module
was changed for these corrections.

Focused tests cover shared actions and keyboard shortcuts, backend capability
limits, measured spectrum, single-click browsing and double-click activation,
local file/folder Copy drops, interrupted gestures, resized playlist slots,
docking/following, hidden-window restore and separate session placement.
Playlist, catalogue and shell family/session/menu race checks pass in plain
and all-engine configurations. Source hash, independent 1×/2× sampling,
manifest lint and byte-for-byte bundle reproduction pass.

`tools/probes/fandom_shape.go` passes on KWin/X11 at 1× and 1.75×. It checks
actual Bounding/Input regions against public `Frame.Shape`, soft source-alpha
captures, Settings wing true/false/true, transparent gaps/corners, a resized
310×356 playlist, current utility ownership and skip-taskbar/skip-pager hints.
It uses its own hidden windows and private bus/config directories. These
native checks do not establish interactive drag smoothness or native
Windows/macOS behavior.

No new numbered toolkit defect was found. All 31 existing entries and their
outstanding requirements remain preserved above.

Final validation passes: full plain `go test ./...`, all-engine Fandom UI
races, all-engine vet, Linux plain/all-engine builds, Windows amd64 and macOS
arm64 cross-builds. The rebuilt executable produced both panels at 1.75× in
screenshot mode without audio or live-session access. Final artwork previews
at 1×/1.75× are saved with the project.

### Subsequent iTunes verification

`Mac OS X - iTunes` uses one Main window for both the resizable full player
and the original horizontally resizable MiniPlayer, plus a fixed owned
equalizer. Public `SetResizable`, `SetMinSize`, `SetSize` and
`SizeLimitSurface.SetSizeLimits` support both modes without replacing the
native window. Full mode has a 640×400 minimum; MiniPlayer limits are
150×62 through 1000×62. Switching preserves the current position and keeps
separate full/mini size caches. Presentation state restores before actual
window placement. Native size acknowledgement prevents a pending mode change
from undoing a subsequent user resize.

Focused tests cover mode cycles at 1×/1.75×/2×, copied persistent state,
startup placement, native size limits, shared transport and EQ hooks,
backend feature limits, keyboard search and hidden-content focus. Narrowing
MiniPlayer folds its LCD/seek and returns focus to visible Play. Plain UI
tests and all-engine core/mode/EQ race tests pass; focused narrow-player
tests pass in plain and all-engine race configurations. Playlist and shell
mode/session/backend/tray races pass in both configurations.

```bash
probe_display=${DISPLAY:-:0}
tools/testenv.sh env DISPLAY="$probe_display" go run -tags theme_engine_all tools/probes/itunes_modes.go
```

The native KWin/X11 probe passes at 1×/1.75× through full, MiniPlayer widths
380/150/430 and repeated returns to full. Main retains its XID. Actual
`WM_NORMAL_HINTS` limits match each mode's scaled dimensions. Bounding/Input
regions match public `Frame.Shape`, the equalizer retains current ownership
and skip-taskbar/skip-pager hints, and ARGB captures preserve source edge
alpha once. Resizable Main intentionally includes the toolkit's invisible
outer resize band; fixed EQ follows its rounded visual extent. The probe
uses only its own hidden windows and private bus/config directories. It does
not establish native interactive drag behavior or Windows/macOS runtime
behavior.

Original resource decoding and adapted layout are application work, not
missing toolkit APIs. The new proportional-placeholder defect is recorded
as item 32; all previous entries remain preserved.

Final verification passed the complete plain application suite, complete
iTunes UI races in plain and all-engine builds, search/clear focus checks,
all-engine vet, reproducible skin generation and original-resource integrity.
Linux plain/all-engine builds and Windows amd64/macOS arm64 cross-builds pass.
Full, wide/compact MiniPlayer and equalizer captures were reviewed at 1×/1.75×;
the rebuilt executable also produced full-player/EQ screenshots at 1.75×.

### Subsequent Series 9 verification

`Win Media Player - Series 9` adapts Microsoft's original `9SeriesDefault.wmz`
(internal theme ID `Corona`). Archive integrity verifies the original SHA-256,
all 119 extracted member bytes, the 115 uncompressed 24-bit BMP headers,
three JavaScript files, one WMS and a defensive archive accessor. Magenta
transparency keys become alpha during import; the original files and notices
remain unchanged and source scripts are not executed.

One native Main contains the resizable 346×344 base chassis, an embedded
playlist that adds 250 pixels of width and an EQ drawer that adds 124 pixels
of height. Compact mode uses the same window at fixed 346×103 and suspends
both drawers while preserving their model preferences. Full size is saved
without drawer travel, state restores before actual placement, and all
transitions retain the current top-left. Public layout slots, sprite controls,
stock list/slider composition and full-window image masks support this design
without toolkit changes. Known list guards in items 28/29 remain.

Native testing exposed a premature application acknowledgement: `OnResize`
laid out the requested backing size before a delayed answer for an earlier
request arrived. Clearing the pending mode resize at the first target layout
let that older size undo a drawer transition. The app now retains prior
requests during a bounded transition, corrects late old answers, avoids
no-op drawer requests and accepts a distinct legal user resize. Quiet target
geometry ends the transition; native user gestures cancel restoration.

```bash
probe_display=${DISPLAY:-:0}
tools/testenv.sh env DISPLAY="$probe_display" go run tools/probes/x11_resize_burst.go
tools/testenv.sh env DISPLAY="$probe_display" go run -tags theme_engine_all tools/probes/series9_shape.go
```

The standalone toolkit burst probe uses no player/controller code. It
observes a requested layout, an older layout and the final requested layout;
four cycles settle with native, backing and public geometry agreeing. This
does not establish a persistent toolkit resize defect, so item 16 stays
resolved and no additional numbered gap is added.

The Series 9 KWin/X11 probe passes at 1×/1.75×: every drawer combination,
two compact/full cycles, enlarged 700×524 full mode, unchanged native XID
and position, actual Bounding/Input regions equal to `Frame.Shape`, ARGB
transparent corners and scaled native minimum/fixed-size hints. Fresh
full and compact players also restore saved state then placement before
first layout, retaining actual size, position and the full base-size cache.
The invisible native resize band remains distinct from visual alpha. Probes
hide only their own windows, use private bus/config directories and start
no audio backend. Interactive native dragging and Windows/macOS runtime
behavior were not tested here.

Post-correction plain Series 9 UI tests pass; all-engine core/mode/EQ races
and focused delayed-answer/user-resize/startup races pass. Source-derived
EQ positions and shared action/capability checks pass. Playlist and shared
shell mode/session/backend/tray races pass in both configurations. All 32
existing toolkit entries remain preserved.

Final Linux plain/all-engine builds, all-engine vet and Windows amd64/macOS
arm64 cross-builds pass with the corrected controller. The rebuilt executable
produces the Series 9 screenshot at 1.75×; post-correction shell Series 9
integration checks pass. The complete plain application suite also passed
before the narrowly tested controller correction.

### Subsequent classic VLC, RealPlayer and Amarok verification

Three independent appearance families now use one resizable Main each:
`VLC - Classic` adapts the VLC 0.8.6 wxWidgets interface, `RealPlayer - Classic`
adapts RealPlayer 8, and `KDE - Amarok 1.4` uses original Amarok 1.4.3 artwork.
The source directories preserve original files, hashes, licensing notices
and provenance. Their platform chrome is recreated as app-owned geometry;
these are appearances over the existing selected audio backend.

VLC starts at 500×430 with a 440×300 minimum; RealPlayer at 720×520 with a
606×500 minimum; Amarok at 920×620 with a 740×480 minimum. Each embeds its
playlist and equalizer. Public slots, stock widget composition and authored
skin parts support the three layouts. The common `QueueView` adds application
gesture policy for items 28/29; Amarok's filtered/sorted view maps visible
rows back to shared queue indices without changing the playback order.

Focused checks cover responsive geometry, original sprite pixels, local
file/folder Copy drops, single-click browsing, double-click/Enter/accessibility
activation, interrupted gestures, shared transport, capability-gated EQ and
measured spectrum, keyboard focus, and primary-only slider changes. Skin
switching preserves the playing queue, backend, tray and update loop, while
each family restores its own position and size. Plain and all-engine skin
UI/source races pass. Artwork previews at 1×/1.75× and minimum/preferred/
expanded sizes were inspected; original higher-resolution Amarok icons are
used for its 2× sheet.

The independent menu-bar reproduction is saved as item 33. VLC uses a narrow
public menu-bar paint wrapper until that binding is fixed. Amarok retains
the existing proportional-placeholder workaround for item 32. Secondary
pointer actions on sliders are application policy, not a new toolkit gap.
All previous numbered items remain preserved; no toolkit module was edited.

Final validation passes: the complete plain application suite, focused shared
queue/menu/appearance
and keyboard races in both configurations, manifest lint and byte-for-byte
bundle reproduction, all-engine vet, Linux plain/all-engine builds, and
Windows amd64/macOS arm64 cross-builds. The rebuilt executable produced all
three Main captures at 1.75× without audio or live-session access. Native
Windows/macOS runtime behavior was not exercised here.

## Earlier player verification at v0.23.3

At the original toolkit update review, Default and all-engine full race
suites passed. After the final rejoin
snapshot guard, affected movement, restoration and resizing race tests
passed again in both configurations. Default and all-engine builds and
`go vet` passed. Fourteen headless screenshots covering all three looks
and all skin panels were reviewed at 1x and 1.75x without new clipping.

Native X11 probes passed utility ownership/recreation, playlist resize and
saved-size restoration, and event-only group following and non-root Main
rejoining for both skins at 1.75x. These tests use owned probe windows and
programmatic moves; they do not establish pointer-drag smoothness. Silver
continues to emit the independently confirmed SHAPE error in item 18.

The actual external XDND test was attempted against `/home/user/Music/net`.
All three presentations advertised `XdndAware=5`, including after frame
recreation, but visible target discovery skipped because their native
windows were not viewable while the desktop was locked. Recursive loading
and headless URI-drop routing pass; a real file-manager drop of this folder
still needs an unlocked-desktop test. The skipped native cases are not
reported as successful file transfers.

The subsequent playlist/taskbar follow-up passed focused race tests in the
default and all-engine configurations, including pointer gestures at 1x and
1.75x, disabled/interrupted input, queue refresh and recording-backend
activation. Selection sends no playback commands; a double-click loads one
track. Native KWin tests passed same-identifier state repair, rapid
hide/re-show and recreation with Main included and both panels excluded
from the taskbar. Retry-scheduling race tests cover refusal, delayed
acceptance, recreation and unsupported backends. Playlist screenshots
confirm readable selection in both skins. Gap 22 records the independent
disabled stock-list defect found during this follow-up.

The logo/tray follow-up passed focused tray race tests in both build
configurations, including headless exclusion, UI-dispatched activation
across skin changes, single-item ownership, shutdown and queued callbacks.
Both builds and affected-package vet passed. A headless screenshot launch
from outside the repository confirmed that the logo is embedded.
Native SNI verification found the player registered with an active desktop
host and `IconName="media-player-music"`. Its 128×128 raster matched all
three windows' 128-pixel icons byte for byte. This verifies registration
and image export; screen pixels were not inspected. Gap 23 records the
independent source-confirmed icon-priority mismatch and app workaround.

The close-to-tray/Classified follow-up passed focused shell race tests in
both build configurations across all four appearances. Coverage includes
continued playback and queued backend updates while hidden, restoring only
previously visible panels, skin changes while hidden, tray-host loss and
explicit Quit. Native close/restore interaction was skipped because the
desktop was locked; the SNI host was present.

Catalogue, saved-session, actual skin-menu activation and popup-bound tests
passed for all four appearances at 1x and 1.75x. Classified source archive
integrity, reproducible generation, lint, paint regressions and the skin
race suite passed. The final EQ and playlist screenshots, including a
420×320 playlist at 1.75x, were reviewed. Gap 24 retains a public standalone
reproduction and documents the background-only workaround.

Final plain and all-engine builds and affected-package vet passed. The
rebuilt local player restored Winamp Silver at 1.75x, and its SNI item
exported Active status with the app-specific icon name. The existing X11
SHAPE error from item 18 remains present during Silver startup.

The eight-skin follow-up on 2026-10-04 passed the full default and
all-engine application suites and all-engine vet. Catalogue/session checks
cover twelve appearances, and actual popup activation/scrolling checks
cover every choice through native, constrained layer and refused-popup
paths at 1x and 1.75x. Source integrity covers nine retained archives and
232 unchanged original files; deterministic decoder tests cover four-bit
indexed and RLE8 bitmap formats used by the supplied archives.

All eleven optional skins passed placement, artwork, keyboard and
accessibility checks. Expanded playlist regressions cover all skins'
resizing, folder-drop routing and single-click selection versus
double-click playback through the recording backend. Imported animated
volume/balance/EQ tracks and independent recolorable footer glyphs have
rendered regressions. Switching keeps the playing queue, offset and exact
window-group geometry. Focused race checks passed in both configurations.

The original three skins' 24 posed panel/resize snapshots were byte-identical
after the new import/runtime hooks. All eight imported skins were reviewed
at 1x and 1.75x, including 420×320 playlists, against their preserved
source artwork. Gaps 25 and 26 retain independent public toolkit probes;
item 17 now includes the source-confirmed font preference authoring omission.
Legacy archive decoding, printed EQ label adaptation and corrected caption
and footer ink are app import responsibilities, not new toolkit defects.

Final caption contrast regressions and the complete skin suite passed after
regeneration. Final previews again confirmed all 24 existing-skin snapshots
unchanged. Plain and all-engine builds passed. The rebuilt local player
restored Winamp Classified at 1.75x with its saved queue stopped; its native
SNI item reports Active and the app-specific icon name. The eight new
choices are included in the embedded packs and command-line help.
