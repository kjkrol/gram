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
	// by sector: the fullest level a Keen entity in it or round it keeps it at, Omitted for none
	keen     []lod.Level
	keens    *goke.Query
	keenBase goke.Comp[Base]
	keenOf   goke.Comp[sector.Keen]
	// by sector: the level it is ticked at, every one Omitted as a game begins
	level []lod.Level
	// the levels switched off: every sector ticked at Standard
	unbanded bool
	// the level every sector is ticked at with no observer, the ladder's Unobserved
	unobserved lod.Level
	// the finest level any observer sees a sector at, a Keen entity's none, and by the level
	// ticked how many levels coarser than it that is, Omitted's none
	finest lod.Level
	far    [lod.Levels]uint8
	// by level
	marks [lod.Levels]goke.CompID
	// by how many levels coarser than the finest seen: none's none
	fars [lod.Omitted]goke.CompID
	// by level but Omitted, and those spawned since the last pass, carrying no level's mark yet
	walks  [lod.Omitted]levelWalk
	fresh  levelWalk
	lookup *goke.Query
	base   goke.Comp[Base]
	off    goke.OptComp[sector.L6]
	// those at Omitted by the sector they sleep in, the space not holding them
	sleep     [][]uid.UID64
	sleepers  *goke.Query
	sleepBase goke.Comp[Base]
	asleep    bool
	woke      []int
	ids       []uid.UID64
	// entities at each level at the last pass, Omitted's but those it holds since
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
	s.unobserved = cfg.LOD.Unobserved.Level()
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
	s.marks = [lod.Levels]goke.CompID{si.RegComp[sector.L0](), si.RegComp[sector.L1](),
		si.RegComp[sector.L2](), si.RegComp[sector.L3](), si.RegComp[sector.L4](),
		si.RegComp[sector.L5](), si.RegComp[sector.L6]()}
	s.fars = [lod.Omitted]goke.CompID{0, si.RegComp[sector.Far1](), si.RegComp[sector.Far2](),
		si.RegComp[sector.Far3](), si.RegComp[sector.Far4](), si.RegComp[sector.Far5]()}
	si.RegComp[sector.Abstract]()
	for k := range lod.Omitted {
		s.walks[k].build(si, markOf(k))
	}
	s.fresh.build(si, excludeMarks()...)
	s.lookup = si.NewQueryBuilder(&s.base).Optional(&s.off).Build()
	s.sleepers = si.NewQueryBuilder(&s.sleepBase).Include(goke.Include[sector.L6]()).Build()
	s.keens = si.NewQueryBuilder(&s.keenBase, &s.keenOf).Build()
}

// the level a fresh entity is taken to be marked at: none of them
const unmarked = lod.Level(lod.Levels)

// the filter of the entities carrying level's mark, of those below Omitted
func markOf(level lod.Level) goke.Opt {
	switch level {
	case lod.L0:
		return goke.Include[sector.L0]()
	case lod.L1:
		return goke.Include[sector.L1]()
	case lod.L2:
		return goke.Include[sector.L2]()
	case lod.L3:
		return goke.Include[sector.L3]()
	case lod.L4:
		return goke.Include[sector.L4]()
	}
	return goke.Include[sector.L5]()
}

// the filters leaving out every entity a level's mark is on but those of the levels in keep
func excludeMarks(keep ...lod.Level) []goke.Opt {
	out := make([]goke.Opt, 0, lod.Levels)
	for k := range lod.Level(lod.Levels) {
		if slices.Contains(keep, k) {
			continue
		}
		switch k {
		case lod.L0:
			out = append(out, goke.Exclude[sector.L0]())
		case lod.L1:
			out = append(out, goke.Exclude[sector.L1]())
		case lod.L2:
			out = append(out, goke.Exclude[sector.L2]())
		case lod.L3:
			out = append(out, goke.Exclude[sector.L3]())
		case lod.L4:
			out = append(out, goke.Exclude[sector.L4]())
		case lod.L5:
			out = append(out, goke.Exclude[sector.L5]())
		default:
			out = append(out, goke.Exclude[sector.L6]())
		}
	}
	return out
}

// the entities a level's filters let through, with the pace's mark of each chunk
type levelWalk struct {
	q    *goke.Query
	base goke.Comp[Base]
	far1 goke.OptComp[sector.Far1]
	far2 goke.OptComp[sector.Far2]
	far3 goke.OptComp[sector.Far3]
	far4 goke.OptComp[sector.Far4]
	far5 goke.OptComp[sector.Far5]
}

func (w *levelWalk) build(si *goke.SysInit, filters ...goke.Opt) {
	w.q = si.NewQueryBuilder(&w.base).Optional(&w.far1, &w.far2, &w.far3, &w.far4, &w.far5).
		With(filters...).Build()
}

// how many levels coarser than the finest seen the chunk under cur is marked
func (w *levelWalk) far(cur *goke.Cursor) uint8 {
	switch {
	case w.far1.Present(cur):
		return 1
	case w.far2.Present(cur):
		return 2
	case w.far3.Present(cur):
		return 3
	case w.far4.Present(cur):
		return 4
	case w.far5.Present(cur):
		return 5
	}
	return 0
}

// what an entity carries of its sector: the level ticked there and how many levels coarser than
// the finest seen that is
type mark struct {
	level lod.Level
	far   uint8
}

// Update lays each sector at the fullest level an observer sees it at or a Keen entity keeps it
// at and moves the marks: of the entities asleep in the sectors that came back from Omitted, then
// of every entity awake whose sector's marks are not the ones it carries.
func (s *sectors) Update(cb *goke.CmdBuf, _ time.Duration) {
	if s.grid == nil {
		return
	}
	if !s.asleep {
		s.lull()
	}
	s.observe()
	if !s.unbanded {
		s.keep()
	}
	s.woke = s.woke[:0]
	for i, from := range s.level {
		to := lod.Standard
		if !s.unbanded {
			to = s.ticked(i)
		}
		if to != from {
			s.level[i] = to
			if from == lod.Omitted {
				s.woke = append(s.woke, i)
			}
		}
	}
	for k := range lod.Omitted {
		s.far[k] = uint8(max(k, s.finest) - s.finest)
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
		s.move(cb, id, mark{level: lod.Omitted}, s.markOf(at), at)
	}
	clear(s.counts[:])
	for k := range s.walks {
		s.remark(cb, &s.walks[k], lod.Level(k))
	}
	s.remark(cb, &s.fresh, unmarked)
}

// moves the marks of every entity w walks, marked at level, whose sector's are not the ones it
// carries
func (s *sectors) remark(cb *goke.CmdBuf, w *levelWalk, level lod.Level) {
	for w.q.All(); w.q.Next(); {
		cur := w.q.Cursor()
		from := mark{level, w.far(cur)}
		bases := w.base.Slice(cur)
		for j, id := range cur.IDs {
			c := bases[j].Pos.Center()
			at := s.grid.Of(c.X, c.Y)
			to := s.markOf(at)
			if to != from {
				s.move(cb, id, from, to, at)
			}
			s.counts[to.level]++
		}
	}
}

// the marks of the entities in sector i
func (s *sectors) markOf(i int) mark {
	level := s.level[i]
	return mark{level, s.far[level]}
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
	s.finest = s.unobserved
	if len(s.cams) > 0 {
		s.finest = lod.Omitted
	}
	for j, cam := range s.cams {
		s.take(j, cam)
		s.hold(cam, s.raw[j])
		lifts, finest := s.grade(s.raw[j], s.graded[j])
		s.lifts += lifts
		s.finest = min(s.finest, finest)
	}
}

// has row j kept for cam: the row it had, else one at Omitted everywhere, which the first pass lays
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
		s.raw[j][i] = lod.Omitted
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
// level lifting the sectors round it in turn from the finest; one at Omitted stays so and lifts
// none.
// It is how many were lifted and the finest level graded.
func (s *sectors) grade(raw, graded []lod.Level) (lifts int, finest lod.Level) {
	copy(graded, raw)
	for k := lod.L0; k+1 < lod.Omitted; k++ {
		for i, l := range graded {
			if l != k {
				continue
			}
			s.grid.Neighbours(i, func(j int) {
				if l := graded[j]; l > k+1 && l < lod.Omitted {
					graded[j] = k + 1
				}
			})
		}
	}
	finest = lod.Omitted
	for i, l := range graded {
		if l < raw[i] {
			lifts++
		}
		finest = min(finest, l)
	}
	return lifts, finest
}

// the fullest level each sector is kept at by the Keen entities in it and round it, those asleep
// too
func (s *sectors) keep() {
	for i := range s.keen {
		s.keen[i] = lod.Omitted
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

// the level sector i is ticked at: the fullest an observer draws it at, or the ladder's
// Unobserved with no observer, or a Keen entity keeps it at
func (s *sectors) ticked(i int) lod.Level {
	level := s.keen[i]
	if len(s.cams) == 0 {
		return min(level, s.unobserved)
	}
	for j := range s.cams {
		level = min(level, s.graded[j][i])
	}
	return level
}

// the level the sector holding (x, y) is ticked at; Standard without sectors or with the levels
// off
func (s *sectors) levelAt(x, y float32) lod.Level { return s.levelOf(s.of(x, y)) }

// the level sector i is ticked at; Standard without sectors or with the levels off
func (s *sectors) levelOf(i int) lod.Level {
	if s.grid == nil || s.unbanded {
		return lod.Standard
	}
	return s.level[i]
}

// the sector holding (x, y); a world of no sectors is one, 0
func (s *sectors) of(x, y float32) int {
	if s.grid == nil {
		return 0
	}
	return s.grid.Of(x, y)
}

// appends to dst the sectors box covers with the part of it in each; a world of no sectors is
// one, 0, holding the whole of it
func (s *sectors) cover(box geom.AABB, dst []sector.Part) []sector.Part {
	if s.grid == nil {
		return append(dst, sector.Part{Box: box})
	}
	return s.grid.Cover(box, dst)
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

// lays every entity marked Omitted in the sector it sleeps in, as a game begins or after a load
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

// gathers the entities asleep in sector i still marked Omitted; one moved off sleeps on where it
// is if that sector is Omitted too
func (s *sectors) wake(i int) {
	for _, id := range s.sleep[i] {
		if !s.lookup.Seek(id) || !s.off.Present(s.lookup.Cursor()) {
			continue
		}
		c := s.base.At(s.lookup.Cursor()).Pos.Center()
		if at := s.grid.Of(c.X, c.Y); at != i && s.level[at] == lod.Omitted {
			s.sleep[at] = append(s.sleep[at], id)
			continue
		}
		s.ids = append(s.ids, id)
	}
	s.sleep[i] = s.sleep[i][:0]
}

// every sector Omitted again, its sleepers laid anew and every observer's levels begun afresh: as a
// game begins and after a load
func (s *sectors) restart() {
	if s.grid != nil {
		for i := range s.level {
			s.level[i] = lod.Omitted
		}
		clear(s.owners)
		s.asleep = false
	}
}

// to the marks of sector at, sleeping in it at Omitted
func (s *sectors) move(cb *goke.CmdBuf, id uid.UID64, from, to mark, at int) {
	if from.far != to.far {
		if from.far != 0 {
			cb.RemoveCompOne(id, s.fars[from.far])
		}
		switch to.far {
		case 1:
			cb.AddOne(id, s.fars[to.far], sector.Far1{})
		case 2:
			cb.AddOne(id, s.fars[to.far], sector.Far2{})
		case 3:
			cb.AddOne(id, s.fars[to.far], sector.Far3{})
		case 4:
			cb.AddOne(id, s.fars[to.far], sector.Far4{})
		case 5:
			cb.AddOne(id, s.fars[to.far], sector.Far5{})
		}
	}
	if from.level == to.level {
		return
	}
	if from.level != unmarked {
		cb.RemoveCompOne(id, s.marks[from.level])
	}
	switch to.level {
	case lod.L0:
		cb.AddOne(id, s.marks[to.level], sector.L0{})
	case lod.L1:
		cb.AddOne(id, s.marks[to.level], sector.L1{})
	case lod.L2:
		cb.AddOne(id, s.marks[to.level], sector.L2{})
	case lod.L3:
		cb.AddOne(id, s.marks[to.level], sector.L3{})
	case lod.L4:
		cb.AddOne(id, s.marks[to.level], sector.L4{})
	case lod.L5:
		cb.AddOne(id, s.marks[to.level], sector.L5{})
	default:
		s.sleep[at] = append(s.sleep[at], id)
		cb.AddOne(id, s.marks[to.level], sector.L6{})
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
	line("Sectors seen", levelsOf(n[:lod.Omitted]))
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
	for _, n := range counts[:lod.Omitted] {
		awake += n
	}
	counts[lod.Omitted] = max(r.w.telemetry.Count-awake, 0)
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
