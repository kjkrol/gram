package navigation

import (
	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

func pathCells(cell board.Cell, mt MoveOrder) []board.CellID {
	cells := []board.CellID{cell.ID}
	next := mt.Target
	if mt.Path.Index < mt.Path.Length {
		next = mt.Path.Steps[mt.Path.Index]
	}
	if mt.Leg.Active && mt.Leg.To != cell.ID && mt.Leg.To != next {
		cells = append(cells, mt.Leg.To)
	}
	for s := mt.Path.Index; s < mt.Path.Length; s++ {
		if step := mt.Path.Steps[s]; step != cells[len(cells)-1] {
			cells = append(cells, step)
		}
	}
	if mt.Path.Length == 0 || mt.Path.Index >= mt.Path.Length {
		if mt.Target != cells[len(cells)-1] {
			cells = append(cells, mt.Target)
		}
	}
	return cells
}

type PathRenderer struct {
	grid    board.Grid
	sprites PathSprites
	atlas   render.AtlasSource
	frame   *render.Frame // the one being composed
	camera  camera.Camera // the one of the frame being composed
	space   *aabbworld.Space
	// tops is the height of a cell's corners and of its ground as the board's Map draws them, for
	// laying sprites on the tiles as they are drawn; nil is level ground at 0.
	tops func(board.CellID) (corners [4]float32, level float32)

	// finder plans the routes between queued goals for the preview; nil draws the goals alone.
	finder   *pathFinder
	previews map[uid.UID64]*preview

	selected plugin.Tag[selection.Family]

	query *goke.Query
	base  goke.Comp[world.Base]
	cell  goke.Comp[board.Cell]
	order goke.Comp[MoveOrder]
	marks goke.Comp[plugin.Tags[selection.Family]]
	mover goke.OptComp[board.Mover]
}

var _ render.Source = (*PathRenderer)(nil)

func NewPathRenderer(grid board.Grid, atlas render.AtlasSource, sprites PathSprites, selected plugin.Tag[selection.Family]) *PathRenderer {
	return &PathRenderer{grid: grid, sprites: sprites, atlas: atlas, selected: selected}
}

// WithTops has the routes laid on the tiles as the board's Map draws them: tops is board.Plugin.Top.
func (r *PathRenderer) WithTops(tops func(board.CellID) (corners [4]float32, level float32)) *PathRenderer {
	r.tops = tops
	return r
}

func (r *PathRenderer) BindSpace(space *aabbworld.Space) { r.space = space }

func (r *PathRenderer) Init(si *goke.SysInit) {
	r.query = si.NewQueryBuilder(&r.base, &r.cell, &r.order, &r.marks).
		Optional(&r.mover).
		Build()
}

// Compose hands f the routes of the selected units on the Overlays tier, each sprite at the depth of
// its cell, so what stands in front of the cell hides it.
func (r *PathRenderer) Compose(f *render.Frame, cam camera.Camera) {
	r.frame, r.camera = f, cam
	if r.space != nil {
		r.query.All()
		for r.query.Next() {
			cursor := r.query.Cursor()
			bases := r.base.Slice(cursor)
			cells := r.cell.Slice(cursor)
			orders := r.order.Slice(cursor)
			marks := r.marks.Slice(cursor)
			movers := r.mover.Slice(cursor)
			for i := range cursor.IDs {
				if !marks[i].Has(r.selected) {
					continue
				}
				center := board.Center(bases[i].Pos)
				r.drawPath(center, bases[i].Vel.Dir, pathCells(cells[i], orders[i]))
				for _, route := range r.queued(cursor.IDs[i], board.DomainAt(movers, i), &orders[i]) {
					r.drawRoute(route)
				}
			}
		}
	}
}

func (r *PathRenderer) drawPath(entityCenter, travel geom.Vec, cells []board.CellID) {
	for i, c := range cells {
		center := r.grid.CellCenter(c)

		if i > 0 {
			dirBack := directionBetween(center, r.grid.CellCenter(cells[i-1]), r.space.Width, r.space.Height, r.space.Edges)
			r.appendCellSprite(c, r.sprites.spoke(dirBack))
		}

		if i == len(cells)-1 {
			r.appendCellSprite(c, r.sprites.Dot)
			continue
		}

		if i == 0 && hasPassedCenter(center, entityCenter, travel, r.space.Width, r.space.Height, r.space.Edges) {
			continue
		}

		dirOut := directionBetween(center, r.grid.CellCenter(cells[i+1]), r.space.Width, r.space.Height, r.space.Edges)
		r.appendCellSprite(c, r.sprites.spoke(dirOut))
	}
}

// drawRoute draws a route between two goals: spokes along it and a dot on its end.
func (r *PathRenderer) drawRoute(cells []board.CellID) {
	for i, c := range cells {
		center := r.grid.CellCenter(c)
		if i > 0 {
			r.appendCellSprite(c, r.sprites.spoke(directionBetween(center, r.grid.CellCenter(cells[i-1]), r.space.Width, r.space.Height, r.space.Edges)))
		}
		if i == len(cells)-1 {
			r.appendCellSprite(c, r.sprites.Dot)
			continue
		}
		r.appendCellSprite(c, r.sprites.spoke(directionBetween(center, r.grid.CellCenter(cells[i+1]), r.space.Width, r.space.Height, r.space.Edges)))
	}
}

// preview is the routes between one order's queued goals, kept until the goals change.
type preview struct {
	goals  [MaxWaypoints + 1]board.CellID
	queued uint8
	routes [][]board.CellID
}

// queued is the routes from the order's Target through each queued goal, planned once per change
// of the goals; a goal no route reaches is drawn on its own.
func (r *PathRenderer) queued(id uid.UID64, domain board.Domain, mt *MoveOrder) [][]board.CellID {
	if mt.Queued == 0 {
		delete(r.previews, id)
		return nil
	}
	var goals [MaxWaypoints + 1]board.CellID
	goals[0] = mt.Target
	copy(goals[1:], mt.Waypoints[:mt.Queued])
	if pv := r.previews[id]; pv != nil && pv.queued == mt.Queued && pv.goals == goals {
		return pv.routes
	}
	pv := &preview{goals: goals, queued: mt.Queued}
	for k := 0; k < int(mt.Queued); k++ {
		from, to := goals[k], goals[k+1]
		route := []board.CellID{from}
		if r.finder != nil {
			if path, ok := r.finder.findPath(id, domain, from, to); ok {
				for _, step := range path.Steps[:path.Length] {
					route = append(route, step)
				}
			}
		}
		if route[len(route)-1] != to {
			route = append(route, to)
		}
		pv.routes = append(pv.routes, route)
	}
	if r.previews == nil {
		r.previews = map[uid.UID64]*preview{}
	}
	r.previews[id] = pv
	return pv.routes
}

// appendCellSprite lays sprite as a square reaching the cell's nearest edges, so a spoke ends where
// the neighbour's begins, on the ground at the tile's corners and at the depth of the tile; on a
// wrapping world, seen from above, over the cell's box split at the seam.
func (r *PathRenderer) appendCellSprite(c board.CellID, sprite render.SpriteID) {
	center := r.grid.CellCenter(c)
	w, h := r.grid.CellBounds()
	half := min(w, h) / 2
	x0, y0 := float32(center.X-half), float32(center.Y-half)
	x1, y1 := float32(center.X+half), float32(center.Y+half)
	if r.space != nil && r.space.Edges != 0 {
		r.frame.SpriteRect(render.Overlays, 0, r.atlas, sprite, x0, y0, x1, y1, render.Even(1))
		return
	}
	var z [4]float32
	var alt float32
	if r.tops != nil {
		z, alt = r.tops(c)
	}
	var dst render.Corners
	for i, p := range [4][2]float32{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}} {
		dst[i][0], dst[i][1] = r.camera.Project(p[0], p[1], z[i])
	}
	r.frame.Sprite(render.Overlays, r.camera.Depth(float32(center.X), float32(center.Y), alt), r.atlas, sprite, dst, render.Even(1))
}

func hasPassedCenter(cellCenter, entityCenter, travel geom.Vec, width, height uint32, edges aabbworld.Edges) bool {
	ex := shortestAxisDelta(cellCenter.X, entityCenter.X, width, edges.WrapsX())
	ey := shortestAxisDelta(cellCenter.Y, entityCenter.Y, height, edges.WrapsY())
	return ex*travel.X+ey*travel.Y > 0
}
