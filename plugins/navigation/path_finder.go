package navigation

import (
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/navigation/internal/pathfind"
	"github.com/kjkrol/uid"
)

// pathFinder is the plugin's route finder over its board, its routes laid in Paths, with what the
// keepers read beside it.
type pathFinder struct {
	*pathfind.Finder
	grid      grid.Grid
	terrain   cell.Terrain
	occupancy cell.Occupancy
}

// newPathFinder builds a pathFinder over grid that respects terrain, its slopes and occupancy.
func newPathFinder(grid grid.Grid, terrain cell.Terrain, slopes pathfind.Slopes, occupancy cell.Occupancy) *pathFinder {
	return &pathFinder{Finder: pathfind.New(grid, terrain, slopes, occupancy), grid: grid, terrain: terrain, occupancy: occupancy}
}

// findPath is the route from from toward to for entity moving in domain, its first MaxPathLength
// steps; false if unreachable.
func (p *pathFinder) findPath(entity uid.UID64, domain cell.Domain, from, to cell.ID) (Path, bool) {
	var path Path
	n, ok := p.Find(entity, domain, from, to, path.Steps[:])
	path.Length = uint16(n)
	return path, ok
}

// findPathAround is findPath going round every cell blocked reports.
func (p *pathFinder) findPathAround(entity uid.UID64, domain cell.Domain, from, to cell.ID, blocked func(cell.ID) bool) (Path, bool) {
	var path Path
	n, ok := p.FindAround(entity, domain, from, to, blocked, path.Steps[:])
	path.Length = uint16(n)
	return path, ok
}

// nearestFree is the free cell nearest target entity can reach — not one taken says — and the
// route to it.
func (p *pathFinder) nearestFree(entity uid.UID64, domain cell.Domain, from, target cell.ID, taken func(cell.ID) bool) (cell.ID, Path, bool) {
	var path Path
	dest, n, ok := p.NearestFree(entity, domain, from, target, taken, path.Steps[:])
	path.Length = uint16(n)
	return dest, path, ok
}

// climb is the slope's price of the step from one cell to its neighbour (Finder.Climb).
func (p *pathFinder) climb(from, to cell.ID, d cell.Domain) float64 { return p.Climb(from, to, d) }

// price is what the step costs an entity moving in d, to's kind being kind (Finder.Price).
func (p *pathFinder) price(from, to cell.ID, kind cell.Kind, d cell.Domain) (float64, bool) {
	return p.Price(from, to, kind, d)
}
