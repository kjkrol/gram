package world

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
)

var _ goke.System = (*moveSystem)(nil)

// moveSystem integrates each entity's already speed-scaled Velocity into Position under the
// space's edge rules, marks whoever left by an open edge Outside, then rebuilds the space.
type moveSystem struct {
	space     *aabbworld.Space
	moveQuery *goke.Query
	base      goke.Comp[Base]
	outside   goke.OptComp[Outside]
	outsideID goke.CompID
	items     []aabbworld.Item
}

// newMoveSystem builds world's movement system; no entity moves past its Position.MaxStep a tick.
func newMoveSystem(space *aabbworld.Space) *moveSystem {
	return &moveSystem{space: space}
}

func (s *moveSystem) Init(si *goke.SysInit) {
	s.outsideID = si.RegComp[Outside]()
	s.moveQuery = si.NewQueryBuilder(&s.base).Optional(&s.outside).Build()
}

func (s *moveSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	dt := d.Seconds()
	s.moveQuery.All()
	for s.moveQuery.Next() {
		cursor := s.moveQuery.Cursor()
		bases := s.base.Slice(cursor)
		marked := s.outside.Present(cursor)
		for i, id := range cursor.IDs {
			rate := bases[i].Vel.Delta()
			step := clampStep(geom.NewVec(rate.X*dt, rate.Y*dt), bases[i].Pos.MaxStep())
			if step.X == 0 && step.Y == 0 {
				continue
			}
			if inside := s.space.Move(&bases[i].Pos.AABB, step); !inside && !marked {
				cb.AddOne(id, s.outsideID, Outside{})
			}
		}
	}
	s.items = rebuild(s.space, s.moveQuery, &s.base, s.items)
}

// rebuild hands space every Base the query finds, reusing items.
func rebuild(space *aabbworld.Space, q *goke.Query, base *goke.Comp[Base], items []aabbworld.Item) []aabbworld.Item {
	items = items[:0]
	q.All()
	for q.Next() {
		cursor := q.Cursor()
		for i, b := range base.Slice(cursor) {
			items = append(items, aabbworld.Item{ID: cursor.IDs[i], Box: b.Pos.AABB, Caps: b.Caps})
		}
	}
	space.Rebuild(items)
	return items
}

// clampStep scales step down to magnitude max if it exceeds it.
func clampStep(step geom.Vec, max float64) geom.Vec {
	mag := math.Hypot(step.X, step.Y)
	if mag <= max || mag == 0 {
		return step
	}
	scale := max / mag
	return geom.NewVec(step.X*scale, step.Y*scale)
}
