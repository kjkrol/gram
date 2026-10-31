package relief

import (
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/f32"
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

	span float32 // a cell's: a body wider stands on the highest ground under its box
}

// AltitudeSystem is the goke.System keeping the movers on r at their heights, standing on what
// surfaces says, or on r's ground where it is nil, one bigger than a cell on the highest ground
// under its box. Run it in every step of the simulation, after movement and collisions.
func (r *Relief) AltitudeSystem(surfaces ground.Surfaces) goke.System {
	return &altitudeSystem{relief: r, surfaces: surfaces, span: r.grid.CellSpan()}
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
			var ground float32
			switch {
			case max(bases[i].Pos.Size.X, bases[i].Pos.Size.Y) > s.span:
				ground = s.under(&bases[i].Pos)
			case s.surfaces != nil:
				ground, _ = s.surfaces.Surface(at, m.Domain, zs[i].Altitude-m.Lift, zs[i].Reach())
			default:
				ground = s.relief.GroundAt(at)
			}
			alt := ground + m.Lift
			flown := m.Domain&cell.Air != 0 && drivens != nil && drivens[i].Flown
			if flown {
				alt = zs[i].Altitude
				if rise, run := drivens[i].Slope(); rise != 0 && courses != nil {
					speed := courses[i].Speed
					step := min(f32.Abs(speed)*float32(d.Seconds()), bases[i].Pos.MaxStep())
					along := f32.Copysign(step, speed)
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
			if pos := &bases[i].Pos; max(pos.Size.X, pos.Size.Y) > s.span {
				zs[i].Altitude = s.under(pos)
				continue
			}
			zs[i].Altitude = s.relief.GroundAt(bases[i].Pos.Center())
		}
	}
}

// under is the highest ground at the middle and the corners on the board of the box at pos, a
// body bigger than a cell's: drawn flat at the ground under its middle, the box sank into a slope
// under its corners. Such a body keeps to the ground, never on a deck.
func (s *altitudeSystem) under(pos *world.Position) float32 {
	c, h := pos.Center(), geom.NewVec(pos.Size.X/2, pos.Size.Y/2)
	ground := s.relief.GroundAt(c)
	for _, q := range [4]geom.Vec{
		geom.NewVec(c.X-h.X, c.Y-h.Y),
		geom.NewVec(c.X+h.X, c.Y-h.Y),
		geom.NewVec(c.X-h.X, c.Y+h.Y),
		geom.NewVec(c.X+h.X, c.Y+h.Y),
	} {
		if _, ok := s.relief.grid.CellAt(q); ok {
			ground = max(ground, s.relief.GroundAt(q))
		}
	}
	return ground
}

// the ground wins over the Ceiling
func held(m *unit.Mover, alt, ground float32) float32 {
	if m.Ceiling > 0 {
		alt = min(alt, m.Ceiling)
	}
	return max(alt, ground+max(m.Clearance, 0))
}
