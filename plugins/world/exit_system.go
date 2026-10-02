package world

import (
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/uid"
)

// Outside marks an entity wholly past an open edge; whoever moved it there puts it on, the
// exitSystem takes it off once the entity is back inside.
type Outside struct{}

// Leaving is what a rule hosted by world gets, every tick, for an entity carrying Outside.
// To reconsider: no game hooks a rule on it yet, so the world despawns every leaver — whether it
// stays a moment or the world simply despawns is open (doc/refactor-notes.md, Questions for review).
type Leaving struct {
	ID   uid.UID64
	Base *Base
}

// Who is the entity leaving: whose moment it is, for a rule.
func (l Leaving) Who() uid.UID64 { return l.ID }

var _ goke.System = (*exitSystem)(nil)

// exitSystem despawns the entities that gave themselves a Despawn, and walks those carrying
// Outside: the hosted rules hear of them, or they are despawned when there are none; one that
// is back inside loses the mark.
type exitSystem struct {
	w    *module
	host *host.EachHost[Leaving]

	query   *goke.Query
	base    goke.Comp[Base]
	outside goke.CompID

	// alive finds whoever gave itself a Despawn: one gone already is not despawned again
	alive     *goke.Query
	aliveBase goke.Comp[Base]

	ids   []uid.UID64
	bases []Base
}

func newExitSystem(w *module, host *host.EachHost[Leaving]) *exitSystem {
	return &exitSystem{w: w, host: host}
}

func (s *exitSystem) Init(si *goke.SysInit) {
	s.outside = si.RegComp[Outside]()
	qb := si.NewQueryBuilder(&s.base).Include(goke.Include[Outside]())
	s.host.Bind(qb)
	s.query = qb.Build()
	s.alive = si.NewQueryBuilder(&s.aliveBase).Build()
}

func (s *exitSystem) Update(cb *goke.CmdBuf, d time.Duration) {
	s.w.despawns.Drain(func(i control.Issued[Despawn]) {
		if i.ByEntity && s.alive.Seek(i.Entity) {
			s.w.despawn(cb, i.Entity)
		}
	})
	tick := s.w.tick(cb, d)
	s.query.All()
	for s.query.Next() {
		cursor := s.query.Cursor()
		s.ids, s.bases = cursor.IDs, s.base.Slice(cursor)
		if !s.host.Empty() {
			s.host.Run(tick, cursor, s.at)
		}
		for i, id := range s.ids {
			switch {
			case inside(s.w.space, s.bases[i].Pos.AABB.AABB):
				cb.RemoveCompOne(id, s.outside)
			case s.host.Empty():
				s.w.despawn(cb, id)
			}
		}
	}
}

// at describes the i-th entity of the chunk being walked.
func (s *exitSystem) at(i int) Leaving { return Leaving{ID: s.ids[i], Base: &s.bases[i]} }

// inside reports whether box is not wholly past an open edge of space.
func inside(space *aabbworld.Space, box geom.AABB) bool {
	width, height, edges := space.Bounds()
	outX := edges&aabbworld.OpenX != 0 && (box.BottomRight.X <= 0 || box.TopLeft.X >= float64(width))
	outY := edges&aabbworld.OpenY != 0 && (box.BottomRight.Y <= 0 || box.TopLeft.Y >= float64(height))
	return !outX && !outY
}
