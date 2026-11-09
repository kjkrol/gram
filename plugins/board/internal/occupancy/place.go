package occupancy

import (
	"slices"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/uid"
)

// PlaceSystem moves a part or an aggregate holding no cell yet, laid where its domain may not
// stand on terrain or where another holds the cell or was placed there this step, onto the
// nearest cell that takes it, placeRings rings round at most: a part a rule laid off its
// aggregate (world.SpawnPart), an aggregate folded where its parts stood. Run before the
// standing, so none is told it fell where it was only laid.
func PlaceSystem(o cell.Occupancy, footing Footing, g grid.Grid, pace goke.Opt,
	terrain cell.Terrain, space func() *aabbworld.Space) goke.System {
	s := &placeSystem{occupancy: o, footing: footing, grid: g, span: g.CellSpan(), pace: pace,
		terrain: terrain, space: space}
	s.holding, s.gone = s.isHolding, s.leaving.gone
	return s
}

// the most rings round a cell a part laid on it is moved within
const placeRings = 3

var _ goke.System = (*placeSystem)(nil)

type placeSystem struct {
	occupancy cell.Occupancy
	footing   Footing
	grid      grid.Grid
	span      float32
	pace      goke.Opt
	terrain   cell.Terrain
	space     func() *aabbworld.Space

	units  *goke.Query
	base   goke.Comp[world.Base]
	at     goke.Comp[unit.At]
	mover  goke.OptComp[unit.Mover]
	z      goke.OptComp[world.Z]
	partOf goke.OptComp[world.PartOf]
	whole  goke.OptComp[world.Aggregate]

	holding func(uid.UID64, cell.Domain, bool) // isHolding, bound once
	asked   uid.UID64
	held    bool
	// whoever left the world this step still holds its cell till the keep lets it go: on a cell
	// taken, the holds of those gone are let go first, once a step
	leaving  leaving
	gone     func(uid.UID64) bool
	released bool
	// the cells placed onto this step, and a search's rings and the cells it has seen
	placed           []cell.ID
	ring, next, seen []cell.ID
}

func (s *placeSystem) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.base, &s.at).Optional(&s.mover, &s.z, &s.partOf, &s.whole).
		Exclude(goke.Exclude[sector.Abstract](), goke.Exclude[world.Unfolded]())
	if s.pace != nil {
		qb.With(s.pace)
	}
	s.units = qb.Build()
	s.leaving.init(si)
}

func (s *placeSystem) Update(_ *goke.CmdBuf, d time.Duration) {
	if s.occupancy == nil {
		return
	}
	s.placed, s.released = s.placed[:0], false
	for s.units.Paced(d); s.units.Next(); {
		cur := s.units.Cursor()
		if !s.partOf.Present(cur) && !s.whole.Present(cur) {
			continue
		}
		bases, ats, movers, zs := s.base.Slice(cur), s.at.Slice(cur), s.mover.Slice(cur),
			s.z.Slice(cur)
		for i, id := range cur.IDs {
			feet, reach := f32.Inf(-1), float32(0)
			if zs != nil {
				feet, reach = zs[i].Altitude, zs[i].Reach()
			}
			p := bases[i].Pos.Center()
			if size := bases[i].Pos.Size; max(size.X, size.Y) > s.span {
				p = unit.LaidFrom(s.grid, size, p)
			}
			c, deck, _, ok := s.footing(p, feet, reach)
			if !ok || s.holds(c, id) || s.holds(ats[i].Cell, id) {
				continue
			}
			domain := unit.DomainAt(movers, i)
			if !s.released && !s.takes(c, deck, id, domain) {
				s.occupancy.Release(s.gone)
				s.released = true
			}
			if s.takes(c, deck, id, domain) {
				s.placed = append(s.placed, c)
				continue
			}
			if to, ok := s.nearest(c, p, id, domain); ok {
				s.placed = append(s.placed, to)
				if sp := s.space(); sp != nil {
					at := s.grid.CellCenter(to)
					sp.Move(&bases[i].Pos.AABB, geom.NewVec(at.X-p.X, at.Y-p.Y))
				}
			}
		}
	}
}

// whether c takes id moving in domain, on its deck where deck: its domain may stand there, nobody
// else holds it, nobody was placed there this step
func (s *placeSystem) takes(c cell.ID, deck bool, id uid.UID64, domain cell.Domain) bool {
	return s.terrain.Kind(c).Admits(domain) && s.occupancy.CanEnter(c, deck, id, domain) &&
		!slices.Contains(s.placed, c)
}

// the cell nearest p, ring by ring round from, that takes id moving in domain on its ground;
// false with none within placeRings
func (s *placeSystem) nearest(from cell.ID, p geom.Vec, id uid.UID64,
	domain cell.Domain) (cell.ID, bool) {
	s.ring, s.seen = append(s.ring[:0], from), append(s.seen[:0], from)
	for range placeRings {
		s.next = s.next[:0]
		for _, c := range s.ring {
			for k := range s.grid.Directions() {
				if n, ok := s.grid.Toward(c, k); ok && !slices.Contains(s.seen, n) {
					s.seen, s.next = append(s.seen, n), append(s.next, n)
				}
			}
		}
		best, found, near := cell.ID(0), false, f32.Inf(1)
		for _, n := range s.next {
			if !s.takes(n, false, id, domain) {
				continue
			}
			if at := s.grid.CellCenter(n); f32.Hypot(at.X-p.X, at.Y-p.Y) < near {
				best, found, near = n, true, f32.Hypot(at.X-p.X, at.Y-p.Y)
			}
		}
		if found {
			return best, true
		}
		s.ring, s.next = s.next, s.ring
	}
	return 0, false
}

func (s *placeSystem) holds(c cell.ID, id uid.UID64) bool {
	s.asked, s.held = id, false
	s.occupancy.Holders(c, s.holding)
	return s.held
}

func (s *placeSystem) isHolding(e uid.UID64, _ cell.Domain, _ bool) {
	if e == s.asked {
		s.held = true
	}
}
