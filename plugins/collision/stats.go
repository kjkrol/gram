package collision

import (
	"fmt"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/render"
)

// ContactStats is the running total of contacts collision counts, given with WithStats. It only
// grows; Reporter works a rate out of it.
type ContactStats struct {
	Counter int
}

// Reporter is the stats' lines for a render.TelemetryRenderer: contacts a second and, at the
// tick rate tps points to, contacts a tick.
func (s *ContactStats) Reporter(tps *int) render.Reporter { return &reporter{stats: s, tps: tps} }

type reporter struct {
	stats *ContactStats
	tps   *int
	rate  perSecond
}

func (*reporter) Init(*goke.SysInit) {}

func (r *reporter) Report(line func(label, value string)) {
	perSec := r.rate.observe(r.stats.Counter, time.Now())
	line("Collision/Sec", fmt.Sprintf("%0.1f", perSec))
	perTick := 0.0
	if r.tps != nil && *r.tps > 0 {
		perTick = perSec / float64(*r.tps)
	}
	line("Collisions/Tick", fmt.Sprintf("%0.2f", perTick))
}

// closed one-second windows a rate is averaged over: a few events a second read as a rate, not
// flicker
const smoothedWindows = 5

type perSecond struct {
	last  int
	since time.Time

	grown   [smoothedWindows]int
	lasted  [smoothedWindows]time.Duration
	next    int
	started bool
}

func (p *perSecond) observe(total int, now time.Time) float64 {
	switch {
	case !p.started:
		p.last, p.since, p.started = total, now, true
	case total < p.last:
		*p = perSecond{last: total, since: now, started: true}
	case now.Sub(p.since) >= time.Second:
		p.grown[p.next], p.lasted[p.next] = total-p.last, now.Sub(p.since)
		p.next = (p.next + 1) % smoothedWindows
		p.last, p.since = total, now
	}

	var grown int
	var lasted time.Duration
	for i := range p.grown {
		grown, lasted = grown+p.grown[i], lasted+p.lasted[i]
	}
	if lasted == 0 {
		return 0
	}
	return float64(grown) / lasted.Seconds()
}
