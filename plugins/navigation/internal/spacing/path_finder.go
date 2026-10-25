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

// Finder searches the board for a unit's route, over its terrain, slopes and occupancy.
type Finder struct {
	*pathfind.Finder
	grid      grid.Grid
	Terrain   cell.Terrain
	Occupancy cell.Occupancy
}

// NewFinder searches grid's terrain, its slopes and occupancy.
func NewFinder(grid grid.Grid, terrain cell.Terrain, slopes pathfind.Slopes, occupancy cell.Occupancy) *Finder {
	return &Finder{Finder: pathfind.New(grid, terrain, slopes, occupancy), grid: grid, Terrain: terrain, Occupancy: occupancy}
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
	path.Length, path.Decks = uint16(n), p.Decks()
	return path, ok
}

func (p *Finder) findPathAround(
	entity uid.UID64, domain cell.Domain, body pathfind.Body, from, to cell.ID, end order.Level,
	blocked func(cell.ID) bool,
) (order.Path, bool) {
	path := order.Path{From: from}
	n, ok := p.FindAround(entity, domain, body, from, to, pathfind.Level(end), blocked,
		path.Steps[:])
	path.Length, path.Decks = uint16(n), p.Decks()
	return path, ok
}

func (p *Finder) nearestFree(
	entity uid.UID64, domain cell.Domain, body pathfind.Body, from, target cell.ID, end order.Level,
	taken func(c cell.ID, deck bool) bool,
) (cell.ID, order.Path, bool) {
	path := order.Path{From: from}
	dest, n, ok := p.NearestFree(entity, domain, body, from, target, pathfind.Level(end),
		taken, path.Steps[:])
	path.Length, path.Decks = uint16(n), p.Decks()
	return dest, path, ok
}

func (p *Finder) on(m bodies.Member, c cell.ID, level order.Level) bool {
	return p.On(m.Domain, m.Body(), c, pathfind.Level(level))
}

func (p *Finder) climb(from, to cell.ID, d cell.Domain) float64 { return p.Climb(from, to, d) }

func (p *Finder) price(from, to cell.ID, kind cell.Kind, d cell.Domain) (float64, bool) {
	return p.Price(from, to, kind, d)
}

func (p *Finder) steps(m bodies.Member, c cell.ID) bool {
	_, ok := p.stepsOnto(m, c)
	return ok
}

func (p *Finder) stepsOnto(m bodies.Member, c cell.ID) (deck, ok bool) {
	if c == m.Cell {
		return p.onDeck(m, c), true
	}
	if _, ok := grid.Way(p.grid, m.Cell, c); !ok {
		return false, true
	}
	return p.Steps(m.Domain, m.Body(), m.Cell, c)
}

func (p *Finder) onDeck(m bodies.Member, c cell.ID) bool {
	return p.Level(m.Domain, m.Body(), c) == pathfind.Deck
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

// BoardLevels are the board's decks and its ground as the search reads them.
type BoardLevels struct{ Read *board.Read }

func (l BoardLevels) Deck(c cell.ID) (top, under float32, carries cell.Domain, ok bool) {
	f := l.Read.Board().Built(c).Floor
	if f.Height <= 0 {
		return 0, 0, 0, false
	}
	_, level := l.Read.Board().Map().Top(c)
	top = level + f.Height
	return top, top - f.Thickness, f.Allows, true
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

func (l BoardLevels) KindAt(c cell.ID, deck bool) cell.Kind {
	return l.Read.Board().KindAt(c, deck)
}
