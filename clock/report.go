package clock

import (
	"fmt"
	"time"

	"github.com/kjkrol/gram/render"
)

// Reporter is the clock's line for the telemetry panel: the game time as minutes and
// seconds, and whether it stands or goes at another tempo.
func (r *Read) Reporter() render.Reporter { return reporter{r.c} }

type reporter struct{ c *Clock }

func (r reporter) Report(line func(label, value string)) { line("Game time", r.c.written()) }

func (c *Clock) written() string {
	t := c.state.Time.Round(time.Second)
	s := fmt.Sprintf("%02d:%02d", int(t.Minutes()), int(t.Seconds())%60)
	switch {
	case c.state.Paused:
		s += " (paused)"
	case c.slowed:
		s += fmt.Sprintf(" (x%g, held back)", c.state.Tempo)
	case c.state.Tempo != 1:
		s += fmt.Sprintf(" (x%g)", c.state.Tempo)
	}
	return s
}

// HUD is a screen renderer showing the clock in the window's bottom-left corner, for a ui.Drawer:
// a plugin.Publisher, reading the clock after the ticks.
func (r *Read) HUD() render.Renderer { return &hud{c: r.c} }

type hud struct {
	c       *Clock
	shown   render.Buffered[string]
	swapped bool
}

// Publish reads the clock's line for the frame drawn next.
func (h *hud) Publish() { *h.shown.Back() = h.c.written() }

// Swap has the line read last drawn.
func (h *hud) Swap() {
	h.shown.Swap()
	h.swapped = true
}

func (h *hud) Draw(screen *render.Image) {
	b := screen.Bounds()
	render.DebugPrintAt(screen, h.line(), b.Min.X+8, b.Max.Y-20)
}

// line is the line drawn: the one read last, the clock's now before any was
func (h *hud) line() string {
	if h.swapped {
		return *h.shown.Front()
	}
	return h.c.written()
}
