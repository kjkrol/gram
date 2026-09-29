package topography

import (
	"math"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
)

var _ goke.System = (*altitudeSystem)(nil)

// altitudeSystem puts every mover carrying a Z at the ground under its centre plus its Lift, every
// step, after movement and collisions. A flyer flown by hand holds its height over sea level
// instead, climbing and diving by the rise of the way it is steered, as far as it went along the
// ground, its Lift following. Each keeps its Clearance over the ground and, the ground allowing,
// stays under its Ceiling over sea level.
type altitudeSystem struct {
	relief *Relief

	query  *goke.Query
	base   goke.Comp[world.Base]
	z      goke.Comp[world.Z]
	mover  goke.Comp[board.Mover]
	driven goke.OptComp[steering.Driven]
	steer  goke.OptComp[steering.Steering]
}

func newAltitudeSystem(r *Relief) *altitudeSystem { return &altitudeSystem{relief: r} }

func (s *altitudeSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.base, &s.z, &s.mover).Optional(&s.driven).Optional(&s.steer).Build()
}

func (s *altitudeSystem) Update(_ *goke.CmdBuf, d time.Duration) {
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		bases, zs, movers := s.base.Slice(cursor), s.z.Slice(cursor), s.mover.Slice(cursor)
		drivens, steers := s.driven.Slice(cursor), s.steer.Slice(cursor)
		for i := range cursor.IDs {
			m := &movers[i]
			ground := s.relief.GroundAt(board.Center(bases[i].Pos))
			alt := ground + m.Lift
			flown := m.Domain&board.Air != 0 && drivens != nil && drivens[i].Flown
			if flown {
				alt = zs[i].Altitude
				if rise, run := drivens[i].Slope(); rise != 0 && steers != nil {
					speed := steers[i].Speed // as far as it went along the ground this step, the world's cap included
					along := math.Copysign(min(math.Abs(speed)*d.Seconds(), bases[i].Pos.MaxStep()), speed)
					alt += along * rise / run
				}
			}
			if held := held(m, alt, ground); flown || held != ground+m.Lift {
				alt, m.Lift = held, held-ground
			}
			zs[i].Altitude = alt
		}
	}
}

// held is alt kept under m's Ceiling and at least its Clearance over ground, the ground winning.
func held(m *board.Mover, alt, ground float64) float64 {
	if m.Ceiling > 0 {
		alt = min(alt, m.Ceiling)
	}
	return max(alt, ground+max(m.Clearance, 0))
}

var _ goke.System = (*shapingSystem)(nil)

// shapingSystem carries out the shaping commands — Raise, Lower, Level — as they come, in the
// interface part of the tick.
type shapingSystem struct {
	relief *Relief
	shape  *shaping
}

func (*shapingSystem) Init(*goke.SysInit) {}

func (s *shapingSystem) Update(*goke.CmdBuf, time.Duration) { s.shape.run(s.relief) }
