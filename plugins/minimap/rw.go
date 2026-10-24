package minimap

// Read reads the minimaps' settings, the same on every goroutine.
type Read struct{ p *Plugin }

// Read reads the minimaps' settings on worker w of the game's pool, 0 the game's own goroutine.
func (p *Plugin) Read(int) *Read { return &p.read }

// Write changes the minimaps' settings, on the game's goroutine.
type Write struct{ p *Plugin }

// Write changes the minimaps' settings, on the game's goroutine.
func (p *Plugin) Write() *Write { return &p.write }

// Shown reports whether the minimaps are shown.
func (r *Read) Shown() bool { return !r.p.Res.Settings.Hidden }

// Turning reports whether the minimaps turn with their cameras.
func (r *Read) Turning() bool { return r.p.Res.Settings.Turning }

// Show has the minimaps shown, or hidden (Settings.Hidden).
func (w *Write) Show(on bool) {
	p := w.p
	p.Res.Settings.Hidden = !on
	for _, m := range p.maps {
		if on {
			m.element.Show()
		} else {
			m.element.Hide()
		}
	}
}

// SetTurning has the minimaps turn with their cameras, or keep north up (Settings.Turning).
func (w *Write) SetTurning(on bool) { w.p.Res.Settings.Turning = on }

// SetReach has the minimaps show reach round their cameras, from the frame drawn after the next
// publish; past the world's last band panics.
func (w *Write) SetReach(reach float64) {
	checkReach(w.p.w, reach)
	w.p.cfg.Reach = reach
}
