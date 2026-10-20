package world

import (
	"unsafe"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/uid"
)

// indexer gathers the space's runs anew once the chunks they lie in have changed
type indexer struct {
	w     *module
	query *goke.Query
	base  goke.Comp[Base]
}

func (x *indexer) init(si *goke.SysInit) { x.query = heldQuery(si, &x.base) }

// what the space holds: every entity but those beyond the last band of the sectors
func heldQuery(si *goke.SysInit, base *goke.Comp[Base]) *goke.Query {
	return si.NewQueryBuilder(base).Exclude(goke.Exclude[sector.Off]()).Build()
}

func (x *indexer) settle() {
	if x.w.moved(x.query) {
		x.w.index(x.query, &x.base)
	}
}

// RunOf is the space's run over bases, a chunk's Base column named by ids: the space reads the
// boxes and their Caps where they lie and pushes the boxes in place.
func RunOf(ids []uid.UID64, bases []Base) aabbworld.Run {
	return aabbworld.RunOf(ids, bases, boxOf, capsOf)
}

func boxOf(b *Base) *plane.AABB { return &b.Pos.AABB }

func capsOf(b *Base) *aabbworld.Capability { return &b.Caps }

func (w *module) index(q *goke.Query, base *goke.Comp[Base]) {
	w.runs, w.versions = w.runs[:0], w.versions[:0]
	for q.All(); q.Next(); {
		cursor := q.Cursor()
		w.runs = append(w.runs, RunOf(cursor.IDs, base.Slice(cursor)))
		w.versions = append(w.versions, q.ChunkSnapshot().TableVer)
	}
	w.space.RebuildRuns(w.runs, nil)
}

// moved reports whether a chunk q walks has come, gone, grown or shrunk since the runs were
// gathered, or goke has moved entities of its table between slots
func (w *module) moved(q *goke.Query) bool {
	k := 0
	for q.All(); q.Next(); k++ {
		cursor := q.Cursor()
		if k == len(w.runs) {
			return true
		}
		ids := w.runs[k].IDs
		if len(ids) != len(cursor.IDs) || unsafe.SliceData(ids) != unsafe.SliceData(cursor.IDs) ||
			w.versions[k] != q.ChunkSnapshot().TableVer {
			return true
		}
	}
	return k != len(w.runs)
}
