package navigation

import (
	"image/color"
	"math"

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

// RouteStyle is how routes and goals are drawn: a route a line in Line, Width pixels wide; a goal
// the entity's outline where it will stand, in Goal.
type RouteStyle struct {
	Line, Goal color.RGBA
	Width      float32
}

// DefaultRouteStyle is a thin orange line and an orange-yellow outline.
var DefaultRouteStyle = RouteStyle{Line: color.RGBA{R: 255, G: 140, B: 0, A: 220}, Goal: color.RGBA{R: 255, G: 200, B: 40, A: 255}, Width: 1.5}

// goalWidth is how wide a goal's outline is drawn: the selection's.
const goalWidth = 2

// PathRenderer draws, for every selected entity, its goals — the entity's outline where it will
// stand, on the render.Marks tier, always — and, when the routes are shown (Routes, Shift+P), the
// remaining route and the routes on to each queued goal. In a world with heights, through a
// camera with Rays, a route is a render.Direct drawing on the GPU on RouteTier: laid on the ground
// the frame drew, read from its depth, so it follows every rise and a hill in front hides it.
// Otherwise it is a line over the ground on the render.Overlays tier in pieces of the ground's
// step, each at the depth of the ground under it, so the line runs straight through any camera.
type PathRenderer struct {
	grid   board.Grid
	style  RouteStyle
	frame  *render.Frame // the one being composed
	camera camera.Camera // the one of the frame being composed
	space  *aabbworld.Space
	// heights is the ground the routes and goals lie on, read when composing starts; nil, level
	// at 0.
	heights func() board.Heights
	ground  board.Heights
	step    float64
	// look lays a goal's outline as the world draws the entity; nil, the box as it lies.
	look   func() world.Look
	routes bool // the routes are drawn; the goals always are

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

	footprint []render.Corners // reused

	gpu   *routes // the routes drawn on the GPU
	onGPU bool    // this frame's
}

var _ render.Direct = (*PathRenderer)(nil)

// NewPathRenderer draws the routes and goals of the entities carrying selected, as style says.
func NewPathRenderer(grid board.Grid, style RouteStyle, selected plugin.Tag[selection.Family]) *PathRenderer {
	if style == (RouteStyle{}) {
		style = DefaultRouteStyle
	}
	return &PathRenderer{grid: grid, style: style, selected: selected, gpu: newRoutes()}
}

// Tier is where the routes drawn on the GPU come: RouteTier.
func (r *PathRenderer) Tier() render.Tier { return RouteTier }

// Draw lays the routes Compose gathered on the ground, on the GPU; nothing where it composed them.
func (r *PathRenderer) Draw(t render.Target, cam camera.Camera, _ render.Uniforms) {
	if r.onGPU {
		r.gpu.draw(t, cam, r.style.Line, r.style.Width)
	}
}

// WithHeights has the routes and goals laid on the ground heights gives when composing starts:
// board.Plugin.Heights; nil, level at 0.
func (r *PathRenderer) WithHeights(heights func() board.Heights) *PathRenderer {
	r.heights = heights
	return r
}

// WithLook has a goal outlined as look draws the entity standing there: world.Plugin.Look.
func (r *PathRenderer) WithLook(look func() world.Look) *PathRenderer {
	r.look = look
	return r
}

// ShowRoutes has the routes drawn, or the goals alone.
func (r *PathRenderer) ShowRoutes(shown bool) { r.routes = shown }

// RoutesShown reports whether the routes are drawn.
func (r *PathRenderer) RoutesShown() bool { return r.routes }

func (r *PathRenderer) BindSpace(space *aabbworld.Space) { r.space = space }

func (r *PathRenderer) Init(si *goke.SysInit) {
	r.query = si.NewQueryBuilder(&r.base, &r.cell, &r.order, &r.marks).
		Optional(&r.mover).
		Build()
}

// Compose hands f the goals of the selected units, and their routes when shown.
func (r *PathRenderer) Compose(f *render.Frame, cam camera.Camera) {
	r.frame, r.camera = f, cam
	r.ground, r.step = nil, 0
	if r.heights != nil {
		if r.ground = r.heights(); r.ground != nil {
			r.step = r.ground.Step()
		}
	}
	_, rays := cam.(camera.Rays)
	r.onGPU = rays && r.ground != nil && r.space != nil && r.space.Edges == 0
	if r.space == nil {
		return
	}
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
			o := &orders[i]
			size := bases[i].Pos.Size
			r.goal(size, o.Target, o.Spot)
			for k := range o.Waypoints[:o.Queued] {
				r.goal(size, o.Waypoints[k].Cell, o.Waypoints[k].Spot)
			}
			if !r.routes {
				continue
			}
			center := board.Center(bases[i].Pos)
			r.drawPath(center, bases[i].Vel.Dir, pathCells(cells[i], *o), r.point(o.Target, o.Spot))
			from := r.point(o.Target, o.Spot)
			for k, route := range r.queued(cursor.IDs[i], board.DomainAt(movers, i), o) {
				to := r.point(o.Waypoints[k].Cell, o.Waypoints[k].Spot)
				r.drawRoute(route, from, to)
				from = to
			}
		}
	}
}

// point is where a goal is: its spot, or its cell's centre.
func (r *PathRenderer) point(c board.CellID, spot geom.Vec) geom.Vec {
	if spot != (geom.Vec{}) {
		return spot
	}
	return r.grid.CellCenter(c)
}

// drawPath draws the route from the entity through the centres of cells, ending at end, leaving
// out the first cell's centre once the entity has passed it.
func (r *PathRenderer) drawPath(entityCenter, travel geom.Vec, cells []board.CellID, end geom.Vec) {
	from := entityCenter
	for i, c := range cells {
		to := r.grid.CellCenter(c)
		if i == len(cells)-1 {
			to = end
		}
		if i == 0 && hasPassedCenter(to, entityCenter, travel, r.space.Width, r.space.Height, r.space.Edges) && len(cells) > 1 {
			continue
		}
		r.line(from, to)
		from = to
	}
}

// drawRoute draws a route between two goals: from the first through the centres of cells to the
// second.
func (r *PathRenderer) drawRoute(cells []board.CellID, from, to geom.Vec) {
	if len(cells) == 0 {
		r.line(from, to)
		return
	}
	at := from
	for _, c := range cells[1 : len(cells)-1] {
		next := r.grid.CellCenter(c)
		r.line(at, next)
		at = next
	}
	r.line(at, to)
}

// line draws the stretch from a to b over the ground: on a wrapping world flat, the short way
// round; else in pieces of the ground's step, each on the ground under its ends and at the depth
// of its middle.
func (r *PathRenderer) line(a, b geom.Vec) {
	if r.space != nil && r.space.Edges != 0 {
		dx := shortestAxisDelta(a.X, b.X, r.space.Width, r.space.Edges.WrapsX())
		dy := shortestAxisDelta(a.Y, b.Y, r.space.Height, r.space.Edges.WrapsY())
		x0, y0 := r.camera.ToScreen(float32(a.X), float32(a.Y))
		x1, y1 := r.camera.ToScreen(float32(a.X+dx), float32(a.Y+dy))
		r.frame.Line(render.Overlays, 0, x0, y0, x1, y1, r.style.Width, r.style.Line)
		return
	}
	if r.onGPU {
		r.gpu.add(r.camera, a, b, r.groundAt, r.step, r.style.Width)
		return
	}
	length := math.Hypot(b.X-a.X, b.Y-a.Y)
	pieces := 1
	if r.step > 0 && length > r.step {
		pieces = int(math.Ceil(length / r.step))
	}
	x0, y0 := r.on(a)
	for i := 1; i <= pieces; i++ {
		t := float64(i) / float64(pieces)
		p := geom.NewVec(a.X+(b.X-a.X)*t, a.Y+(b.Y-a.Y)*t)
		x1, y1 := r.on(p)
		mid := geom.NewVec(a.X+(b.X-a.X)*(t-0.5/float64(pieces)), a.Y+(b.Y-a.Y)*(t-0.5/float64(pieces)))
		r.frame.Line(render.Overlays, r.camera.Depth(float32(mid.X), float32(mid.Y), r.groundAt(mid)), x0, y0, x1, y1, r.style.Width, r.style.Line)
		x0, y0 = x1, y1
	}
}

// groundAt is the height of the ground at p.
func (r *PathRenderer) groundAt(p geom.Vec) float32 {
	if r.ground == nil {
		return 0
	}
	return float32(r.ground.At(p))
}

// on is where the ground point p is drawn.
func (r *PathRenderer) on(p geom.Vec) (float32, float32) {
	return r.camera.Project(float32(p.X), float32(p.Y), r.groundAt(p))
}

// goal outlines a box of size standing on the goal — its spot, or its cell's centre — on the
// ground there, on the Marks tier, as the look draws it.
func (r *PathRenderer) goal(size geom.Vec, c board.CellID, spot geom.Vec) {
	at := r.point(c, spot)
	box := geom.NewAABBAt(geom.NewVec(at.X-size.X/2, at.Y-size.Y/2), size.X, size.Y)
	alt := r.groundAt(at)
	r.footprint = r.footprint[:0]
	if r.look != nil {
		r.footprint = r.look().Footprint(r.camera, box, alt, r.footprint)
	} else {
		var q render.Corners
		for i, p := range [4][2]float32{{float32(box.TopLeft.X), float32(box.TopLeft.Y)}, {float32(box.BottomRight.X), float32(box.TopLeft.Y)}, {float32(box.TopLeft.X), float32(box.BottomRight.Y)}, {float32(box.BottomRight.X), float32(box.BottomRight.Y)}} {
			q[i][0], q[i][1] = r.camera.Project(p[0], p[1], alt)
		}
		r.footprint = append(r.footprint, q)
	}
	for _, q := range r.footprint {
		pts := [4][2]float32{q[0], q[1], q[3], q[2]}
		for i, p := range pts {
			n := pts[(i+1)%len(pts)]
			r.frame.Line(render.Marks, 0, p[0], p[1], n[0], n[1], goalWidth, r.style.Goal)
		}
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
	for k, g := range mt.Waypoints[:mt.Queued] {
		goals[k+1] = g.Cell
	}
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

func hasPassedCenter(cellCenter, entityCenter, travel geom.Vec, width, height uint32, edges aabbworld.Edges) bool {
	ex := shortestAxisDelta(cellCenter.X, entityCenter.X, width, edges.WrapsX())
	ey := shortestAxisDelta(cellCenter.Y, entityCenter.Y, height, edges.WrapsY())
	return ex*travel.X+ey*travel.Y > 0
}
