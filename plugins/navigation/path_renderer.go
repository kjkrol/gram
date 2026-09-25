package navigation

import (
	"github.com/hajimehoshi/ebiten/v2"
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
	batch   *render.QuadBatch
	camera  camera.Camera
	space   *aabbworld.Space
	// heights is the grid's altitudes when it has them (a Board), for laying sprites on the ground;
	// corners its corner heights on a sloped grid, so a sprite lies on the tile as it is drawn.
	heights interface{ Altitude(board.CellID) float64 }
	corners interface {
		Corners(board.CellID) ([4]float64, uint32, uint32, bool)
	}

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

var _ render.Renderer = (*PathRenderer)(nil)

func NewPathRenderer(cam camera.Camera, grid board.Grid, atlas render.AtlasSource, sprites PathSprites, selected plugin.Tag[selection.Family]) *PathRenderer {
	r := &PathRenderer{grid: grid, sprites: sprites, batch: render.NewQuadBatch(atlas, cam), camera: cam, selected: selected}
	r.heights, _ = grid.(interface{ Altitude(board.CellID) float64 })
	r.corners, _ = grid.(interface {
		Corners(board.CellID) ([4]float64, uint32, uint32, bool)
	})
	return r
}

func (r *PathRenderer) BindSpace(space *aabbworld.Space) { r.space = space }

func (r *PathRenderer) Init(si *goke.SysInit) {
	r.query = si.NewQueryBuilder(&r.base, &r.cell, &r.order, &r.marks).
		Optional(&r.mover).
		Build()
}

func (r *PathRenderer) Draw(screen *ebiten.Image) {
	r.batch.Reset()
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
	r.batch.Flush(screen)
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
// the neighbour's begins; through an isometric camera it lies on the ground at the cell's altitude.
func (r *PathRenderer) appendCellSprite(c board.CellID, sprite render.SpriteID) {
	center := r.grid.CellCenter(c)
	w, h := r.grid.CellBounds()
	half := min(w, h) / 2
	x0, y0 := float32(center.X-half), float32(center.Y-half)
	x1, y1 := float32(center.X+half), float32(center.Y+half)
	if _, iso := r.camera.Projection().(camera.Isometric); iso {
		z := r.spriteHeights(c)
		var dst render.Corners
		for i, p := range [4][2]float32{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}} {
			dst[i][0], dst[i][1] = r.camera.Project(p[0], p[1], z[i])
		}
		r.batch.AppendCorners(dst, sprite)
		return
	}
	r.batch.AppendQuad(x0, y0, x1, y1, sprite)
}

// spriteHeights is the height of a cell sprite's four corners: the tile's own corners on a sloped
// grid (the sprite covers a square cell), else the cell's altitude, else the ground at 0.
func (r *PathRenderer) spriteHeights(c board.CellID) [4]float32 {
	if r.corners != nil {
		if hs, _, _, ok := r.corners.Corners(c); ok {
			return [4]float32{float32(hs[0]), float32(hs[1]), float32(hs[2]), float32(hs[3])}
		}
	}
	alt := float32(0)
	if r.heights != nil {
		alt = float32(r.heights.Altitude(c))
	}
	return [4]float32{alt, alt, alt, alt}
}

func hasPassedCenter(cellCenter, entityCenter, travel geom.Vec, width, height uint32, edges aabbworld.Edges) bool {
	ex := shortestAxisDelta(cellCenter.X, entityCenter.X, width, edges.WrapsX())
	ey := shortestAxisDelta(cellCenter.Y, entityCenter.Y, height, edges.WrapsY())
	return ex*travel.X+ey*travel.Y > 0
}
