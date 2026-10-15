package navigation

import (
	"image/color"
	"math"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/navigation/internal/pathfind"
	"github.com/kjkrol/gram/plugins/navigation/internal/routes"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

func pathCells(at unit.At, mt MoveOrder) []cell.ID {
	cells := []cell.ID{at.Cell}
	next := mt.Target
	if mt.Path.Index < mt.Path.Length {
		next = mt.Path.Steps[mt.Path.Index]
	}
	if mt.Leg.Active && mt.Leg.To != at.Cell && mt.Leg.To != next {
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

// RouteStyle is how routes and goals are drawn: a route a line in Line, Width pixels wide, a goal
// the entity's outline in Goal.
type RouteStyle struct {
	Line, Goal color.RGBA
	Width      float32
}

// Style: a figure declares it on the world's atlas (render.Style).
func (RouteStyle) Style() {}

var _ render.Style = RouteStyle{}

// DefaultRouteStyle is a thin orange line and an orange-yellow outline.
var DefaultRouteStyle = RouteStyle{Line: color.RGBA{R: 255, G: 140, B: 0, A: 220}, Goal: color.RGBA{R: 255, G: 200, B: 40, A: 255}, Width: 1.5}

// the selection's width
const goalWidth = 2

type pathRenderer struct {
	grid    grid.Grid
	style   RouteStyle // the kinds' default
	current RouteStyle // the unit's being drawn
	styles  func() *render.Styles
	frame   *render.Frame // the one being composed
	camera  camera.Camera // the one of the frame being composed
	space   *aabbworld.Space
	// nil: level at 0
	heights func() ground.Heights
	ground  ground.Heights
	step    float64
	// nil: the box as it lies
	look   func() world.Look
	routes bool

	// nil: the goals alone, no routes between them
	finder   *pathFinder
	previews map[uid.UID64]*preview

	selected tag.Tag[selection.Family]

	query *goke.Query
	base  goke.Comp[world.Base]
	cell  goke.Comp[unit.At]
	order goke.Comp[MoveOrder]
	marks goke.Comp[tag.Tags[selection.Family]]
	app   goke.OptComp[render.Appearance]
	mover goke.OptComp[unit.Mover]

	footprint []render.Corners

	gpu   *routes.Lines
	onGPU bool // this frame's
}

var _ render.Direct = (*pathRenderer)(nil)

func newPathRenderer(grid grid.Grid, style RouteStyle, selected tag.Tag[selection.Family]) *pathRenderer {
	if style == (RouteStyle{}) {
		style = DefaultRouteStyle
	}
	return &pathRenderer{grid: grid, style: style, current: style, selected: selected, gpu: routes.New()}
}

func (r *pathRenderer) Draw(t render.Target, cam camera.Camera, _ render.Uniforms) {
	if r.onGPU {
		r.gpu.Draw(t, cam)
	}
}

func (r *pathRenderer) WithStyles(styles func() *render.Styles) *pathRenderer {
	r.styles = styles
	return r
}

func (r *pathRenderer) WithHeights(heights func() ground.Heights) *pathRenderer {
	r.heights = heights
	return r
}

func (r *pathRenderer) WithLook(look func() world.Look) *pathRenderer {
	r.look = look
	return r
}

func (r *pathRenderer) ShowRoutes(shown bool) { r.routes = shown }

func (r *pathRenderer) RoutesShown() bool { return r.routes }

func (r *pathRenderer) BindSpace(space *aabbworld.Space) { r.space = space }

func (r *pathRenderer) Init(si *goke.SysInit) {
	r.query = si.NewQueryBuilder(&r.base, &r.cell, &r.order, &r.marks).
		Optional(&r.app).
		Optional(&r.mover).
		Build()
}

func (r *pathRenderer) Compose(f *render.Frame, cam camera.Camera) {
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
	var styles *render.Styles
	if r.styles != nil {
		styles = r.styles()
	}
	r.query.All()
	for r.query.Next() {
		cursor := r.query.Cursor()
		bases := r.base.Slice(cursor)
		cells := r.cell.Slice(cursor)
		orders := r.order.Slice(cursor)
		marks := r.marks.Slice(cursor)
		apps := r.app.Slice(cursor)
		movers := r.mover.Slice(cursor)
		for i := range cursor.IDs {
			if !marks[i].Has(r.selected) {
				continue
			}
			r.current = r.style
			if apps != nil {
				if st, ok := render.StyleOf[RouteStyle](styles, apps[i].SpriteID); ok {
					r.current = st
				}
			}
			o := &orders[i]
			r.goals(bases[i].Pos.Size, unit.DomainAt(movers, i), o)
			if !r.routes {
				continue
			}
			center := bases[i].Pos.Center()
			r.drawPath(center, bases[i].Vel.Dir, pathCells(cells[i], *o), r.point(o.Target, o.Spot))
			from := r.point(o.Target, o.Spot)
			body := bodyOf(world.Z{}, bases[i].Pos.Size)
			for k, route := range r.queued(cursor.IDs[i], unit.DomainAt(movers, i), body, o) {
				to := r.point(o.Waypoints[k].Cell, o.Waypoints[k].Spot)
				r.drawRoute(route, from, to)
				from = to
			}
		}
	}
	if r.onGPU && r.gpu.Len() > 0 {
		f.ReadGround() // the routes find the ground under them
		f.DrawAt(f.OnGround(routeLift))
	}
}

// how far over the ground a route lies among what lies on it: over the sight, under the figures
const routeLift = 0.75

// a step aside is no goal
func (r *pathRenderer) goals(size geom.Vec, domain cell.Domain, o *MoveOrder) {
	if !o.GivingWay {
		r.goal(size, o.Target, o.Spot, r.standing(r.point(o.Target, o.Spot), domain, o.Level))
	}
	for k := range o.Waypoints[:o.Queued] {
		g := &o.Waypoints[k]
		r.goal(size, g.Cell, g.Spot, r.standing(r.point(g.Cell, g.Spot), domain, g.Level))
	}
}

func (r *pathRenderer) standing(p geom.Vec, domain cell.Domain, level Level) float32 {
	if s, ok := r.ground.(ground.Surfaces); ok && level != OnGround {
		top, _ := s.Surface(p, domain, math.Inf(1), 0)
		return float32(top)
	}
	return r.groundAt(p)
}

func (r *pathRenderer) point(c cell.ID, spot geom.Vec) geom.Vec {
	if spot != (geom.Vec{}) {
		return spot
	}
	return r.grid.CellCenter(c)
}

func (r *pathRenderer) drawPath(entityCenter, travel geom.Vec, cells []cell.ID, end geom.Vec) {
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

func (r *pathRenderer) drawRoute(cells []cell.ID, from, to geom.Vec) {
	if len(cells) < 2 {
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

// on a wrapping world flat, the short way round; else in pieces of the ground's step, each at
// the depth of its middle, so the line runs straight through any camera
func (r *pathRenderer) line(a, b geom.Vec) {
	if r.space != nil && r.space.Edges != 0 {
		dx := shortestAxisDelta(a.X, b.X, r.space.Width, r.space.Edges.WrapsX())
		dy := shortestAxisDelta(a.Y, b.Y, r.space.Height, r.space.Edges.WrapsY())
		x0, y0 := r.camera.ToScreen(float32(a.X), float32(a.Y))
		x1, y1 := r.camera.ToScreen(float32(a.X+dx), float32(a.Y+dy))
		mid := geom.NewVec(a.X+dx/2, a.Y+dy/2)
		at := r.frame.Lifted(float32(mid.X), float32(mid.Y), 0, routeLift)
		r.frame.Line(at, x0, y0, x1, y1, r.current.Width, r.current.Line)
		return
	}
	if r.onGPU {
		r.gpu.Add(r.camera, a, b, r.groundAt, r.step, r.current.Width, r.current.Line)
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
		at := r.frame.Lifted(float32(mid.X), float32(mid.Y), r.groundAt(mid), routeLift)
		r.frame.Line(at, x0, y0, x1, y1, r.current.Width, r.current.Line)
		x0, y0 = x1, y1
	}
}

func (r *pathRenderer) groundAt(p geom.Vec) float32 {
	if r.ground == nil {
		return 0
	}
	return float32(r.ground.At(p))
}

func (r *pathRenderer) on(p geom.Vec) (float32, float32) {
	return r.camera.Project(float32(p.X), float32(p.Y), r.groundAt(p))
}

func (r *pathRenderer) goal(size geom.Vec, c cell.ID, spot geom.Vec, alt float32) {
	at := r.point(c, spot)
	box := geom.NewAABBAt(geom.NewVec(at.X-size.X/2, at.Y-size.Y/2), size.X, size.Y)
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
			r.frame.Line(render.OnTop, p[0], p[1], n[0], n[1], goalWidth, r.current.Goal)
		}
	}
}

type preview struct {
	goals  [MaxWaypoints + 1]cell.ID
	queued uint8
	routes [][]cell.ID
}

// planned once per change of the goals; a goal no route reaches is drawn on its own
func (r *pathRenderer) queued(id uid.UID64, domain cell.Domain, body pathfind.Body,
	mt *MoveOrder) [][]cell.ID {
	if mt.Queued == 0 {
		delete(r.previews, id)
		return nil
	}
	var goals [MaxWaypoints + 1]cell.ID
	var levels [MaxWaypoints + 1]Level
	goals[0], levels[0] = mt.Target, mt.Level
	for k, g := range mt.Waypoints[:mt.Queued] {
		goals[k+1], levels[k+1] = g.Cell, g.Level
	}
	if pv := r.previews[id]; pv != nil && pv.queued == mt.Queued && pv.goals == goals {
		return pv.routes
	}
	pv := &preview{goals: goals, queued: mt.Queued}
	for k := 0; k < int(mt.Queued); k++ {
		from, to := goals[k], goals[k+1]
		route := []cell.ID{from}
		if r.finder != nil {
			if path, ok := r.finder.findPath(id, domain, body, from, to, levels[k+1]); ok {
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
