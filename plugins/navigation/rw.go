package navigation

import "github.com/kjkrol/gram/render"

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

// Hierarchy is how the long routes are planned: as asked until Use, then as decided, its Side and
// From set.
func (r *Read) Hierarchy() Hierarchy { return r.p.hierarchy }

// RoutesShown reports whether the routes are drawn.
func (r *Read) RoutesShown() bool { return r.p.Res.Settings.Routes }

// Counts are what the routing did since the plugin was made, its routers' and its traffic's
// searches together; none before Use. Read them on the game's goroutine, between its ticks.
func (r *Read) Counts() Counts {
	p := r.p
	if p.coordinator == nil {
		return Counts{}
	}
	c, f := p.coordinator, p.finder
	return Counts{
		Searches:   c.Searches() + f.Searches(),
		Walks:      c.Walks() + f.Walks(),
		Fills:      c.Fills() + f.Fills(),
		Climbs:     c.Climbs() + f.Climbs(),
		Builds:     c.Builds() + f.Builds(),
		Expansions: c.Expansions() + f.Expansions(),
	}
}

// Reporter is navigation's line on the telemetry panel: the Counts.
func (r *Read) Reporter() render.Reporter { return reporter{r} }

// ShowRoutes has the selected units' routes drawn, or their goals alone (Settings.Routes).
func (w *Write) ShowRoutes(shown bool) {
	w.p.Res.Settings.Routes = shown
	w.p.pathRenderer.ShowRoutes(shown)
}
