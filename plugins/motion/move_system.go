package motion

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
)

var _ goke.System = (*moveSystem)(nil)

type moveSystem struct {
	space     *aabbworld.Space
	moveQuery *goke.Query
	base      goke.Comp[world.Base]
	drift     goke.OptComp[Drift]
	outside   goke.OptComp[Outside]
	outsideID goke.CompID
	items     []aabbworld.Item
	pace      goke.Opt
	// what the space holds: all but those beyond the last band
	held     *goke.Query
	heldBase goke.Comp[world.Base]
}

func newMoveSystem(space *aabbworld.Space) *moveSystem {
	return &moveSystem{space: space}
}

func (s *moveSystem) Init(si *goke.SysInit) {
	s.outsideID = si.RegComp[Outside]()
	qb := si.NewQueryBuilder(&s.base).Optional(&s.drift).Optional(&s.outside)
	if s.pace != nil {
		qb.With(s.pace)
	}
	s.moveQuery = qb.Build()
	s.held = si.NewQueryBuilder(&s.heldBase).Exclude(goke.Exclude[sector.Off]()).Build()
}

func (s *moveSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	for s.moveQuery.Paced(d); s.moveQuery.Next(); {
		cursor, step := s.moveQuery.Cursor(), s.moveQuery.Dt()
		// a longer step reaches as many steps' worth
		dt, reach := step.Seconds(), float64(max(step/d, 1))
		bases, drifts := s.base.Slice(cursor), s.drift.Slice(cursor)
		marked := s.outside.Present(cursor)
		for i, id := range cursor.IDs {
			rate := bases[i].Vel.Delta()
			if drifts != nil {
				rate.X += drifts[i].X
				rate.Y += drifts[i].Y
			}
			by := clampStep(geom.NewVec(rate.X*dt, rate.Y*dt), bases[i].Pos.MaxStep()*reach)
			if by.X == 0 && by.Y == 0 {
				continue
			}
			if inside := s.space.Move(&bases[i].Pos.AABB, by); !inside && !marked {
				cb.AddOne(id, s.outsideID, Outside{})
			}
		}
	}
	s.rebuild()
}

// the space holds the items handed it, so motion keeps its own
func (s *moveSystem) rebuild() {
	s.items = s.items[:0]
	for s.held.All(); s.held.Next(); {
		cursor := s.held.Cursor()
		for i, b := range s.heldBase.Slice(cursor) {
			item := aabbworld.Item{ID: cursor.IDs[i], Box: b.Pos.AABB, Caps: b.Caps}
			s.items = append(s.items, item)
		}
	}
	s.space.Rebuild(s.items)
}

func clampStep(step geom.Vec, max float64) geom.Vec {
	mag := math.Hypot(step.X, step.Y)
	if mag <= max || mag == 0 {
		return step
	}
	scale := max / mag
	return geom.NewVec(step.X*scale, step.Y*scale)
}
