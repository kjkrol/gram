package world

import (
	"unsafe"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/uid"
)

// indexer gathers the space's runs anew once the chunks they lie in have changed, the moving
// layer's and the still one's each on its own: what stands for good is indexed once
type indexer struct {
	w         *module
	query     *goke.Query
	base      goke.Comp[Base]
	still     *goke.Query
	stillBase goke.Comp[Base]
}

func (x *indexer) init(si *goke.SysInit) {
	x.query, x.still = heldQueries(si, &x.base, &x.stillBase)
}

// what the space holds: every entity but those asleep at Omitted, those standing for good in its
// still layer
func heldQueries(si *goke.SysInit, base, stillBase *goke.Comp[Base]) (moving, still *goke.Query) {
	moving = si.NewQueryBuilder(base).
		Exclude(goke.Exclude[sector.L6](), goke.Exclude[Fixed]()).
		Build()
	still = si.NewQueryBuilder(stillBase).
		Include(goke.Include[Fixed]()).
		Exclude(goke.Exclude[sector.L6]()).
		Build()
	return moving, still
}

func (x *indexer) settle() {
	if x.w.moving.moved(x.query) {
		x.w.index(x.query, &x.base)
	}
	if x.w.still.moved(x.still) {
		x.w.indexStill(x.still, &x.stillBase)
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
	w.space.RebuildRuns(w.moving.gather(q, base), nil)
}

func (w *module) indexStill(q *goke.Query, base *goke.Comp[Base]) {
	w.space.RebuildStill(w.still.gather(q, base), nil)
}

// a layer of the space as it was last gathered, a run a chunk
type spaceLayer struct {
	runs     []aabbworld.Run
	versions []uint32 // each run's table version as it was gathered
}

func (l *spaceLayer) gather(q *goke.Query, base *goke.Comp[Base]) []aabbworld.Run {
	l.runs, l.versions = l.runs[:0], l.versions[:0]
	for q.All(); q.Next(); {
		cursor := q.Cursor()
		l.runs = append(l.runs, RunOf(cursor.IDs, base.Slice(cursor)))
		l.versions = append(l.versions, q.ChunkSnapshot().TableVer)
	}
	return l.runs
}

// moved reports whether a chunk q walks has come, gone, grown or shrunk since the runs were
// gathered, or goke has moved entities of its table between slots
func (l *spaceLayer) moved(q *goke.Query) bool {
	k := 0
	for q.All(); q.Next(); k++ {
		cursor := q.Cursor()
		if k == len(l.runs) {
			return true
		}
		ids := l.runs[k].IDs
		if len(ids) != len(cursor.IDs) || unsafe.SliceData(ids) != unsafe.SliceData(cursor.IDs) ||
			l.versions[k] != q.ChunkSnapshot().TableVer {
			return true
		}
	}
	return k != len(l.runs)
}
