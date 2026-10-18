package world

import (
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world/sector"
)

var _ goke.System = (*indexSystem)(nil)

// indexSystem hands the space every entity after a spawn or a despawn; one moved is indexed
// again by whoever moved it
type indexSystem struct {
	w     *module
	query *goke.Query
	base  goke.Comp[Base]
}

func newIndexSystem(w *module) *indexSystem { return &indexSystem{w: w} }

func (s *indexSystem) Init(si *goke.SysInit) { s.query = heldQuery(si, &s.base) }

// what the space holds: every entity but those beyond the last band of the sectors
func heldQuery(si *goke.SysInit, base *goke.Comp[Base]) *goke.Query {
	return si.NewQueryBuilder(base).Exclude(goke.Exclude[sector.Off]()).Build()
}

func (s *indexSystem) Update(*goke.CmdBuf, time.Duration) {
	if s.w.unindexed {
		s.w.items = rebuild(s.w.space, s.query, &s.base, s.w.items)
		s.w.unindexed = false
	}
}

func rebuild(
	space *aabbworld.Space,
	q *goke.Query,
	base *goke.Comp[Base],
	items []aabbworld.Item,
) []aabbworld.Item {
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
