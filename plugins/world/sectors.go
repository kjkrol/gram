package world

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

// Anchor has the world live round the points a gives: the sectors' bands (Config.Sectors) are
// taken from the nearest of them.
func (p *Plugin) Anchor(a sector.Anchors) {
	p.module.sectors.anchors = append(p.module.sectors.anchors, a)
}

// Sectors is the world's part of the telemetry panel: how many sectors and entities lie in each
// band.
func (p *Plugin) Sectors() render.Reporter { return sectorsReport{p.module} }

const defaultSectorBuckets = 4

// the world cut into sectors and their bands round the anchors, the marks moved with them
type sectors struct {
	w       *module
	grid    *sector.Grid
	anchors []sector.Anchors
	at      []geom.Vec
	margin  float64
	// by band: the first's none, Beyond's Off
	marks []goke.CompID
	// by band but Beyond's, each its own column
	bands  []*goke.Query
	bases  []goke.Comp[Base]
	lookup *goke.Query
	base   goke.Comp[Base]
	off    goke.OptComp[sector.Off]
	woke   []int
	ids    []uid.UID64
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
	side := float64(buckets) * float64(w.space.BucketSize)
	space := w.config.Space
	s.grid = sector.NewGrid(space.Width, space.Height, space.Edges.WrapsX(), space.Edges.WrapsY(),
		side, cfg.Bands)
	s.margin = side / 2
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
	s.bands = make([]*goke.Query, n)
	for k := range n {
		qb := si.NewQueryBuilder(&s.bases[k])
		if k == 0 {
			qb.Exclude(goke.Exclude[sector.Band2](), goke.Exclude[sector.Band3](),
				goke.Exclude[sector.Band4](), goke.Exclude[sector.Band5](),
				goke.Exclude[sector.Off]())
		} else {
			qb.Include(markOf(k))
		}
		s.bands[k] = qb.Build()
	}
	s.lookup = si.NewQueryBuilder(&s.base).Optional(&s.off).Build()
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
// in the sectors that came back from beyond the last band, then of every entity this side of it
// whose sector's band is not the one it carries.
func (s *sectors) Update(cb *goke.CmdBuf, _ time.Duration) {
	if s.grid == nil {
		return
	}
	s.at = s.at[:0]
	for _, a := range s.anchors {
		a.Anchors(func(x, y float32) { s.at = append(s.at, geom.NewVec(float64(x), float64(y))) })
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
	// a box across a seam is found once a piece
	slices.Sort(s.ids)
	for _, id := range slices.Compact(s.ids) {
		s.lookup.Seek(id)
		c := s.base.At(s.lookup.Cursor()).Pos.Center()
		s.move(cb, id, beyond, s.grid.Band(s.grid.Of(c.X, c.Y)))
	}
	clear(s.counts)
	for k, q := range s.bands {
		for q.All(); q.Next(); {
			cur := q.Cursor()
			bases := s.bases[k].Slice(cur)
			for j, id := range cur.IDs {
				c := bases[j].Pos.Center()
				to := s.grid.Band(s.grid.Of(c.X, c.Y))
				if to != k {
					s.move(cb, id, k, to)
				}
				s.counts[to]++
			}
		}
	}
}

// gathers the entities marked Off whose middle lies in sector i
func (s *sectors) wake(i int) {
	box := s.grid.Box(i)
	// a box's edge touching the next sector's buckets would reach into them
	inner := geom.NewAABB(box.TopLeft, geom.NewVec(box.BottomRight.X-1e-6, box.BottomRight.Y-1e-6))
	s.w.space.Query(inner, aabbworld.AnyCapability, func(id uid.UID64) {
		if !s.lookup.Seek(id) || !s.off.Present(s.lookup.Cursor()) {
			return
		}
		if c := s.base.At(s.lookup.Cursor()).Pos.Center(); s.grid.Of(c.X, c.Y) == i {
			s.ids = append(s.ids, id)
		}
	})
}

func (s *sectors) move(cb *goke.CmdBuf, id uid.UID64, from, to int) {
	if from != 0 {
		cb.RemoveCompOne(id, s.marks[from])
	}
	switch {
	case to == 0:
	case to == s.grid.Beyond():
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

type sectorsReport struct{ w *module }

func (sectorsReport) Init(*goke.SysInit) {}

func (r sectorsReport) Report(line func(label, value string)) {
	s := r.w.sectors
	if s.grid == nil {
		return
	}
	in := s.grid.Count(nil)
	line("Sectors", bandsOf(in))
	counts := append([]int(nil), s.counts...)
	near := 0
	for _, n := range counts[:len(counts)-1] {
		near += n
	}
	counts[len(counts)-1] = max(r.w.telemetry.Count-near, 0)
	line("Entities by band", bandsOf(counts))
}

// the bands' numbers nearest first, beyond the last apart
func bandsOf(n []int) string {
	var b strings.Builder
	for k, v := range n[:len(n)-1] {
		if k > 0 {
			b.WriteString(" · ")
		}
		fmt.Fprint(&b, v)
	}
	fmt.Fprintf(&b, ", beyond %d", n[len(n)-1])
	return b.String()
}
