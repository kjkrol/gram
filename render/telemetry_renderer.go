package render

import (
	"fmt"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/kjkrol/goke/v3"
)

var _ Renderer = (*TelemetryRenderer)(nil)

// TelemetryRenderer prints how the game runs in the corner of the screen: the tick rate, the
// entity count, collisions, and the lines of every Reporter it was built With.
type TelemetryRenderer struct {
	measuredTPS    *int
	entityCount    func() int
	collisionTotal *int
	collisions     perSecond
	reporters      []Reporter
	text           []byte
}

// NewTelemetryRenderer shows tick rate, entity count and collisions a second from a running total.
func NewTelemetryRenderer(measuredTPS *int, entityCount func() int, collisionTotal *int) *TelemetryRenderer {
	return &TelemetryRenderer{
		measuredTPS:    measuredTPS,
		entityCount:    entityCount,
		collisionTotal: collisionTotal,
	}
}

// With adds the lines of reporters, in order, under the engine's own.
func (s *TelemetryRenderer) With(reporters ...Reporter) *TelemetryRenderer {
	s.reporters = append(s.reporters, reporters...)
	return s
}

func (s *TelemetryRenderer) Init(si *goke.SysInit) {
	for _, r := range s.reporters {
		r.Init(si)
	}
}

func (s *TelemetryRenderer) Draw(screen *ebiten.Image) {
	collisionsPerSec := s.collisions.observe(*s.collisionTotal, time.Now())
	avgCollisionsPerTick := float64(0)
	if *s.measuredTPS > 0 {
		avgCollisionsPerTick = collisionsPerSec / float64(*s.measuredTPS)
	}
	s.text = fmt.Appendf(s.text[:0],
		"FPS: %0.2f\nTPS (Ebiten): %0.2f\nTPS (Physics): %d\nEntities: %d\nCollision/Sec: %0.1f\nCollisions/Tick: %0.2f",
		ebiten.ActualFPS(),
		ebiten.ActualTPS(),
		*s.measuredTPS,
		s.entityCount(),
		collisionsPerSec,
		avgCollisionsPerTick,
	)
	s.text = s.reported(s.text)
	ebitenutil.DebugPrint(screen, string(s.text))
}

// reported appends to text a line "label: value" for every line of the reporters.
func (s *TelemetryRenderer) reported(text []byte) []byte {
	for _, r := range s.reporters {
		r.Report(func(label, value string) {
			text = append(append(append(append(text, '\n'), label...), ": "...), value...)
		})
	}
	return text
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
