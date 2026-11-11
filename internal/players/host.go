package players

import (
	"strings"
	"time"

	"goframework.io/desktop-ui"
	uiskins "goframework.io/desktop-ui-skins"
	"goframework.io/desktop-ui-widgets"
	"goframework.io/desktop-ui/platform"
	"goframework.io/desktop-ui/style"
	"goframework.io/desktop-ui/widget"
	"goframework.io/graphics-paint2d"
)

// Model holds the player state shared by the default UI and its optional skins.
// Changing the appearance replaces windows without replacing this model.
type Model struct {
	Transport *Transport
	Spectrum  *Spectrum
	Equaliser *Equalizer

	EqualizerVisible bool
	PlaylistVisible  bool
	// Live distinguishes actual playback from deterministic screenshot fixtures.
	Live  bool
	Error string
	// Screenshot fixtures keep every control available. Live playback sets
	// these capabilities without changing the saved EQ or balance values.
	BackendLimited                                    bool
	BackendEqualizer, BackendBalance, BackendSpectrum bool
	// Cover is the front cover of the file at CoverPath (its ID3 APIC
	// picture), for the now-playing display; nil when it has none.
	Cover     *paint2d.Image
	CoverPath string
}

// NowPlayingCover returns the cover of the track playing, if it has one.
func (m *Model) NowPlayingCover() *paint2d.Image {
	if m == nil || m.Cover == nil || m.Transport == nil || m.CoverPath == "" || m.CoverPath != m.Transport.Track().Path {
		return nil
	}
	return m.Cover
}

func NewModel() *Model {
	return &Model{
		Transport:        NewTransport(NewLibrary()),
		Spectrum:         NewSpectrum(48),
		Equaliser:        NewEqualizer(),
		EqualizerVisible: true,
		PlaylistVisible:  true,
	}
}

// DefaultSkin selects the ordinary UI without a skin pack.
const DefaultSkin = "default"

// Themed is the ordinary look used by the default UI. Breeze matches the
// former unskinned Lantern UI; a build without that engine uses dark.
var Themed = unskinnedTheme()

func unskinnedTheme() string {
	if _, ok := style.LoadTheme("breeze-night"); ok && !uiskins.IsSkin("breeze-night") {
		return "breeze-night"
	}
	return "dark"
}

// Host gives a presentation the model and the application's actions.
// Hooks are optional, so a presentation can also run on its own in tests.
type Host struct {
	App          *ui.Application
	Model        *Model
	AudioBackend string

	Run    func(Command)
	Shared func(w *ui.Window, e widget.KeyEvent) bool
	Packs  func() []string
	// ChooseSkin lets the application switch between the ordinary UI and
	// the panel windows. Its implementation schedules window replacement
	// after the control or menu requesting it has finished its event.
	ChooseSkin func(string)
	Wrote      func(string)
	LoadFiles  func(widget.Component, bool)
	// DropFiles appends local files or folders supplied by a playlist drop.
	// The application expands folders and loads the resulting music files.
	DropFiles    func([]string)
	DeleteTrack  func()
	SyncPlayback func()
	ShowSettings func()

	lastSkin string
}

func NewHost(a *ui.Application) *Host { return &Host{App: a, Model: NewModel()} }

func (h *Host) Command(c Command) {
	if h.Run != nil {
		h.Run(c)
		return
	}
	h.Model.Transport.Do(c)
}

func (h *Host) Play()               { h.Command(CmdPlay) }
func (h *Host) Seek(f float32)      { h.Model.Transport.Seek(f); h.syncPlayback() }
func (h *Host) SetVolume(v float32) { h.Model.Transport.SetVolume(v); h.syncPlayback() }
func (h *Host) SetPan(v float32)    { h.Model.Transport.SetPan(v); h.syncPlayback() }
func (h *Host) SelectTrack(i int) {
	if i < 0 || i >= h.Model.Transport.List.Len() {
		return
	}
	h.Model.Transport.SelectTrack(i)
	h.Play()
}
func (h *Host) OpenFiles(from widget.Component, appendToList bool) {
	if h.LoadFiles != nil {
		h.LoadFiles(from, appendToList)
	}
}
func (h *Host) RemoveTrack() {
	if h.DeleteTrack != nil {
		h.DeleteTrack()
	}
}

func (h *Host) OpenSettings() {
	if h.ShowSettings != nil {
		h.ShowSettings()
	}
}

// AcceptFileDrop routes a copy of decoded local paths to the application.
// The player loads local media; a mixed local/remote drop is rejected as a
// whole so the user does not receive a silently incomplete queue insertion.
func (h *Host) AcceptFileDrop(e widget.DropEvent) bool {
	TraceFileDrop("delivered mime=%q action=%s data_bytes=%d paths=%q", e.Mime, e.Action, len(e.Data), e.Paths)
	if h == nil || h.DropFiles == nil || e.Mime != "text/uri-list" || len(e.Paths) == 0 {
		TraceFileDrop("rejected: missing loader, URI-list type or decoded paths")
		return false
	}
	if e.Action != platform.DragNone && e.Action != platform.DragCopy {
		TraceFileDrop("rejected: source action is %s", e.Action)
		return false
	}
	if len(e.Remote) > 0 {
		TraceFileDrop("rejected: unsupported remote URI")
		return false
	}
	h.DropFiles(append([]string(nil), e.Paths...))
	TraceFileDrop("accepted for background loading")
	return true
}
func (h *Host) syncPlayback() {
	if h.SyncPlayback != nil {
		h.SyncPlayback()
	}
}
func (h *Host) StatusText() string {
	if h.Model.Error != "" {
		return h.Model.Error
	}
	if !h.Model.Live {
		return "Screenshot fixture · simulated playback"
	}
	backend := h.AudioBackend
	if backend == "No backend" {
		return "No audio backend available · choose one in Settings"
	}
	if backend == "" {
		backend = "mpv"
	}
	if h.Model.Transport.List.Len() == 0 {
		return "Open a music file to play · " + backend
	}
	status := "Playback through " + backend
	if h.Model.BackendLimited {
		var unavailable []string
		if !h.Model.BackendEqualizer {
			unavailable = append(unavailable, "EQ")
		}
		if !h.Model.BackendBalance {
			unavailable = append(unavailable, "balance")
		}
		if !h.Model.BackendSpectrum {
			unavailable = append(unavailable, "visualizer")
		}
		if len(unavailable) > 0 {
			status += " · " + strings.Join(unavailable, ", ") + " unavailable"
		}
	}
	return status
}

func (h *Host) EqualizerAvailable() bool { return !h.Model.BackendLimited || h.Model.BackendEqualizer }
func (h *Host) BalanceAvailable() bool   { return !h.Model.BackendLimited || h.Model.BackendBalance }
func (h *Host) SpectrumAvailable() bool  { return !h.Model.BackendLimited || h.Model.BackendSpectrum }
func (h *Host) FeatureTip(feature string, available bool) string {
	if available {
		return ""
	}
	backend := h.AudioBackend
	if backend == "" {
		backend = "the selected backend"
	}
	return feature + " unavailable with " + backend + " · change the backend in Settings"
}

func (h *Host) Keys(w *ui.Window, e widget.KeyEvent) bool {
	if h.Shared != nil {
		return h.Shared(w, e)
	}
	if Typing(w.Focus()) {
		return false
	}
	if c := CommandFor(e); c != CmdNone {
		h.Command(c)
		return true
	}
	return false
}

// Pose produces a reproducible screenshot without running the clock.
func (h *Host) Pose(pos time.Duration) {
	t := h.Model.Transport
	t.State, t.Pos = Playing, pos
	h.Model.Spectrum.Reset()
	h.Model.Spectrum.Advance(pos, true, 250*time.Millisecond)
}

func (h *Host) Skins() []string {
	if h.Packs == nil {
		return nil
	}
	return h.Packs()
}

func (h *Host) Worn() string  { return style.LookAppearance(h.App.Look()).Name }
func (h *Host) Skinned() bool { return h.IsSkin(h.Worn()) }

func (h *Host) IsSkin(id string) bool {
	for _, skin := range h.Skins() {
		if id == skin {
			return true
		}
	}
	return false
}

// SetSkin asks the application to change appearance. A standalone host
// applies the look directly instead.
func (h *Host) SetSkin(id string) {
	if id == "" || h.App == nil {
		return
	}
	if h.ChooseSkin != nil {
		h.ChooseSkin(id)
		return
	}
	h.ApplySkin(id)
	h.RememberSkin(id)
}

// ApplySkin changes the toolkit look without calling ChooseSkin. The
// shell uses it while replacing windows around the shared model.
func (h *Host) ApplySkin(id string) {
	if id == "" || h.App == nil {
		return
	}
	if id == DefaultSkin {
		id = Themed
	}
	ap := style.LookAppearance(h.App.Look())
	ap.FollowDesktop, ap.Name = false, id
	h.App.SetLook(style.WithAppearance(h.App.Look(), ap))
	if h.Wrote != nil {
		h.Wrote(id)
	}
}

// RememberSkin records a successfully applied optional skin. The shell
// calls it after its windows open so a failed switch cannot replace it.
func (h *Host) RememberSkin(id string) {
	if h.IsSkin(id) {
		h.lastSkin = id
	}
}

// NextSkin cycles through the optional skins and returns to the default UI.
func (h *Host) NextSkin() {
	skins := append(append([]string(nil), h.Skins()...), Themed)
	if len(skins) < 2 {
		return
	}
	for i, id := range skins {
		if id == h.Worn() {
			h.SetSkin(skins[(i+1)%len(skins)])
			return
		}
	}
	h.SetSkin(skins[0])
}

func (h *Host) DropSkin() {
	if h.Skinned() {
		h.SetSkin(Themed)
		return
	}
	id := h.lastSkin
	if id == "" && len(h.Skins()) > 0 {
		id = h.Skins()[0]
	}
	h.SetSkin(id)
}

type NamedWindow struct {
	Name   string
	Window *ui.Window
}

func (h *Host) PackLabel(id string) string {
	return SkinLabel(id)
}

// SkinItems is the application's complete appearance menu.
func (h *Host) SkinItems() []*widgets.MenuItem {
	cur := h.Worn()
	var items []*widgets.MenuItem
	for _, id := range append([]string{Themed}, h.Skins()...) {
		id := id
		items = append(items, widgets.RadioItem(h.PackLabel(id), "skin", cur == id, func() { h.SetSkin(id) }))
	}
	return items
}

func (h *Host) ContextMenu(from widget.Component, at paint2d.Point) {
	items := h.SkinItems()
	if h.ShowSettings != nil {
		items = append(items, widgets.Sep(), widgets.ItemAccel("Settings…", "Ctrl+,", h.OpenSettings))
	}
	if len(items) > 0 {
		widgets.ShowContextMenu(from, at, items...)
	}
}
