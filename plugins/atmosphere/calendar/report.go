package calendar

import (
	"fmt"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/render"
)

// Reporter is the calendar's lines for a render.TelemetryRenderer: the time of day and the date.
func (c *Calendar) Reporter() render.Reporter { return reporter{c} }

type reporter struct{ c *Calendar }

func (reporter) Init(*goke.SysInit) {}

func (r reporter) Report(line func(label, value string)) {
	m := r.c.Now()
	line("Time of day", m.Hour())
	line("Season", fmt.Sprintf("%s, %s, %s", m.Season(), m.Written(), m.MoonName()))
}

// HUD is a screen layer showing the calendar in the window's bottom-right corner: the hour, the
// date and the moon. Add it to a scene's layers.
func (c *Calendar) HUD() render.Layer { return &hud{c: c} }

type hud struct{ c *Calendar }

func (*hud) Init(*goke.SysInit) {}

func (h *hud) Draw(screen *render.Image) {
	m := h.c.Now()
	text := fmt.Sprintf("%s  %s, %s  %s", m.Hour(), m.Season(), m.Written(), m.MoonName())
	b := screen.Bounds()
	render.DebugPrintAt(screen, text, b.Max.X-8-6*len(text), b.Max.Y-20)
}
