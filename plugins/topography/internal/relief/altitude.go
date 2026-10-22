package relief

import (
	"math"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
)

var _ goke.System = (*altitudeSystem)(nil)

// one carried at a motion.Drift is gravity's, which writes its height itself
type altitudeSystem struct {
	relief   *Relief
	surfaces ground.Surfaces

	query  *goke.Query
	base   goke.Comp[world.Base]
	z      goke.Comp[world.Z]
	mover  goke.Comp[unit.Mover]
	driven goke.OptComp[steering.Driven]
	course goke.OptComp[steering.Course]

	standing *goke.Query
	on       goke.Comp[cell.On]
	floor    goke.OptComp[cell.Floor]
}

// AltitudeSystem is the goke.System keeping the movers on r at their heights, standing on what
// surfaces says, or on r's ground where it is nil. Run it in every step of the simulation, after
// movement and collisions.
func (r *Relief) AltitudeSystem(surfaces ground.Surfaces) goke.System {
	return &altitudeSystem{relief: r, surfaces: surfaces}
}

func (s *altitudeSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.base, &s.z, &s.mover).Optional(&s.driven).Optional(&s.course).
		Exclude(goke.Exclude[motion.Drift]()).Build()
	s.standing = si.NewQueryBuilder(&s.base, &s.z, &s.on).Optional(&s.floor).Build()
}

func (s *altitudeSystem) Update(_ *goke.CmdBuf, d time.Duration) {
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		bases, zs, movers := s.base.Slice(cursor), s.z.Slice(cursor), s.mover.Slice(cursor)
		drivens, courses := s.driven.Slice(cursor), s.course.Slice(cursor)
		for i := range cursor.IDs {
			m := &movers[i]
			at := bases[i].Pos.Center()
			var ground float64
			if s.surfaces != nil {
				ground, _ = s.surfaces.Surface(at, m.Domain, zs[i].Altitude-m.Lift, zs[i].Reach())
			} else {
				ground = s.relief.GroundAt(at)
			}
			alt := ground + m.Lift
			flown := m.Domain&cell.Air != 0 && drivens != nil && drivens[i].Flown
			if flown {
				alt = zs[i].Altitude
				if rise, run := drivens[i].Slope(); rise != 0 && courses != nil {
					speed := courses[i].Speed
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
	for s.standing.All(); s.standing.Next(); {
		cursor := s.standing.Cursor()
		bases, zs, ons, floors := s.base.Slice(cursor), s.z.Slice(cursor), s.on.Slice(cursor),
			s.floor.Slice(cursor)
		for i := range zs {
			if floors != nil {
				zs[i].Altitude = s.relief.Altitude(ons[i].Cell) + floors[i].Height
				continue
			}
			zs[i].Altitude = s.relief.GroundAt(bases[i].Pos.Center())
		}
	}
}

// the ground wins over the Ceiling
func held(m *unit.Mover, alt, ground float64) float64 {
	if m.Ceiling > 0 {
		alt = min(alt, m.Ceiling)
	}
	return max(alt, ground+max(m.Clearance, 0))
}
