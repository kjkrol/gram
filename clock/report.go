package clock

import (
	"fmt"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/render"
)

// Reporter is the clock's line for a render.TelemetryRenderer: the game time as minutes and
// seconds, and whether it stands or goes at another tempo.
func (c *Clock) Reporter() render.Reporter { return reporter{c} }

type reporter struct{ c *Clock }

func (reporter) Init(*goke.SysInit) {}

func (r reporter) Report(line func(label, value string)) { line("Game time", r.c.written()) }

// written is the clock as a player reads it: mm:ss, "(paused)" in the tactical pause, "(x2)" at
// another tempo, "(x2, held back)" when the engine could not keep a higher one up.
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

// HUD is a screen layer showing the clock in the window's bottom-left corner: the game time and,
// when it is not real time, the pause or the tempo. Add it to a scene's layers.
func (c *Clock) HUD() render.Layer { return &hud{c: c} }

type hud struct{ c *Clock }

func (*hud) Init(*goke.SysInit) {}

func (h *hud) Draw(screen *render.Image) {
	b := screen.Bounds()
	render.DebugPrintAt(screen, h.c.written(), b.Min.X+8, b.Max.Y-20)
}
