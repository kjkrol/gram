package climate

import (
	"fmt"
	"math"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/render"
)

var _ render.Reporter = (*report)(nil)

type report struct {
	names []string
	query *goke.Query
	now   goke.Comp[Weather]
}

func (r *report) Init(si *goke.SysInit) { r.query = si.NewQueryBuilder(&r.now).Build() }

func (r *report) Report(line func(label, value string)) {
	for r.query.All(); r.query.Next(); {
		w := r.now.Slice(r.query.Cursor())[0]
		name := "?" // from an older save
		if w.State >= 0 && int(w.State) < len(r.names) {
			name = r.names[w.State]
		}
		if w.Snow > w.Rain && w.Snow > 0.05 {
			name += " (snow)"
		}
		s := fmt.Sprintf("%s, %.0f°C, wind %.0f %s", name, w.Temperature, w.Blow, compass(w.Heading))
		line("Weather", s)
		return
	}
}

// heading in radians from +x, east, with y running south
func compass(heading float32) string {
	ways := [...]string{"E", "SE", "S", "SW", "W", "NW", "N", "NE"}
	at := int(math.Round(float64(heading)/(math.Pi/4))) % 8
	if at < 0 {
		at += 8
	}
	return ways[at]
}
