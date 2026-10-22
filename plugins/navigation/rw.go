package navigation

// Read reads navigation's settings, the same on every goroutine.
type Read struct{ p *Plugin }

// Read reads navigation's settings on worker w of the game's pool, 0 the game's own goroutine.
func (p *Plugin) Read(int) *Read { return &p.read }

// Write changes navigation's settings, on the game's goroutine.
type Write struct{ p *Plugin }

// Write changes navigation's settings, on the game's goroutine.
func (p *Plugin) Write() *Write { return &p.write }

// Spacing is how units keep out of each other's way: as asked until Use, then as decided.
func (r *Read) Spacing() Spacing { return r.p.spacing }

// RoutesShown reports whether the routes are drawn.
func (r *Read) RoutesShown() bool { return r.p.Res.Settings.Routes }

// ShowRoutes has the selected units' routes drawn, or their goals alone (Settings.Routes).
func (w *Write) ShowRoutes(shown bool) {
	w.p.Res.Settings.Routes = shown
	w.p.pathRenderer.ShowRoutes(shown)
}
