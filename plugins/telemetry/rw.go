package telemetry

import (
	"slices"

	"github.com/kjkrol/gram/game"
)

// Read reads the telemetry panel's state, the same on every goroutine.
type Read struct{ p *Plugin }

// Read reads the telemetry panel's state on worker w of the game's pool, 0 the game's own
// goroutine.
func (p *Plugin) Read(int) *Read { return &p.read }

// Write changes the telemetry panel's state, on the game's goroutine.
type Write struct{ p *Plugin }

// Write changes the telemetry panel's state, on the game's goroutine.
func (p *Plugin) Write() *Write { return &p.write }

// Shown says whether the panel is shown.
func (r *Read) Shown(c game.Composition) bool { return slices.Contains(c.Order(), SceneName) }

// Mode is what the panel shows.
func (r *Read) Mode() Mode {
	p := r.p
	if p.Res.Settings.Extended {
		return Extended
	}
	return Basic
}

// Show shows the panel on top of the scenes shown.
func (w *Write) Show(c game.Composition) { c.Show(SceneName) }

// Hide hides the panel.
func (w *Write) Hide(c game.Composition) { c.Hide(SceneName) }

// Toggle shows the panel, or hides it when shown.
func (w *Write) Toggle(c game.Composition) {
	if w.p.read.Shown(c) {
		w.Hide(c)
	} else {
		w.Show(c)
	}
}

// SetMode has the panel show the basics or every line (Settings.Extended).
func (w *Write) SetMode(m Mode) {
	w.p.Res.Settings.Extended = m == Extended
}
