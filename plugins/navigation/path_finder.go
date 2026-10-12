package navigation

import (
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/navigation/internal/pathfind"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

type pathFinder struct {
	*pathfind.Finder
	grid      grid.Grid
	terrain   cell.Terrain
	occupancy cell.Occupancy
}

func newPathFinder(grid grid.Grid, terrain cell.Terrain, slopes pathfind.Slopes, occupancy cell.Occupancy) *pathFinder {
	return &pathFinder{Finder: pathfind.New(grid, terrain, slopes, occupancy), grid: grid, terrain: terrain, occupancy: occupancy}
}

func (p *pathFinder) findPath(
	entity uid.UID64, domain cell.Domain, z world.Z, from, to cell.ID, end Level,
) (Path, bool) {
	path := Path{From: from}
	n, ok := p.Find(entity, domain, bodyOf(z), from, to, pathfind.Level(end), path.Steps[:])
	path.Length, path.Decks = uint16(n), p.Decks()
	return path, ok
}

func (p *pathFinder) findPathAround(
	entity uid.UID64, domain cell.Domain, z world.Z, from, to cell.ID, end Level,
	blocked func(cell.ID) bool,
) (Path, bool) {
	path := Path{From: from}
	n, ok := p.FindAround(entity, domain, bodyOf(z), from, to, pathfind.Level(end), blocked,
		path.Steps[:])
	path.Length, path.Decks = uint16(n), p.Decks()
	return path, ok
}

func (p *pathFinder) nearestFree(
	entity uid.UID64, domain cell.Domain, z world.Z, from, target cell.ID, end Level,
	taken func(c cell.ID, deck bool) bool,
) (cell.ID, Path, bool) {
	path := Path{From: from}
	dest, n, ok := p.NearestFree(entity, domain, bodyOf(z), from, target, pathfind.Level(end),
		taken, path.Steps[:])
	path.Length, path.Decks = uint16(n), p.Decks()
	return dest, path, ok
}

func (p *pathFinder) on(m member, c cell.ID, level Level) bool {
	return p.On(m.domain, bodyOf(m.z), c, pathfind.Level(level))
}

func (p *pathFinder) climb(from, to cell.ID, d cell.Domain) float64 { return p.Climb(from, to, d) }

func (p *pathFinder) price(from, to cell.ID, kind cell.Kind, d cell.Domain) (float64, bool) {
	return p.Price(from, to, kind, d)
}

func (p *pathFinder) steps(m member, c cell.ID) bool {
	_, ok := p.stepsOnto(m, c)
	return ok
}

func (p *pathFinder) stepsOnto(m member, c cell.ID) (deck, ok bool) {
	if c == m.cell {
		return p.onDeck(m, c), true
	}
	if !slices.Contains(p.grid.Neighbors(m.cell), c) {
		return false, true
	}
	return p.Steps(m.domain, bodyOf(m.z), m.cell, c)
}

func (p *pathFinder) onDeck(m member, c cell.ID) bool {
	return p.Level(m.domain, bodyOf(m.z), c) == pathfind.Deck
}

func bodyOf(z world.Z) pathfind.Body {
	return pathfind.Body{Feet: z.Altitude, Step: z.Reach(), Height: z.Height}
}

type boardLevels struct{ brd *board.Plugin }

func (l boardLevels) Deck(c cell.ID) (top, under float64, carries cell.Domain, ok bool) {
	f := l.brd.Board().Built(c).Floor
	if f.Height <= 0 {
		return 0, 0, 0, false
	}
	_, level := l.brd.Board().Map().Top(c)
	top = float64(level) + f.Height
	return top, top - f.Thickness, f.Allows, true
}

func (l boardLevels) Ground(p geom.Vec) float64 {
	if h := l.brd.Heights(); h != nil {
		return h.At(p)
	}
	return 0
}

func (l boardLevels) Over(p geom.Vec) (under float64, ok bool) {
	brd := l.brd.Board()
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

func (l boardLevels) KindAt(c cell.ID, deck bool) cell.Kind {
	return l.brd.Board().KindAt(c, deck)
}
