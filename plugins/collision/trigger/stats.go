package trigger

import (
	"fmt"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world/act"
	"github.com/kjkrol/gram/render"
)

// CountContacts adds every contact it is handed to a ContactStats the game owns.
func CountContacts(stats *ContactStats) act.Instant {
	var r act.Reaction[collision.Meeting]
	return r.Run(func(plugin.Tick, collision.Meeting) { stats.Counter++ })
}

// ContactStats is the running total of contacts CountContacts maintains. It only grows; Reporter
// works a rate out of it.
type ContactStats struct {
	Counter int
}

// Reset zeroes Counter, for a game that wants to start counting afresh.
func (s *ContactStats) Reset() { s.Counter = 0 }

// Reporter is the stats' lines for a render.TelemetryRenderer: contacts a second, averaged over
// the last few, and contacts a tick at the tick rate tps points to.
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

// smoothedWindows is how many closed one-second windows a rate is averaged over:
// enough that a few events a second read as a rate rather than as flicker.
const smoothedWindows = 5

// perSecond turns a running total, looked at as often as anyone likes, into how
// fast it has been growing over the last few seconds.
type perSecond struct {
	last  int
	since time.Time

	grown   [smoothedWindows]int
	lasted  [smoothedWindows]time.Duration
	next    int
	started bool
}

// observe takes the total as of now and returns its growth a second, averaged over closed windows.
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
