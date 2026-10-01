package pathfind

import (
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/uid"
)

// MaxPathLength is how many steps a Path holds, as the navigation's.
const MaxPathLength = 64

// legCellSize is the side of the tests' cells.
const legCellSize = uint32(32)

// Path is a route as the navigation keeps it: its steps and how many.
type Path struct {
	Steps  [MaxPathLength]cell.ID
	Length uint16
}

// testFinder is a Finder laying its routes in Paths, as the navigation does.
type testFinder struct{ *Finder }

func newPathFinder(g grid.Grid, terrain cell.Terrain, slopes Slopes, occupancy cell.Occupancy) testFinder {
	return testFinder{New(g, terrain, slopes, occupancy)}
}

func (f testFinder) findPath(entity uid.UID64, domain cell.Domain, from, to cell.ID) (Path, bool) {
	var path Path
	n, ok := f.Find(entity, domain, from, to, path.Steps[:])
	path.Length = uint16(n)
	return path, ok
}

func (f testFinder) nearestFree(entity uid.UID64, domain cell.Domain, from, target cell.ID, taken func(cell.ID) bool) (cell.ID, Path, bool) {
	var path Path
	dest, n, ok := f.NearestFree(entity, domain, from, target, taken, path.Steps[:])
	path.Length = uint16(n)
	return dest, path, ok
}

func (f testFinder) price(from, to cell.ID, kind cell.Kind, d cell.Domain) (float64, bool) {
	return f.Price(from, to, kind, d)
}

func chebyshev(grid grid.Grid, a, b cell.ID) float64 {
	ca, cb := grid.CellCenter(a), grid.CellCenter(b)
	return max(abs(ca.X-cb.X), abs(ca.Y-cb.Y)) / float64(grid.CellSpan())
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func openTerrain() *cell.TerrainMap {
	terrain := cell.NewTerrainMap()
	terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	return terrain
}

// stubInstallCtx is a minimal plugin.Installer for tests that call Install directly.
type stubInstallCtx struct {
	ecs     *goke.ECS
	pending []func() []goke.System
}

func (c *stubInstallCtx) UseModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(si *goke.SysInit) { m.RegSystems(c.ecs) }}
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}
func (c *stubInstallCtx) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.pending = append(c.pending, p.SetupSystems)
	}
}
func (c *stubInstallCtx) RegSys(factory func() goke.System) goke.Runnable {
	return c.ecs.RegSys(factory())
}
func (c *stubInstallCtx) ECS() *goke.ECS { return c.ecs }
