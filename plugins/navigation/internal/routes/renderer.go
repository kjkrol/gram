package routes

import (
	"image/color"
	"slices"

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

func pathCells(at unit.At, mt order.MoveOrder, rest []cell.ID) []cell.ID {
	return appendPath(nil, at, mt, rest)
}

// appendPath appends to dst the cells from at to the order's target: the leg's, the path's left,
// then rest, the steps past the path's last cell where it ends short of the target
func appendPath(dst []cell.ID, at unit.At, mt order.MoveOrder, rest []cell.ID) []cell.ID {
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
	if len(rest) > 0 {
		if from, _ := windowEnd(at, &mt); from != cells[len(cells)-1] {
			cells = append(cells, from)
		}
		for _, step := range rest {
			if step != cells[len(cells)-1] {
				cells = append(cells, step)
			}
		}
	}
	if len(rest) > 0 || mt.Path.Length == 0 || mt.Path.Index >= mt.Path.Length {
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
	Finder *spacing.Finder
	// nil: every route searched
	Hierarchies *spacing.Hierarchies
	// nil: every unit planned as its own body
	Classes   *pathfind.Classes
	previews  map[uid.UID64]*preview
	wholes    int       // the previews holding a whole route
	budget    int       // the whole routes left to find this Publish
	publishes uint64    // counted, a preview's seen the last that drew its unit
	steps     []cell.ID // the whole route found last, wholeSteps long

	selected tag.Tag[selection.Family]

	query *goke.Query
	base  goke.Comp[world.Base]
	cell  goke.Comp[unit.At]
	order goke.Comp[order.MoveOrder]
	marks goke.Comp[tag.Tags[selection.Family]]
	app   goke.OptComp[render.Appearance]
	mover goke.OptComp[unit.Mover]
	z     goke.OptComp[world.Z]

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
	off               geom.Vec // how far off its cells' middles it stands
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
		Optional(&r.z).
		Build()
}

// Publish takes every selected unit's goals and, the routes shown, its way, whole past the window
// its order holds, and the routes to its queued goals, found once a change of them, for the frame
// drawn next.
func (r *Renderer) Publish() {
	b := r.drawn.Back()
	b.routes = r.routes
	b.units, b.cells, b.goals, b.legs = b.units[:0], b.cells[:0], b.goals[:0], b.legs[:0]
	if r.space == nil {
		return
	}
	r.publishes++
	r.budget = wholesAPublish
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
		zs := r.z.Slice(cursor)
		for i := range cursor.IDs {
			if !marks[i].Has(r.selected) {
				continue
			}
			size := bases[i].Pos.Size
			u := routed{style: r.Style, size: size,
				off: unit.Offset(r.grid, unit.Footprint(r.grid, size))}
			if apps != nil {
				if st, ok := render.StyleOf[RouteStyle](styles, apps[i].SpriteID); ok {
					u.style = st
				}
			}
			o, domain := &orders[i], unit.DomainAt(movers, i)
			u.goals[0] = int32(len(b.goals))
			r.goals(b, g, domain, o, u.off)
			u.goals[1], u.path[0], u.legs[0] = int32(len(b.goals)), int32(len(b.cells)),
				int32(len(b.legs))
			if r.routes {
				u.center, u.dir, u.end = bases[i].Pos.Center(), bases[i].Vel.Dir,
					r.point(o.Target, o.Spot, u.off)
				m := bodies.Member{ID: cursor.IDs[i], Domain: domain, Pos: bases[i].Pos}
				if zs != nil {
					m.Z = zs[i]
				}
				if movers != nil {
					m.Weight = movers[i].Weight
				}
				if pv := r.previews[m.ID]; pv != nil {
					pv.seen = r.publishes
				}
				b.cells = appendPath(b.cells, cells[i], *o, r.rest(m, cells[i], o))
				from := u.end
				for k, route := range r.queued(m, o) {
					to := r.point(o.Waypoints[k].Cell, o.Waypoints[k].Spot, u.off)
					b.legs = append(b.legs, leg{cells: route, from: from, to: to})
					from = to
				}
			}
			u.path[1], u.legs[1] = int32(len(b.cells)), int32(len(b.legs))
			b.units = append(b.units, u)
		}
	}
	r.sweep()
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
		r.drawPath(u.center, u.dir, d.cells[u.path[0]:u.path[1]], u.end, u.off)
		for _, l := range d.legs[u.legs[0]:u.legs[1]] {
			r.drawRoute(l.cells, l.from, l.to, u.off)
		}
	}
	if r.onGPU && r.gpu.Len() > 0 {
		f.ReadGround() // the routes find the ground under them, hidden by it alone
		f.DrawAt(render.OnTop(routeLevel))
	}
}

// the marks' levels: a route under its goals, both under the selection's outline (1)
const (
	routeLevel = 0.5
	goalLevel  = 0.75
)

// a step aside is no goal; each stands on g, where a unit off its cells' middles by off stands
func (r *Renderer) goals(b *planned, g ground.Heights, domain cell.Domain, o *order.MoveOrder,
	off geom.Vec) {
	if !o.GivingWay {
		at := r.point(o.Target, o.Spot, off)
		b.goals = append(b.goals, goalAt{at: at, alt: standing(g, at, domain, o.Level)})
	}
	for k := range o.Waypoints[:o.Queued] {
		w := &o.Waypoints[k]
		at := r.point(w.Cell, w.Spot, off)
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

// point is where a unit off its cells' middles by off stands on c: at spot where it has one
func (r *Renderer) point(c cell.ID, spot, off geom.Vec) geom.Vec {
	if spot != (geom.Vec{}) {
		return spot
	}
	return r.place(c, off)
}

func (r *Renderer) place(c cell.ID, off geom.Vec) geom.Vec {
	m := r.grid.CellCenter(c)
	return geom.NewVec(m.X+off.X, m.Y+off.Y)
}

func (r *Renderer) drawPath(entityCenter, travel geom.Vec, cells []cell.ID, end, off geom.Vec) {
	from := entityCenter
	for i, c := range cells {
		to := r.place(c, off)
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

func (r *Renderer) drawRoute(cells []cell.ID, from, to, off geom.Vec) {
	if len(cells) < 2 {
		r.line(from, to)
		return
	}
	at := from
	for _, c := range cells[1 : len(cells)-1] {
		next := r.place(c, off)
		r.line(at, next)
		at = next
	}
	r.line(at, to)
}

// on a wrapping world flat, the short way round; else in pieces of the ground's step, so the
// line runs over the relief through any camera
func (r *Renderer) line(a, b geom.Vec) {
	at := render.OnTop(routeLevel)
	if r.space != nil && r.space.Edges != 0 {
		dx := spacing.ShortestAxisDelta(a.X, b.X, r.space.Width, r.space.Edges.WrapsX())
		dy := spacing.ShortestAxisDelta(a.Y, b.Y, r.space.Height, r.space.Edges.WrapsY())
		x0, y0 := r.camera.ToScreen(a.X, a.Y)
		x1, y1 := r.camera.ToScreen(a.X+dx, a.Y+dy)
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
	lies := render.OnTop(goalLevel)
	for _, q := range r.footprint {
		pts := [4][2]float32{q[0], q[1], q[3], q[2]}
		for i, p := range pts {
			n := pts[(i+1)%len(pts)]
			r.frame.Line(lies, p[0], p[1], n[0], n[1], goalWidth, r.Current.Goal)
		}
	}
}

// a selected unit's routes past what its order holds: to its queued goals, and whole, the rest of
// its route from the last cell of its window to its Target, found as key says, none where no
// route was
type preview struct {
	goals  [order.MaxWaypoints + 1]cell.ID
	queued uint8
	routes [][]cell.ID
	whole  bool
	key    restKey
	rest   []cell.ID // the steps past key's from
	seen   uint64    // the Publish that last drew its unit
}

// what a whole route's rest is found anew on a change of
type restKey struct {
	from, to cell.ID
	level    order.Level
	passage  uint64
}

const (
	// the most units drawing their routes whole, the others' rests drawn straight
	maxWholes = 16
	// so many found a Publish at most, those of a selection in need found over the next ones
	wholesAPublish = 1
	// the most steps of a whole route drawn, the rest past them straight
	wholeSteps = 1 << 13
)

// rest is the rest of o's route past its window, from the window's last cell to its Target, as
// the routing plans it for m; nil where it is drawn straight
func (r *Renderer) rest(m bodies.Member, at unit.At, o *order.MoveOrder) []cell.ID {
	pv := r.previews[m.ID]
	from, short := windowEnd(at, o)
	if !short || r.Finder == nil {
		if pv != nil {
			r.unrest(m.ID, pv)
		}
		return nil
	}
	key := restKey{from: from, to: o.Target, level: o.Level, passage: r.passage()}
	switch {
	case pv != nil && pv.whole && pv.key == key:
		return pv.rest
	case (pv == nil || !pv.whole) && r.wholes >= maxWholes:
		return nil
	case r.budget == 0:
		return pv.spliced(key)
	}
	n, found, wait := r.plan(m, from, o)
	if wait {
		return pv.spliced(key)
	}
	r.budget--
	if pv == nil {
		pv = r.entry(m.ID)
	}
	if !pv.whole {
		pv.whole = true
		r.wholes++
	}
	pv.key, pv.rest = key, pv.rest[:0]
	if found {
		pv.rest = append(pv.rest, r.steps[:n]...)
	}
	return pv.rest
}

// windowEnd is the last cell of o's window, the path its order holds, and whether the window ends
// short of its Target; an empty window's is where the unit's step ends, short while a route is
// to be asked
func windowEnd(at unit.At, o *order.MoveOrder) (cell.ID, bool) {
	if p := &o.Path; p.Length > 0 {
		last := p.Steps[p.Length-1]
		return last, last != o.Target
	}
	from := at.Cell
	if o.Leg.Active {
		from = o.Leg.To
	}
	return from, from != o.Target && (o.Ask == order.AskNone || o.Ask == order.AskRoute)
}

// plan writes into r.steps the route from from to o's Target for m as the routing plans it:
// climbed through the hierarchy serving m's class where the route is far, else searched; wait
// while that hierarchy is not ready, a far route searched cell by cell too long for a frame
func (r *Renderer) plan(m bodies.Member, from cell.ID, o *order.MoveOrder) (n int, found,
	wait bool) {
	if r.steps == nil {
		r.steps = make([]cell.ID, wholeSteps)
	}
	body, end := r.planned(m), pathfind.Level(o.Level)
	if hs := r.Hierarchies; hs != nil && hs.Far(from, o.Target) {
		if key, served := hs.Key(m); served {
			h := hs.Take(key)
			if h == nil || hs.Due() {
				return 0, false, true
			}
			n, found = r.Finder.Climb(h, m.ID, m.Domain, body, from, o.Target, end, r.steps)
			if found {
				return n, true, false
			}
		}
	}
	n, found = r.Finder.Find(m.ID, m.Domain, body, from, o.Target, end, r.steps)
	return n, found, false
}

// spliced is pv's rest from key's from on, where pv's leads to key's Target over the passage key
// holds through key's from: drawn while the rest is not yet found anew
func (pv *preview) spliced(key restKey) []cell.ID {
	if pv == nil || !pv.whole || pv.key.to != key.to || pv.key.level != key.level ||
		pv.key.passage != key.passage {
		return nil
	}
	if k := slices.Index(pv.rest, key.from); k >= 0 {
		return pv.rest[k+1:]
	}
	return nil
}

// planned is m's body as the routing plans its routes: its class's
func (r *Renderer) planned(m bodies.Member) pathfind.Body {
	if r.Classes == nil {
		return m.Body()
	}
	return r.Classes.Planned(m.Domain, m.Body())
}

// passage is the board's, 0 where the terrain counts no changes
func (r *Renderer) passage() uint64 {
	if p, ok := r.Finder.Terrain.(interface{ Passage() uint64 }); ok {
		return p.Passage()
	}
	return 0
}

func (r *Renderer) entry(id uid.UID64) *preview {
	if r.previews == nil {
		r.previews = map[uid.UID64]*preview{}
	}
	pv := &preview{seen: r.publishes}
	r.previews[id] = pv
	return pv
}

func (r *Renderer) unrest(id uid.UID64, pv *preview) {
	if pv.whole {
		pv.whole, pv.rest = false, pv.rest[:0]
		r.wholes--
	}
	r.tidy(id, pv)
}

func (r *Renderer) tidy(id uid.UID64, pv *preview) {
	if !pv.whole && pv.queued == 0 {
		delete(r.previews, id)
	}
}

// sweep drops the previews of the units this Publish drew none of: unselected, done, or the
// routes hidden
func (r *Renderer) sweep() {
	for id, pv := range r.previews {
		if pv.seen != r.publishes {
			if pv.whole {
				r.wholes--
			}
			delete(r.previews, id)
		}
	}
}

// planned once per change of the goals; a goal no route reaches is drawn on its own
func (r *Renderer) queued(m bodies.Member, mt *order.MoveOrder) [][]cell.ID {
	id := m.ID
	pv := r.previews[id]
	if mt.Queued == 0 {
		if pv != nil {
			pv.queued, pv.routes = 0, nil
			r.tidy(id, pv)
		}
		return nil
	}
	var goals [order.MaxWaypoints + 1]cell.ID
	var levels [order.MaxWaypoints + 1]order.Level
	goals[0], levels[0] = mt.Target, mt.Level
	for k, g := range mt.Waypoints[:mt.Queued] {
		goals[k+1], levels[k+1] = g.Cell, g.Level
	}
	if pv != nil && pv.queued == mt.Queued && pv.goals == goals {
		return pv.routes
	}
	if pv == nil {
		pv = r.entry(id)
	}
	pv.goals, pv.queued, pv.routes = goals, mt.Queued, nil
	body := r.planned(m)
	for k := 0; k < int(mt.Queued); k++ {
		from, to := goals[k], goals[k+1]
		route := []cell.ID{from}
		if r.Finder != nil {
			if path, ok := r.Finder.FindPath(id, m.Domain, body, from, to, levels[k+1]); ok {
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
	return pv.routes
}

func hasPassedCenter(cellCenter, entityCenter, travel geom.Vec, width, height uint32, edges aabbworld.Edges) bool {
	ex := spacing.ShortestAxisDelta(cellCenter.X, entityCenter.X, width, edges.WrapsX())
	ey := spacing.ShortestAxisDelta(cellCenter.Y, entityCenter.Y, height, edges.WrapsY())
	return ex*travel.X+ey*travel.Y > 0
}
