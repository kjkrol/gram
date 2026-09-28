package sky

import (
	"fmt"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/render"
)

// Reporter is the sky's line for a render.TelemetryRenderer: whether the light goes with the
// calendar or stands frozen, and at what hour.
func (s *Sky) Reporter() render.Reporter { return reporter{s} }

type reporter struct{ s *Sky }

func (reporter) Init(*goke.SysInit) {}

func (r reporter) Report(line func(label, value string)) {
	if !r.s.frozen {
		line("Light", "the hour's")
		return
	}
	minutes := int(r.s.hour*24*60+0.5) % (24 * 60)
	line("Light", fmt.Sprintf("frozen at %02d:%02d", minutes/60, minutes%60))
}
