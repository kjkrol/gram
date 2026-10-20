package calendar

import (
	"fmt"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/render"
)

// Reporter is the calendar's lines for the telemetry panel.
func (c *Calendar) Reporter() render.Reporter { return reporter{c} }

type reporter struct{ c *Calendar }

func (reporter) Init(*goke.SysInit) {}

func (r reporter) Report(line func(label, value string)) {
	m := r.c.Now()
	line("Time of day", m.Hour())
	line("Season", fmt.Sprintf("%s, %s", m.Season(), m.Written()))
	line("Moon", m.MoonName())
}
