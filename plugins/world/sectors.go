package world

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/uid"
)

const defaultSectorBuckets = 4

// the world cut into sectors and their bands round the anchors, the marks moved with them
type sectors struct {
	w       *module
	grid    *sector.Grid
	anchors []sector.Anchors
	at      []geom.Vec
	margin  float32
	// the bands switched off: the anchors left out, every sector in the first
	unbanded bool
	// by band: the first's none, Beyond's Off
	marks []goke.CompID
	// by band but Beyond's, each its own column
	queries []*goke.Query
	bases   []goke.Comp[Base]
	lookup  *goke.Query
	base    goke.Comp[Base]
	off     goke.OptComp[sector.Off]
	// those beyond the last band by the sector they sleep in, the space not holding them
	sleep     [][]uid.UID64
	sleepers  *goke.Query
	sleepBase goke.Comp[Base]
	asleep    bool
	woke      []int
	ids       []uid.UID64
	// entities in each band at the last pass, Beyond's but those it holds since
	counts []int
}

func newSectors(w *module) *sectors {
	s := &sectors{w: w}
	cfg := w.config.Sectors
	if len(cfg.Bands) == 0 {
		return s
	}
	checkBands(cfg.Bands)
	buckets := cfg.Buckets
	if buckets == 0 {
		buckets = defaultSectorBuckets
	}
	side := float32(buckets) * float32(w.space.BucketSize)
	space := w.config.Space
	s.grid = sector.NewGrid(space.Width, space.Height, space.Edges.WrapsX(), space.Edges.WrapsY(),
		side, cfg.Bands)
	s.margin = side / 2
	s.sleep = make([][]uid.UID64, s.grid.Len())
	return s
}

func checkBands(bands []sector.Band) {
	if len(bands) > sector.MaxBands {
		panic(fmt.Sprintf("world: %d bands of sectors, at most %d", len(bands), sector.MaxBands))
	}
	for k, b := range bands {
		switch {
		case b.Reach <= 0 || k > 0 && b.Reach <= bands[k-1].Reach:
			panic(fmt.Sprintf(
				"world: band %d of sectors reaches %v, want farther than the one before",
				k+1, b.Reach))
		case k == 0 && b.Pace > 1, k > 0 && b.Pace < 1:
			panic(fmt.Sprintf("world: band %d of sectors goes a step every %d, want the first "+
				"every one and the others at least every one", k+1, b.Pace))
		}
	}
}

func (s *sectors) Init(si *goke.SysInit) {
	if s.grid == nil {
		return
	}
	n := s.grid.Beyond()
	s.marks = make([]goke.CompID, n+1)
	regs := [sector.MaxBands]goke.CompID{0, si.RegComp[sector.Band2](), si.RegComp[sector.Band3](),
		si.RegComp[sector.Band4](), si.RegComp[sector.Band5]()}
	copy(s.marks[1:n], regs[1:n])
	s.marks[n] = si.RegComp[sector.Off]()
	s.bases = make([]goke.Comp[Base], n)
	s.queries = make([]*goke.Query, n)
	for k := range n {
		qb := si.NewQueryBuilder(&s.bases[k])
		if k == 0 {
			qb.Exclude(goke.Exclude[sector.Band2](), goke.Exclude[sector.Band3](),
				goke.Exclude[sector.Band4](), goke.Exclude[sector.Band5](),
				goke.Exclude[sector.Off]())
		} else {
			qb.Include(markOf(k))
		}
		s.queries[k] = qb.Build()
	}
	s.lookup = si.NewQueryBuilder(&s.base).Optional(&s.off).Build()
	s.sleepers = si.NewQueryBuilder(&s.sleepBase).Include(goke.Include[sector.Off]()).Build()
	s.counts = make([]int, n+1)
}

func markOf(band int) goke.Opt {
	switch band {
	case 1:
		return goke.Include[sector.Band2]()
	case 2:
		return goke.Include[sector.Band3]()
	case 3:
		return goke.Include[sector.Band4]()
	}
	return goke.Include[sector.Band5]()
}

// Update lays the sectors in their bands round the anchors and moves the marks: of the entities
// asleep in the sectors that came back from beyond the last band, then of every entity this side
// of it whose sector's band is not the one it carries.
func (s *sectors) Update(cb *goke.CmdBuf, _ time.Duration) {
	if s.grid == nil {
		return
	}
	if !s.asleep {
		s.lull()
	}
	s.at = s.at[:0]
	for _, a := range s.anchors {
		if s.unbanded {
			break
		}
		a.Anchors(func(x, y float32) { s.at = append(s.at, geom.NewVec(x, y)) })
	}
	s.woke = s.woke[:0]
	beyond := s.grid.Beyond()
	s.grid.Place(s.at, s.margin, func(i, from, _ int) {
		if from == beyond {
			s.woke = append(s.woke, i)
		}
	})
	s.ids = s.ids[:0]
	for _, i := range s.woke {
		s.wake(i)
	}
	slices.Sort(s.ids)
	for _, id := range slices.Compact(s.ids) {
		s.lookup.Seek(id)
		c := s.base.At(s.lookup.Cursor()).Pos.Center()
		at := s.grid.Of(c.X, c.Y)
		s.move(cb, id, beyond, s.grid.Band(at), at)
	}
	clear(s.counts)
	for k, q := range s.queries {
		for q.All(); q.Next(); {
			cur := q.Cursor()
			bases := s.bases[k].Slice(cur)
			for j, id := range cur.IDs {
				c := bases[j].Pos.Center()
				at := s.grid.Of(c.X, c.Y)
				to := s.grid.Band(at)
				if to != k {
					s.move(cb, id, k, to, at)
				}
				s.counts[to]++
			}
		}
	}
}

// lays every entity marked Off in the sector it sleeps in, as a game begins or after a load
func (s *sectors) lull() {
	for i := range s.sleep {
		s.sleep[i] = s.sleep[i][:0]
	}
	for s.sleepers.All(); s.sleepers.Next(); {
		cur := s.sleepers.Cursor()
		for j, b := range s.sleepBase.Slice(cur) {
			c := b.Pos.Center()
			i := s.grid.Of(c.X, c.Y)
			s.sleep[i] = append(s.sleep[i], cur.IDs[j])
		}
	}
	s.asleep = true
}

// gathers the entities asleep in sector i still marked Off; one moved off sleeps on where it is
// if that sector lies beyond too
func (s *sectors) wake(i int) {
	for _, id := range s.sleep[i] {
		if !s.lookup.Seek(id) || !s.off.Present(s.lookup.Cursor()) {
			continue
		}
		c := s.base.At(s.lookup.Cursor()).Pos.Center()
		if at := s.grid.Of(c.X, c.Y); at != i && s.grid.Band(at) == s.grid.Beyond() {
			s.sleep[at] = append(s.sleep[at], id)
			continue
		}
		s.ids = append(s.ids, id)
	}
	s.sleep[i] = s.sleep[i][:0]
}

// after a load every sector begins beyond again, its sleepers laid anew
func (s *sectors) restart() {
	if s.grid != nil {
		s.grid.Reset()
		s.asleep = false
	}
}

// to the band of sector at, sleeping in it if that lies beyond the last band
func (s *sectors) move(cb *goke.CmdBuf, id uid.UID64, from, to, at int) {
	if from != 0 {
		cb.RemoveCompOne(id, s.marks[from])
	}
	switch {
	case to == 0:
	case to == s.grid.Beyond():
		s.sleep[at] = append(s.sleep[at], id)
		cb.AddOne(id, s.marks[to], sector.Off{})
	case to == 1:
		cb.AddOne(id, s.marks[to], sector.Band2{})
	case to == 2:
		cb.AddOne(id, s.marks[to], sector.Band3{})
	case to == 3:
		cb.AddOne(id, s.marks[to], sector.Band4{})
	default:
		cb.AddOne(id, s.marks[to], sector.Band5{})
	}
}

// the sectors as a camera's view sees them
type seenSectors struct{ s *sectors }

var _ view.Sectors = seenSectors{}

func (s seenSectors) Len() int            { return s.s.grid.Len() }
func (s seenSectors) Cols() int           { return s.s.grid.Cols() }
func (s seenSectors) Box(i int) geom.AABB { return s.s.grid.Box(i) }
func (s seenSectors) Of(x, y float32) int { return s.s.grid.Of(x, y) }
func (s seenSectors) Off(i int) bool      { return s.s.grid.Band(i) == s.s.grid.Beyond() }

type seenReport struct{ w *module }

func (r seenReport) Report(line func(label, value string)) {
	if r.w.sectors.grid == nil || len(r.w.views) == 0 {
		return
	}
	var n [4]int
	r.w.views[0].Sectors(func(_ int, _ geom.AABB, l view.Level) { n[l]++ })
	line("Sectors seen", fmt.Sprintf("exact %d\ncoarse %d\nfog %d", n[view.Exact],
		n[view.Coarse], n[view.Fogged]))
}

type sectorsReport struct{ w *module }

func (r sectorsReport) Report(line func(label, value string)) {
	s := r.w.sectors
	if s.grid == nil {
		return
	}
	in := s.grid.Count(nil)
	if s.unbanded {
		line("Sectors", bandsOf(in)+"\nbands off")
	} else {
		line("Sectors", bandsOf(in))
	}
	counts := append([]int(nil), s.counts...)
	near := 0
	for _, n := range counts[:len(counts)-1] {
		near += n
	}
	counts[len(counts)-1] = max(r.w.telemetry.Count-near, 0)
	line("Entities by band", bandsOf(counts))
}

// the bands' numbers a line each, nearest first, beyond the last
func bandsOf(n []int) string {
	var b strings.Builder
	for k, v := range n[:len(n)-1] {
		fmt.Fprintf(&b, "%s %d\n", bandNames[k], v)
	}
	fmt.Fprintf(&b, "beyond %d", n[len(n)-1])
	return b.String()
}

var bandNames = [sector.MaxBands]string{"first", "second", "third", "fourth", "fifth"}
