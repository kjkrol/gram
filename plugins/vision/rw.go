package vision

// Read reads vision's settings, the same on every goroutine.
type Read struct{ p *Plugin }

// Read reads vision's settings on worker w of the game's pool, 0 the game's own goroutine.
func (p *Plugin) Read(int) *Read { return &p.read }

// Write changes vision's settings, on the game's goroutine.
type Write struct{ p *Plugin }

// Write changes vision's settings, on the game's goroutine.
func (p *Plugin) Write() *Write { return &p.write }

// ConesShown reports whether the views drawn are shown.
func (r *Read) ConesShown() bool { return r.p.Res.Settings.Cones }

// ShowCones shows every view drawn, as the Cones command does (Settings.Cones).
func (w *Write) ShowCones() { w.p.setHidden(false) }

// HideCones hides every view drawn; the scan goes on.
func (w *Write) HideCones() { w.p.setHidden(true) }
