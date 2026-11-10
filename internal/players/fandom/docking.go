package fandom

// Native move bursts and outstanding request deadlines are managed by the
// toolkit Desk. The application follows explicitly for capture, resizing and
// backends without move events; startup and restoration retain their retries.
func (p *Player) followWindows() {
	p.Desk.Follow()
}

func (p *Player) needsFollow(dtZero bool) bool {
	for i := 0; i < p.Desk.Rack.Len(); i++ {
		w, pane := p.Desk.Window(i), p.Desk.Rack.Pane(i)
		if w == nil || w.Closed() || pane == nil {
			continue
		}
		width, height := w.Size()
		if dtZero || !w.SendsMoveEvents() || width != pane.Box.W || height != pane.Box.H {
			return true
		}
	}
	return false
}
