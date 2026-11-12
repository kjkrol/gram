package spacing

import (
	"iter"
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/navigation/internal/pathfind"
	"github.com/kjkrol/uid"
)

// Finder searches the board for a unit's route, over its terrain, pacing and occupancy, as the
// class of its body.
type Finder struct {
	*pathfind.Finder
	grid      grid.Grid
	Terrain   cell.Terrain
	Occupancy cell.Occupancy
	leveled   bool              // planning by levels, as a hierarchy is built over
	classes   *pathfind.Classes // nil: every unit planned as its own body
}

// NewFinder searches grid's terrain, its pacing and occupancy.
func NewFinder(grid grid.Grid, terrain cell.Terrain, pacing pathfind.Pacing,
	occupancy cell.Occupancy) *Finder {
	return &Finder{Finder: pathfind.New(grid, terrain, pacing, occupancy), grid: grid,
		Terrain: terrain, Occupancy: occupancy}
}

// WithLevels has p plan over the raised floors levels say, as it plans a board's routes.
func (p *Finder) WithLevels(levels pathfind.Levels) *Finder {
	p.Finder.WithLevels(levels)
	p.leveled = true
	return p
}

// WithClasses has p plan every unit as the class classes holds its body in, classes set at the
// latest before the first route is asked.
func (p *Finder) WithClasses(classes *pathfind.Classes) *Finder {
	p.classes = classes
	return p
}

// bodyOf is m as its routes are planned: the body of its class, on its own feet
func (p *Finder) bodyOf(m bodies.Member) pathfind.Body { return planned(p.classes, m) }

// planned is m's body as classes plan it, its own where classes is nil
func planned(classes *pathfind.Classes, m bodies.Member) pathfind.Body {
	if classes == nil {
		return m.Body()
	}
	return classes.Planned(m.Domain, m.Body())
}

// Read has p, made over a board's read and WithLevels its BoardLevels, search through r: the
// board's read on another goroutine.
func (p *Finder) Read(r *board.Read) {
	p.Terrain = r.Board()
	p.Reads(r.Board(), r, BoardLevels{Read: r})
}

// FindPath is the route from from to to, ending on level end, for a body of domain.
func (p *Finder) FindPath(
	entity uid.UID64, domain cell.Domain, body pathfind.Body, from, to cell.ID, end order.Level,
) (order.Path, bool) {
	path := order.Path{From: from}
	n, ok := p.Find(entity, domain, body, from, to, pathfind.Level(end), path.Steps[:])
	path.Length = uint16(n)
	path.Decks, path.Upper = p.Levels()
	return path, ok
}

// climbPath is FindPath climbed through h, the hierarchy serving the body's class; false where h
// refuses it.
func (p *Finder) climbPath(
	h *pathfind.Hierarchy, entity uid.UID64, domain cell.Domain, body pathfind.Body,
	from, to cell.ID, end order.Level,
) (order.Path, bool) {
	path := order.Path{From: from}
	n, ok := p.Climb(h, entity, domain, body, from, to, pathfind.Level(end), path.Steps[:])
	path.Length = uint16(n)
	path.Decks, path.Upper = p.Levels()
	return path, ok
}

// walkField is the route from from to o's Target for a body of body's class read off f, the
// field of m's group's goal.
func (p *Finder) walkField(f *pathfind.Field, m bodies.Member, body pathfind.Body, from cell.ID,
	o *order.MoveOrder) (order.Path, bool) {
	path := order.Path{From: from}
	n, ok := p.Walk(f, m.ID, m.Domain, body, from, o.Target, pathfind.Level(o.Level),
		path.Steps[:])
	path.Length = uint16(n)
	path.Decks, path.Upper = p.Levels()
	return path, ok
}

func (p *Finder) findPathAround(
	entity uid.UID64, domain cell.Domain, body pathfind.Body, from, to cell.ID, end order.Level,
	blocked func(cell.ID) bool,
) (order.Path, bool) {
	path := order.Path{From: from}
	n, ok := p.FindAround(entity, domain, body, from, to, pathfind.Level(end), blocked,
		path.Steps[:])
	path.Length = uint16(n)
	path.Decks, path.Upper = p.Levels()
	return path, ok
}

// nearestFree is NearestFree asking h, the hierarchy serving the body's class where nil is none,
// which cells no route reaches once a search found none
func (p *Finder) nearestFree(
	h *pathfind.Hierarchy, entity uid.UID64, domain cell.Domain, body pathfind.Body,
	from, target cell.ID, end order.Level, taken func(c cell.ID, level cell.Level) bool,
) (cell.ID, order.Path, bool) {
	path := order.Path{From: from}
	dest, n, ok := p.NearestFreeThrough(h, entity, domain, body, from, target,
		pathfind.Level(end), taken, path.Steps[:])
	path.Length = uint16(n)
	path.Decks, path.Upper = p.Levels()
	return dest, path, ok
}

func (p *Finder) on(m bodies.Member, c cell.ID, level order.Level) bool {
	return p.On(m.Domain, p.bodyOf(m), c, pathfind.Level(level))
}

func (p *Finder) price(from, to cell.ID, kind cell.Kind, d cell.Domain) (float64, bool) {
	return p.Price(from, to, kind, cell.OnGround, d)
}

// asides reports whether m may be given way onto c: a step it takes whose slope leaves it at
// least YieldShare of the pace the ground gives it on the level, so a flat forest is stepped
// into and a cliff is not
func (p *Finder) asides(m bodies.Member, c cell.ID) bool {
	_, ok := p.stepsAside(m, c)
	return ok
}

func (p *Finder) stepsAside(m bodies.Member, c cell.ID) (level cell.Level, ok bool) {
	level, ok = p.stepsOnto(m, c)
	if !ok || c == m.Cell || !p.Sloped() {
		return level, ok
	}
	kind := p.KindAt(c, level)
	flat := p.Flat(c, kind, level, m.Domain)
	return level, flat > 0 && p.Pace(m.Cell, c, kind, level, m.Domain) >= YieldShare*flat
}

func (p *Finder) stepsOnto(m bodies.Member, c cell.ID) (level cell.Level, ok bool) {
	if c == m.Cell {
		return p.levelOn(m, c), true
	}
	if _, ok := grid.Way(p.grid, m.Cell, c); !ok {
		return cell.OnGround, true
	}
	return p.Steps(m.Domain, p.bodyOf(m), m.Cell, c)
}

// the level of c m stands on, the ground's on a cell of one level
func (p *Finder) levelOn(m bodies.Member, c cell.ID) cell.Level {
	l, _ := p.Level(m.Domain, p.bodyOf(m), c).Of()
	return l
}

// rings are the cells round target dom crosses, the nearest first, priced for costs as a route
// is
func (p *Finder) rings(target cell.ID, dom, costs cell.Domain) iter.Seq[cell.ID] {
	return func(yield func(cell.ID) bool) {
		type reach struct {
			c    cell.ID
			cost float64
		}
		best := map[cell.ID]float64{target: 0}
		done := map[cell.ID]bool{}
		frontier := []reach{{target, 0}}
		for len(frontier) > 0 {
			n := 0
			for i := range frontier {
				if frontier[i].cost < frontier[n].cost {
					n = i
				}
			}
			r := frontier[n]
			frontier = slices.Delete(frontier, n, n+1)
			if done[r.c] {
				continue
			}
			done[r.c] = true
			if !yield(r.c) {
				return
			}
			for i := range p.grid.Directions() {
				nb, ok := p.grid.Toward(r.c, i)
				if !ok {
					continue
				}
				kind := p.Terrain.Kind(nb)
				if done[nb] || !kind.Admits(dom) {
					continue
				}
				step, ok := p.price(r.c, nb, kind, costs)
				if !ok {
					continue
				}
				cost := r.cost + step
				if b, seen := best[nb]; !seen || cost < b {
					best[nb] = cost
					frontier = append(frontier, reach{nb, cost})
				}
			}
		}
	}
}

// tilesRound are the first placeCells of the rings
func (p *Finder) tilesRound(target cell.ID, dom, costs cell.Domain) []cell.ID {
	var out []cell.ID
	for c := range p.rings(target, dom, costs) {
		if out = append(out, c); len(out) == placeCells {
			break
		}
	}
	return out
}

// what a search asks of the terrain beyond its kinds, by assertion: a board's read answering by
// another signature would be searched as though nothing stood on it
var _ interface {
	Reaches(c, n cell.ID) bool
	ReachesOn(c, n cell.ID, level cell.Level) bool
	Fenced(from, to cell.ID, d cell.Domain, level cell.Level) bool
	Lane(from, to cell.ID, d cell.Domain, level cell.Level) float32
	Limits(c cell.ID, d cell.Domain, level cell.Level) (float32, cell.Gap)
	Railed(c cell.ID, d cell.Domain, level cell.Level) bool
} = (*board.BoardRead)(nil)

// BoardLevels are the board's decks and its ground as the search reads them.
type BoardLevels struct{ Read *board.Read }

func (l BoardLevels) Storeys(c cell.ID, dst *[cell.Storeys]pathfind.Storey) (made, sunk uint8) {
	brd := l.Read.Board()
	if !brd.Storeyed() {
		return 0, 0
	}
	b := brd.Built(c)
	if !b.Levelled() {
		return 0, 0
	}
	_, level := brd.Map().Top(c)
	for s := range uint8(cell.Storeys) {
		f := b.Storey(s)
		if f.Height == 0 || f.Allows == 0 {
			continue
		}
		top := level + f.Height
		made |= 1 << s
		dst[s] = pathfind.Storey{
			Top:     top,
			Dip:     brd.FloorDip(c, *f),
			Bound:   top - f.Thickness,
			Carries: f.Allows,
		}
		if f.Sunk() {
			sunk |= 1 << s
			dst[s].Bound = top + f.Clear
		}
	}
	return made, sunk
}

func (l BoardLevels) Ground(p geom.Vec) float32 {
	if h := l.Read.Heights(); h != nil {
		return h.At(p)
	}
	return 0
}

func (l BoardLevels) Over(p geom.Vec) (under float32, ok bool) {
	brd := l.Read.Board()
	brd.DecksOver(p, func(c cell.ID, f cell.Floor) {
		if f.Height <= 0 {
			return
		}
		_, level := brd.Map().Top(c)
		if u := level + f.Height - f.Thickness; !ok || u < under {
			under, ok = u, true
		}
	})
	return under, ok
}

func (l BoardLevels) KindAt(c cell.ID, level cell.Level) cell.Kind {
	return l.Read.Board().KindAt(c, level)
}
