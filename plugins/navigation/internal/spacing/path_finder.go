package spacing

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/navigation/internal/pathfind"
	"github.com/kjkrol/uid"
)

type Finder struct {
	*pathfind.Finder
	grid      grid.Grid
	Terrain   cell.Terrain
	Occupancy cell.Occupancy
}

func NewFinder(grid grid.Grid, terrain cell.Terrain, slopes pathfind.Slopes, occupancy cell.Occupancy) *Finder {
	return &Finder{Finder: pathfind.New(grid, terrain, slopes, occupancy), grid: grid, Terrain: terrain, Occupancy: occupancy}
}

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

type BoardLevels struct{ Read *board.Read }

func (l BoardLevels) Deck(c cell.ID) (top, under float64, carries cell.Domain, ok bool) {
	f := l.Read.Board().Built(c).Floor
	if f.Height <= 0 {
		return 0, 0, 0, false
	}
	_, level := l.Read.Board().Map().Top(c)
	top = float64(level) + f.Height
	return top, top - f.Thickness, f.Allows, true
}

func (l BoardLevels) Ground(p geom.Vec) float64 {
	if h := l.Read.Heights(); h != nil {
		return h.At(p)
	}
	return 0
}

func (l BoardLevels) Over(p geom.Vec) (under float64, ok bool) {
	brd := l.Read.Board()
	brd.DecksOver(p, func(c cell.ID, f cell.Floor) {
		if f.Height <= 0 {
			return
		}
		_, level := brd.Map().Top(c)
		if u := float64(level) + f.Height - f.Thickness; !ok || u < under {
			under, ok = u, true
		}
	})
	return under, ok
}

func (l BoardLevels) KindAt(c cell.ID, deck bool) cell.Kind {
	return l.Read.Board().KindAt(c, deck)
}
