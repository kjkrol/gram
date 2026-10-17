package motion

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world"
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
}

func newMoveSystem(space *aabbworld.Space) *moveSystem {
	return &moveSystem{space: space}
}

func (s *moveSystem) Init(si *goke.SysInit) {
	s.outsideID = si.RegComp[Outside]()
	s.moveQuery = si.NewQueryBuilder(&s.base).Optional(&s.drift).Optional(&s.outside).Build()
}

func (s *moveSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	dt := d.Seconds()
	s.moveQuery.All()
	for s.moveQuery.Next() {
		cursor := s.moveQuery.Cursor()
		bases, drifts := s.base.Slice(cursor), s.drift.Slice(cursor)
		marked := s.outside.Present(cursor)
		for i, id := range cursor.IDs {
			rate := bases[i].Vel.Delta()
			if drifts != nil {
				rate.X += drifts[i].X
				rate.Y += drifts[i].Y
			}
			step := clampStep(geom.NewVec(rate.X*dt, rate.Y*dt), bases[i].Pos.MaxStep())
			if step.X == 0 && step.Y == 0 {
				continue
			}
			if inside := s.space.Move(&bases[i].Pos.AABB, step); !inside && !marked {
				cb.AddOne(id, s.outsideID, Outside{})
			}
		}
	}
	s.rebuild()
}

// the space holds the items handed it, so motion keeps its own
func (s *moveSystem) rebuild() {
	s.items = s.items[:0]
	s.moveQuery.All()
	for s.moveQuery.Next() {
		cursor := s.moveQuery.Cursor()
		for i, b := range s.base.Slice(cursor) {
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
