package pathfind

import (
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/hosts"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

// as the navigation's
const MaxPathLength = 64

const legCellSize = uint32(32)

type Path struct {
	Steps  [MaxPathLength]cell.ID
	Length uint16
}

type testFinder struct{ *Finder }

func newPathFinder(g grid.Grid, terrain cell.Terrain, slopes Slopes, occupancy cell.Occupancy) testFinder {
	return testFinder{New(g, terrain, slopes, occupancy)}
}

func (f testFinder) findPath(entity uid.UID64, domain cell.Domain, from, to cell.ID) (Path, bool) {
	var path Path
	n, ok := f.Find(entity, domain, Body{}, from, to, Either, path.Steps[:])
	path.Length = uint16(n)
	return path, ok
}

func (f testFinder) nearestFree(entity uid.UID64, domain cell.Domain, from, target cell.ID, taken func(cell.ID) bool) (cell.ID, Path, bool) {
	var path Path
	takenOn := func(c cell.ID, _ bool) bool { return taken(c) }
	dest, n, ok := f.NearestFree(entity, domain, Body{}, from, target, Either, takenOn,
		path.Steps[:])
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

type stubInstallCtx struct {
	hosts   []plugin.Host // of the rules of the moments the plugins installed catch
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

func (c *stubInstallCtx) Hosts(h ...plugin.Host) { c.hosts = append(c.hosts, h...) }

// as the engine does with the roles played once a Stage's Init returns
func (c *stubInstallCtx) Deliver(rules ...rule.Rule) error { return hosts.Deliver(c.hosts, rules...) }
