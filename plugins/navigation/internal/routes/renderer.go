package routes

import (
	"image/color"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/order"
	"github.com/kjkrol/gram/plugins/navigation/internal/pathfind"
	"github.com/kjkrol/gram/plugins/navigation/internal/spacing"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

func pathCells(at unit.At, mt order.MoveOrder) []cell.ID { return appendPath(nil, at, mt) }

// appendPath appends to dst the cells from at to the order's target: the leg's, the path's left
func appendPath(dst []cell.ID, at unit.At, mt order.MoveOrder) []cell.ID {
	cells := append(dst, at.Cell)
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

// Renderer draws the selected units' routes and every unit's goal, as published once a frame.
type Renderer struct {
	grid    grid.Grid
	Style   RouteStyle // the kinds' default
	Current RouteStyle // the unit's being drawn
	styles  func() *render.Styles
	frame   *render.Frame // the one being composed
	camera  camera.Camera // the one of the frame being composed
	space   *aabbworld.Space
	// nil: level at 0; those of the frame drawn, nil: heights
	heights      func() ground.Heights
	drawnHeights func() ground.Heights
	ground       ground.Heights
	step         float32
	// nil: the box as it lies
	look   func() world.Look
	routes bool

	// nil: the goals alone, no routes between them
	Finder   *spacing.Finder
	previews map[uid.UID64]*preview

	selected tag.Tag[selection.Family]

	query *goke.Query
	base  goke.Comp[world.Base]
	cell  goke.Comp[unit.At]
	order goke.Comp[order.MoveOrder]
	marks goke.Comp[tag.Tags[selection.Family]]
	app   goke.OptComp[render.Appearance]
	mover goke.OptComp[unit.Mover]

	footprint []render.Corners
	drawn     render.Buffered[planned]

	gpu   *Lines
	onGPU bool // this frame's
}

var _ render.Direct = (*Renderer)(nil)

// planned is what the Renderer draws: every selected unit and, the routes shown, its way: the
// cells of its path planned.cells[path], its goals, the routes to its queued goals
type planned struct {
	routes bool
	units  []routed
	cells  []cell.ID
	goals  []goalAt
	legs   []leg
}

type routed struct {
	style             RouteStyle
	size, center, dir geom.Vec
	end               geom.Vec // where its path ends
	path, goals, legs [2]int32 // from, to
}

// goalAt is where a goal stands and how high
type goalAt struct {
	at  geom.Vec
	alt float32
}

// leg is the route to a queued goal, from the goal before, its cells a preview's (queued)
type leg struct {
	cells    []cell.ID
	from, to geom.Vec
}

// NewRenderer draws over grid in style the routes of the units marked selected.
func NewRenderer(grid grid.Grid, style RouteStyle, selected tag.Tag[selection.Family]) *Renderer {
	if style == (RouteStyle{}) {
		style = DefaultRouteStyle
	}
	return &Renderer{grid: grid, Style: style, Current: style, selected: selected, gpu: New()}
}

func (r *Renderer) Draw(t render.Target, cam camera.Camera, _ render.Uniforms) {
	if r.onGPU {
		r.gpu.Draw(t, cam)
	}
}

// WithStyles reads the route style of a unit's kind off styles, the world's.
func (r *Renderer) WithStyles(styles func() *render.Styles) *Renderer {
	r.styles = styles
	return r
}

func (r *Renderer) WithHeights(heights func() ground.Heights) *Renderer {
	r.heights = heights
	return r
}

// WithDrawnHeights has the routes drawn over the ground as the frame drawn holds it.
func (r *Renderer) WithDrawnHeights(heights func() ground.Heights) *Renderer {
	r.drawnHeights = heights
	return r
}

func (r *Renderer) WithLook(look func() world.Look) *Renderer {
	r.look = look
	return r
}

func (r *Renderer) ShowRoutes(shown bool) { r.routes = shown }

func (r *Renderer) RoutesShown() bool { return r.routes }

func (r *Renderer) BindSpace(space *aabbworld.Space) { r.space = space }

func (r *Renderer) Init(si *goke.SysInit) {
	r.query = si.NewQueryBuilder(&r.base, &r.cell, &r.order, &r.marks).
		Optional(&r.app).
		Optional(&r.mover).
		Build()
}

// Publish takes every selected unit's goals and, the routes shown, its way, the routes to its
// queued goals found once a change of them, for the frame drawn next.
func (r *Renderer) Publish() {
	b := r.drawn.Back()
	b.routes = r.routes
	b.units, b.cells, b.goals, b.legs = b.units[:0], b.cells[:0], b.goals[:0], b.legs[:0]
	if r.space == nil {
		return
	}
	var g ground.Heights
	if r.heights != nil {
		g = r.heights()
	}
	var styles *render.Styles
	if r.styles != nil {
		styles = r.styles()
	}
	for r.query.All(); r.query.Next(); {
		cursor := r.query.Cursor()
		bases, cells, orders := r.base.Slice(cursor), r.cell.Slice(cursor), r.order.Slice(cursor)
		marks, apps, movers := r.marks.Slice(cursor), r.app.Slice(cursor), r.mover.Slice(cursor)
		for i := range cursor.IDs {
			if !marks[i].Has(r.selected) {
				continue
			}
			u := routed{style: r.Style, size: bases[i].Pos.Size}
			if apps != nil {
				if st, ok := render.StyleOf[RouteStyle](styles, apps[i].SpriteID); ok {
					u.style = st
				}
			}
			o, domain := &orders[i], unit.DomainAt(movers, i)
			u.goals[0] = int32(len(b.goals))
			r.goals(b, g, domain, o)
			u.goals[1], u.path[0], u.legs[0] = int32(len(b.goals)), int32(len(b.cells)),
				int32(len(b.legs))
			if r.routes {
				u.center, u.dir, u.end = bases[i].Pos.Center(), bases[i].Vel.Dir,
					r.point(o.Target, o.Spot)
				b.cells = appendPath(b.cells, cells[i], *o)
				from := u.end
				var weight float32
				if movers != nil {
					weight = movers[i].Weight
				}
				body := bodies.BodyOf(world.Z{}, bases[i].Pos.Size, weight)
				for k, route := range r.queued(cursor.IDs[i], domain, body, o) {
					to := r.point(o.Waypoints[k].Cell, o.Waypoints[k].Spot)
					b.legs = append(b.legs, leg{cells: route, from: from, to: to})
					from = to
				}
			}
			u.path[1], u.legs[1] = int32(len(b.cells)), int32(len(b.legs))
			b.units = append(b.units, u)
		}
	}
}

// Swap has what was published last drawn.
func (r *Renderer) Swap() { r.drawn.Swap() }

func (r *Renderer) Compose(f *render.Frame, cam camera.Camera) {
	r.frame, r.camera = f, cam
	r.gpu.Reset()
	r.ground, r.step = nil, 0
	heights := r.drawnHeights
	if heights == nil {
		heights = r.heights
	}
	if heights != nil {
		if r.ground = heights(); r.ground != nil {
			r.step = r.ground.Step()
		}
	}
	_, rays := cam.(camera.Rays)
	r.onGPU = rays && r.ground != nil && r.space != nil && r.space.Edges == 0
	if r.space == nil {
		return
	}
	d := r.drawn.Front()
	for _, u := range d.units {
		r.Current = u.style
		for _, g := range d.goals[u.goals[0]:u.goals[1]] {
			r.goal(u.size, g.at, g.alt)
		}
		if !d.routes {
			continue
		}
		r.drawPath(u.center, u.dir, d.cells[u.path[0]:u.path[1]], u.end)
		for _, l := range d.legs[u.legs[0]:u.legs[1]] {
			r.drawRoute(l.cells, l.from, l.to)
		}
	}
	if r.onGPU && r.gpu.Len() > 0 {
		f.ReadGround() // the routes find the ground under them
		f.DrawAt(f.OnGround(routeLift))
	}
}

// how far over the ground a route lies among what lies on it: over the sight, under the figures;
// its goal over it
const (
	routeLift = 0.75
	goalLift  = 0.8
)

// a step aside is no goal; each stands on g
func (r *Renderer) goals(b *planned, g ground.Heights, domain cell.Domain, o *order.MoveOrder) {
	if !o.GivingWay {
		at := r.point(o.Target, o.Spot)
		b.goals = append(b.goals, goalAt{at: at, alt: standing(g, at, domain, o.Level)})
	}
	for k := range o.Waypoints[:o.Queued] {
		w := &o.Waypoints[k]
		at := r.point(w.Cell, w.Spot)
		b.goals = append(b.goals, goalAt{at: at, alt: standing(g, at, domain, w.Level)})
	}
}

func standing(g ground.Heights, p geom.Vec, domain cell.Domain, level order.Level) float32 {
	if s, ok := g.(ground.Surfaces); ok && level != order.OnGround {
		top, _ := s.Surface(p, domain, f32.Inf(1), 0)
		return top
	}
	if g == nil {
		return 0
	}
	return g.At(p)
}

func (r *Renderer) point(c cell.ID, spot geom.Vec) geom.Vec {
	if spot != (geom.Vec{}) {
		return spot
	}
	return r.grid.CellCenter(c)
}

func (r *Renderer) drawPath(entityCenter, travel geom.Vec, cells []cell.ID, end geom.Vec) {
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

func (r *Renderer) drawRoute(cells []cell.ID, from, to geom.Vec) {
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
func (r *Renderer) line(a, b geom.Vec) {
	if r.space != nil && r.space.Edges != 0 {
		dx := spacing.ShortestAxisDelta(a.X, b.X, r.space.Width, r.space.Edges.WrapsX())
		dy := spacing.ShortestAxisDelta(a.Y, b.Y, r.space.Height, r.space.Edges.WrapsY())
		x0, y0 := r.camera.ToScreen(a.X, a.Y)
		x1, y1 := r.camera.ToScreen(a.X+dx, a.Y+dy)
		mid := geom.NewVec(a.X+dx/2, a.Y+dy/2)
		at := r.frame.Lifted(mid.X, mid.Y, 0, routeLift)
		r.frame.Line(at, x0, y0, x1, y1, r.Current.Width, r.Current.Line)
		return
	}
	if r.onGPU {
		r.gpu.Add(r.camera, a, b, r.groundAt, r.step, r.Current.Width, r.Current.Line)
		return
	}
	length := f32.Hypot(b.X-a.X, b.Y-a.Y)
	pieces := 1
	if r.step > 0 && length > r.step {
		pieces = int(f32.Ceil(length / r.step))
	}
	x0, y0 := r.on(a)
	for i := 1; i <= pieces; i++ {
		t := float32(i) / float32(pieces)
		p := geom.NewVec(a.X+(b.X-a.X)*t, a.Y+(b.Y-a.Y)*t)
		x1, y1 := r.on(p)
		mid := geom.NewVec(a.X+(b.X-a.X)*(t-0.5/float32(pieces)), a.Y+(b.Y-a.Y)*(t-0.5/float32(pieces)))
		at := r.frame.Lifted(mid.X, mid.Y, r.groundAt(mid), routeLift)
		r.frame.Line(at, x0, y0, x1, y1, r.Current.Width, r.Current.Line)
		x0, y0 = x1, y1
	}
}

func (r *Renderer) groundAt(p geom.Vec) float32 {
	if r.ground == nil {
		return 0
	}
	return r.ground.At(p)
}

func (r *Renderer) on(p geom.Vec) (float32, float32) {
	return r.camera.Project(p.X, p.Y, r.groundAt(p))
}

func (r *Renderer) goal(size, at geom.Vec, alt float32) {
	box := geom.NewAABBAt(geom.NewVec(at.X-size.X/2, at.Y-size.Y/2), size.X, size.Y)
	r.footprint = r.footprint[:0]
	if r.look != nil {
		r.footprint = r.look().Footprint(r.camera, box, alt, r.footprint)
	} else {
		var q render.Corners
		for i, p := range [4][2]float32{{box.TopLeft.X, box.TopLeft.Y}, {box.BottomRight.X, box.TopLeft.Y}, {box.TopLeft.X, box.BottomRight.Y}, {box.BottomRight.X, box.BottomRight.Y}} {
			q[i][0], q[i][1] = r.camera.Project(p[0], p[1], alt)
		}
		r.footprint = append(r.footprint, q)
	}
	lies := r.frame.Lifted(at.X, at.Y, alt, goalLift)
	for _, q := range r.footprint {
		pts := [4][2]float32{q[0], q[1], q[3], q[2]}
		for i, p := range pts {
			n := pts[(i+1)%len(pts)]
			r.frame.Line(lies, p[0], p[1], n[0], n[1], goalWidth, r.Current.Goal)
		}
	}
}

type preview struct {
	goals  [order.MaxWaypoints + 1]cell.ID
	queued uint8
	routes [][]cell.ID
}

// planned once per change of the goals; a goal no route reaches is drawn on its own
func (r *Renderer) queued(id uid.UID64, domain cell.Domain, body pathfind.Body,
	mt *order.MoveOrder) [][]cell.ID {
	if mt.Queued == 0 {
		delete(r.previews, id)
		return nil
	}
	var goals [order.MaxWaypoints + 1]cell.ID
	var levels [order.MaxWaypoints + 1]order.Level
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
		if r.Finder != nil {
			if path, ok := r.Finder.FindPath(id, domain, body, from, to, levels[k+1]); ok {
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
	ex := spacing.ShortestAxisDelta(cellCenter.X, entityCenter.X, width, edges.WrapsX())
	ey := spacing.ShortestAxisDelta(cellCenter.Y, entityCenter.Y, height, edges.WrapsY())
	return ex*travel.X+ey*travel.Y > 0
}
