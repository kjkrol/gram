package sky

import (
	"fmt"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/render"
)

var _ render.Reporter = (*clock)(nil)

// clock is the sky's line in the telemetry: the time of day, and whether it stands or at what pace
// it goes by when not 1.
type clock struct {
	query *goke.Query
	day   goke.Comp[Day]
}

func (c *clock) Init(si *goke.SysInit) { c.query = si.NewQueryBuilder(&c.day).Build() }

func (c *clock) Report(line func(label, value string)) {
	for c.query.All(); c.query.Next(); {
		day := c.day.Slice(c.query.Cursor())[0]
		line("Time of day", hourOf(day))
		line("Season", fmt.Sprintf("%s, %s, %s", day.Season(), day.Written(), moonOf(day.Moon())))
		return
	}
}

// hourOf is the day's time as hours and minutes, and whether it stands or its pace when hurried or
// held.
func hourOf(d Day) string {
	minutes := int(d.Time*24*60+0.5) % (24 * 60)
	s := fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
	switch {
	case d.Stopped:
		s += " (stopped)"
	case d.Pace != 1:
		s += fmt.Sprintf(" (x%g)", d.Pace)
	}
	return s
}

// moonOf is the moon's phase by name.
func moonOf(moon float32) string {
	names := [...]string{"new moon", "waxing crescent", "first quarter", "waxing gibbous", "full moon", "waning gibbous", "last quarter", "waning crescent"}
	return names[int(moon*8+0.5)%8]
}
