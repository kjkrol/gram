package clock

import (
	"fmt"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/render"
)

// Reporter is the clock's line for the telemetry panel: the game time as minutes and
// seconds, and whether it stands or goes at another tempo.
func (r *Read) Reporter() render.Reporter { return reporter{r.c} }

type reporter struct{ c *Clock }

func (reporter) Init(*goke.SysInit) {}

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

// HUD is a screen renderer showing the clock in the window's bottom-left corner, for a ui.Drawer.
func (r *Read) HUD() render.Renderer { return &hud{c: r.c} }

type hud struct{ c *Clock }

func (*hud) Init(*goke.SysInit) {}

func (h *hud) Draw(screen *render.Image) {
	b := screen.Bounds()
	render.DebugPrintAt(screen, h.c.written(), b.Min.X+8, b.Max.Y-20)
}
