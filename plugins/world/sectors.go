package world

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/uid"
)

// the world cut into sectors, each ticked at the fullest level an observer sees it at or a Keen
// entity keeps it at, the marks moved with them
type sectors struct {
	w         *module
	grid      *sector.Grid
	ladder    lod.Ladder
	observers []sector.Observers
	// the observers' cameras as of this tick, gathered by gather
	cams   []camera.Camera
	gather func(cam camera.Camera)
	// by observer, row j kept for the camera owners[j]: the level it sees every sector at, held
	// (lod.Ladder.Hold), and those levels graded among the sectors drawn
	owners []camera.Camera
	raw    [][]lod.Level
	graded [][]lod.Level
	// the sectors graded finer than their raw levels at the last pass, over every observer
	lifts int
	// the observer seenBy last found, looked at first: the views refresh one after another
	last int
	// by sector: the fullest level a Keen entity in it or round it keeps it at, L4 for none
	keen     []lod.Level
	keens    *goke.Query
	keenBase goke.Comp[Base]
	keenOf   goke.Comp[sector.Keen]
	// by sector: the level it is ticked at, every one L4 as a game begins
	level []lod.Level
	// the levels switched off: every sector ticked at L0
	unbanded bool
	// by level: L0's none
	marks [lod.Levels]goke.CompID
	// by level but L4, each its own column
	queries [lod.L4]*goke.Query
	bases   [lod.L4]goke.Comp[Base]
	lookup  *goke.Query
	base    goke.Comp[Base]
	off     goke.OptComp[sector.L4]
	// those at L4 by the sector they sleep in, the space not holding them
	sleep     [][]uid.UID64
	sleepers  *goke.Query
	sleepBase goke.Comp[Base]
	asleep    bool
	woke      []int
	ids       []uid.UID64
	// entities at each level at the last pass, L4's but those it holds since
	counts [lod.Levels]int
}

func newSectors(w *module) *sectors {
	s := &sectors{w: w}
	s.gather = func(cam camera.Camera) { s.cams = append(s.cams, cam) }
	cfg := w.config
	if cfg.Sectors.Buckets <= 0 {
		if cfg.LOD != (lod.Ladder{}) {
			panic("world: a ladder needs sectors, Config.Sectors.Buckets")
		}
		return s
	}
	cfg.LOD.Check()
	s.ladder = cfg.LOD
	side := float32(cfg.Sectors.Buckets) * float32(w.space.BucketSize)
	space := cfg.Space
	s.grid = sector.NewGrid(space.Width, space.Height, space.Edges.WrapsX(), space.Edges.WrapsY(),
		side)
	s.level = make([]lod.Level, s.grid.Len())
	s.keen = make([]lod.Level, s.grid.Len())
	s.sleep = make([][]uid.UID64, s.grid.Len())
	s.restart()
	return s
}

func (s *sectors) Init(si *goke.SysInit) {
	if s.grid == nil {
		return
	}
	s.marks = [lod.Levels]goke.CompID{0, si.RegComp[sector.L1](), si.RegComp[sector.L2](),
		si.RegComp[sector.L3](), si.RegComp[sector.L4]()}
	for k := range lod.L4 {
		qb := si.NewQueryBuilder(&s.bases[k])
		if k == lod.L0 {
			qb.Exclude(goke.Exclude[sector.L1](), goke.Exclude[sector.L2](),
				goke.Exclude[sector.L3](), goke.Exclude[sector.L4]())
		} else {
			qb.Include(markOf(k))
		}
		s.queries[k] = qb.Build()
	}
	s.lookup = si.NewQueryBuilder(&s.base).Optional(&s.off).Build()
	s.sleepers = si.NewQueryBuilder(&s.sleepBase).Include(goke.Include[sector.L4]()).Build()
	s.keens = si.NewQueryBuilder(&s.keenBase, &s.keenOf).Build()
}

func markOf(level lod.Level) goke.Opt {
	switch level {
	case lod.L1:
		return goke.Include[sector.L1]()
	case lod.L2:
		return goke.Include[sector.L2]()
	}
	return goke.Include[sector.L3]()
}

// Update lays each sector at the fullest level an observer sees it at or a Keen entity keeps it
// at and moves the marks: of the entities asleep in the sectors that came back from L4, then of
// every entity awake whose sector's level is not the one it carries.
func (s *sectors) Update(cb *goke.CmdBuf, _ time.Duration) {
	if s.grid == nil {
		return
	}
	if !s.asleep {
		s.lull()
	}
	s.observe()
	if !s.unbanded && len(s.cams) > 0 {
		s.keep()
	}
	s.woke = s.woke[:0]
	for i, from := range s.level {
		to := lod.L0
		if !s.unbanded {
			to = s.ticked(i)
		}
		if to != from {
			s.level[i] = to
			if from == lod.L4 {
				s.woke = append(s.woke, i)
			}
		}
	}
	s.ids = s.ids[:0]
	for _, i := range s.woke {
		s.wake(i)
	}
	slices.Sort(s.ids)
	for _, id := range slices.Compact(s.ids) {
		s.lookup.Seek(id)
		c := s.base.At(s.lookup.Cursor()).Pos.Center()
		at := s.grid.Of(c.X, c.Y)
		s.move(cb, id, lod.L4, s.level[at], at)
	}
	clear(s.counts[:])
	for k, q := range s.queries {
		from := lod.Level(k)
		for q.All(); q.Next(); {
			cur := q.Cursor()
			bases := s.bases[k].Slice(cur)
			for j, id := range cur.IDs {
				c := bases[j].Pos.Center()
				at := s.grid.Of(c.X, c.Y)
				to := s.level[at]
				if to != from {
					s.move(cb, id, from, to, at)
				}
				s.counts[to]++
			}
		}
	}
}

// the level each observer sees every sector at, held and graded
func (s *sectors) observe() {
	s.cams = s.cams[:0]
	for _, o := range s.observers {
		o.Observers(s.gather)
	}
	for len(s.raw) < len(s.cams) {
		s.owners = append(s.owners, nil)
		s.raw = append(s.raw, make([]lod.Level, s.grid.Len()))
		s.graded = append(s.graded, make([]lod.Level, s.grid.Len()))
	}
	s.lifts = 0
	for j, cam := range s.cams {
		s.take(j, cam)
		s.hold(cam, s.raw[j])
		s.lifts += s.grade(s.raw[j], s.graded[j])
	}
}

// has row j kept for cam: the row it had, else one at L4 everywhere, which the first pass lays
// at once
func (s *sectors) take(j int, cam camera.Camera) {
	if s.owners[j] == cam {
		return
	}
	if k := slices.Index(s.owners[j+1:], cam); k >= 0 {
		k += j + 1
		s.owners[j], s.owners[k] = s.owners[k], s.owners[j]
		s.raw[j], s.raw[k] = s.raw[k], s.raw[j]
		s.graded[j], s.graded[k] = s.graded[k], s.graded[j]
		return
	}
	s.owners[j] = cam
	for i := range s.raw[j] {
		s.raw[j][i] = lod.L4
	}
}

// the level cam sees every sector at, from the scale shown at the sector's point nearest its
// viewpoint and how far that lies, a coarser one than row had only half a sector past it
func (s *sectors) hold(cam camera.Camera, row []lod.Level) {
	ladder := s.ladder.Widened(camera.SightOf(cam))
	margin := s.grid.Side() / 2
	x, y, ok := camera.Viewpoint(cam)
	for i, from := range row {
		nx, ny, dist := s.grid.Nearest(i, x, y)
		if !ok {
			box := s.grid.Box(i)
			nx, ny = (box.TopLeft.X+box.BottomRight.X)/2, (box.TopLeft.Y+box.BottomRight.Y)/2
			dist = 0
		}
		row[i] = ladder.Hold(from, camera.ScaleShown(cam, nx, ny, 0), dist, margin)
	}
}

// has graded raw with no sector drawn more than a level coarser than one drawn beside it, each
// level lifting the sectors round it in turn from the finest; one at L4 stays so and lifts none.
// It is how many were lifted.
func (s *sectors) grade(raw, graded []lod.Level) (lifts int) {
	copy(graded, raw)
	for k := lod.L0; k+1 < lod.L4; k++ {
		for i, l := range graded {
			if l != k {
				continue
			}
			s.grid.Neighbours(i, func(j int) {
				if l := graded[j]; l > k+1 && l < lod.L4 {
					graded[j] = k + 1
				}
			})
		}
	}
	for i, l := range graded {
		if l < raw[i] {
			lifts++
		}
	}
	return lifts
}

// the fullest level each sector is kept at by the Keen entities in it and round it, those asleep
// too
func (s *sectors) keep() {
	for i := range s.keen {
		s.keen[i] = lod.L4
	}
	for s.keens.All(); s.keens.Next(); {
		cur := s.keens.Cursor()
		bases, keens := s.keenBase.Slice(cur), s.keenOf.Slice(cur)
		for j := range bases {
			c := bases[j].Pos.Center()
			at, level := s.grid.Of(c.X, c.Y), keens[j].Level
			s.keen[at] = min(s.keen[at], level)
			s.grid.Neighbours(at, func(i int) { s.keen[i] = min(s.keen[i], level) })
		}
	}
}

// the level sector i is ticked at: the fullest an observer draws it at or a Keen entity keeps it
// at, L0 with no observer
func (s *sectors) ticked(i int) lod.Level {
	if len(s.cams) == 0 {
		return lod.L0
	}
	level := s.keen[i]
	for j := range s.cams {
		level = min(level, s.graded[j][i])
	}
	return level
}

// the level cam draws sector i at as an observer; the level ticked for a camera that is none
func (s *sectors) seenBy(cam camera.Camera, i int) lod.Level {
	if s.last >= len(s.cams) || s.cams[s.last] != cam {
		s.last = slices.Index(s.cams, cam)
		if s.last < 0 {
			s.last = 0
			return s.level[i]
		}
	}
	return s.graded[s.last][i]
}

// lays every entity marked L4 in the sector it sleeps in, as a game begins or after a load
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

// gathers the entities asleep in sector i still marked L4; one moved off sleeps on where it is
// if that sector is at L4 too
func (s *sectors) wake(i int) {
	for _, id := range s.sleep[i] {
		if !s.lookup.Seek(id) || !s.off.Present(s.lookup.Cursor()) {
			continue
		}
		c := s.base.At(s.lookup.Cursor()).Pos.Center()
		if at := s.grid.Of(c.X, c.Y); at != i && s.level[at] == lod.L4 {
			s.sleep[at] = append(s.sleep[at], id)
			continue
		}
		s.ids = append(s.ids, id)
	}
	s.sleep[i] = s.sleep[i][:0]
}

// every sector at L4 again, its sleepers laid anew and every observer's levels begun afresh: as a
// game begins and after a load
func (s *sectors) restart() {
	if s.grid != nil {
		for i := range s.level {
			s.level[i] = lod.L4
		}
		clear(s.owners)
		s.asleep = false
	}
}

// to the level of sector at, sleeping in it at L4
func (s *sectors) move(cb *goke.CmdBuf, id uid.UID64, from, to lod.Level, at int) {
	if from != lod.L0 {
		cb.RemoveCompOne(id, s.marks[from])
	}
	switch to {
	case lod.L0:
	case lod.L1:
		cb.AddOne(id, s.marks[to], sector.L1{})
	case lod.L2:
		cb.AddOne(id, s.marks[to], sector.L2{})
	case lod.L3:
		cb.AddOne(id, s.marks[to], sector.L3{})
	default:
		s.sleep[at] = append(s.sleep[at], id)
		cb.AddOne(id, s.marks[to], sector.L4{})
	}
}

// the sectors as a camera's view sees them
type seenSectors struct{ s *sectors }

var _ view.Sectors = seenSectors{}

func (s seenSectors) Len() int            { return s.s.grid.Len() }
func (s seenSectors) Cols() int           { return s.s.grid.Cols() }
func (s seenSectors) Box(i int) geom.AABB { return s.s.grid.Box(i) }
func (s seenSectors) Of(x, y float32) int { return s.s.grid.Of(x, y) }
func (s seenSectors) Ladder() lod.Ladder  { return s.s.ladder }

func (s seenSectors) Level(cam camera.Camera, i int) lod.Level { return s.s.seenBy(cam, i) }

type seenReport struct{ w *module }

func (r seenReport) Report(line func(label, value string)) {
	if r.w.sectors.grid == nil || len(r.w.views) == 0 {
		return
	}
	var n [lod.Levels]int
	r.w.views[0].Sectors(func(_ int, _ geom.AABB, l lod.Level) { n[l]++ })
	line("Sectors seen", levelsOf(n[:lod.L4]))
}

type sectorsReport struct{ w *module }

func (r sectorsReport) Report(line func(label, value string)) {
	s := r.w.sectors
	if s.grid == nil {
		return
	}
	var in [lod.Levels]int
	for _, l := range s.level {
		in[l]++
	}
	if s.unbanded {
		line("Sectors", levelsOf(in[:])+"\nlevels off")
	} else {
		line("Sectors", levelsOf(in[:]))
	}
	line("Sectors lifted", strconv.Itoa(s.lifts))
	counts := s.counts
	awake := 0
	for _, n := range counts[:lod.L4] {
		awake += n
	}
	counts[lod.L4] = max(r.w.telemetry.Count-awake, 0)
	line("Entities by level", levelsOf(counts[:]))
}

// the levels' numbers a line each, L0 first
func levelsOf(n []int) string {
	var b strings.Builder
	for k, v := range n {
		if k > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "L%d %d", k, v)
	}
	return b.String()
}
