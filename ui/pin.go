package ui

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/uid"
)

// OffScreen is what an element pinned to an entity does while the entity is out of every picture
// of the world on the screen.
type OffScreen int

const (
	// PointAtIt stands at the edge of the picture on the entity's side, an arrow towards it.
	PointAtIt OffScreen = iota
	// ShowIt stands in the middle of the picture with a button moving the camera onto the entity.
	ShowIt
	// GoToIt moves the camera onto the entity as it appears, and stands by it.
	GoToIt
)

// placing is where a pinned element stands by its entity's point on the screen.
type placing int

const (
	above placing = iota
	below
	beside
)

// pin says which entities an element is shown for, once each, and where it stands by each.
type pin struct {
	under     *effect.Effect
	on        entity.Whom
	placing   placing
	dx, dy    float64
	off       OffScreen
	instances []instance
	gone      map[uid.UID64]bool // GoToIt: the entities the camera was moved to
	show      *Element           // ShowIt: the button moving the camera, laid under the element
}

// instance is the element shown for one entity this frame.
type instance struct {
	id     uid.UID64
	box    geom.AABB  // where the element stands; zero: where its parent lays it
	arrow  []geom.Vec // PointAtIt: the arrow's corners
	show   bool       // ShowIt: the button is shown
	looker Looker     // who moves the camera onto it
}

// Looker is an Input that can move its camera onto an entity (players.Plugin.Through).
type Looker interface{ LookAt(entity uid.UID64) }

func (e *Element) pinned() *pin {
	if e.pin == nil {
		e.pin = &pin{gone: map[uid.UID64]bool{}}
	}
	return e.pin
}

// Under shows the element for every entity e is on, pinned to it: by it in the picture of the world
// that shows it, or where its parent lays it for one with no place in the world (the world's own
// entity, a plugin's).
func (e *Element) Under(ef effect.Effect) *Element {
	e.pinned().under = &ef
	return e
}

// On shows the element for every entity whom names, pinned to it as Under does.
func (e *Element) On(whom entity.Whom) *Element {
	e.pinned().on = whom
	return e
}

// Above stands the pinned element above its entity: the default.
func (e *Element) Above() *Element {
	e.pinned().placing = above
	return e
}

// Below stands the pinned element below its entity.
func (e *Element) Below() *Element {
	e.pinned().placing = below
	return e
}

// Beside stands the pinned element to the right of its entity.
func (e *Element) Beside() *Element {
	e.pinned().placing = beside
	return e
}

// Offset moves the pinned element dx, dy pixels from where it stands by its entity.
func (e *Element) Offset(dx, dy float64) *Element {
	p := e.pinned()
	p.dx, p.dy = dx, dy
	return e
}

// OffScreen says what the pinned element does while its entity is out of sight: PointAtIt unless
// said.
func (e *Element) OffScreen(how OffScreen) *Element {
	p := e.pinned()
	p.off = how
	if how == ShowIt && p.show == nil {
		p.show = Button("Show")
	}
	return e
}

// spot is an entity a pin is shown for, and its point in the world, if it has one.
type spot struct {
	id     uid.UID64
	placed bool
	x, y   float32
	z      float32
}

// locator is a picture of the world that says where a world point lies in it (render.Feed).
type locator interface {
	ToPixels(x, y, z float32) (px, py, depth float32, visible bool)
}

// view is a picture of the world on the screen: where it lies, and who takes its input.
type view struct {
	box   geom.AABB
	at    locator
	input Input
}

// views are the scene's pictures of the world shown, in the order drawn.
func views(root *Element) []view {
	var out []view
	var visit func(e *Element)
	visit = func(e *Element) {
		if e.hidden {
			return
		}
		if p, ok := e.content.(*picture); ok {
			if l, ok := p.src.(locator); ok {
				out = append(out, view{box: shrink(e.box, e.padding), at: l, input: p.input})
			}
		}
		for _, c := range e.children {
			visit(c)
		}
	}
	visit(root)
	return out
}

// stand works out where the element stands for each of spots this frame.
func (p *pin) stand(e *Element, spots []spot, vs []view, screen geom.AABB) {
	p.instances = p.instances[:0]
	for _, s := range spots {
		in := instance{id: s.id}
		if s.placed && len(vs) > 0 {
			p.place(e, s, vs, screen, &in)
		}
		p.instances = append(p.instances, in)
		if e.holds {
			break // a modal element waits its turn: one at a time
		}
	}
}

// place stands in by s in the first view that shows it, else as OffScreen says in the first view.
func (p *pin) place(e *Element, s spot, vs []view, screen geom.AABB, in *instance) {
	w, h := e.needs()
	for _, v := range vs {
		if px, py, _, ok := v.at.ToPixels(s.x, s.y, s.z); ok {
			in.box = clamp(p.by(v.box.TopLeft.Add(geom.NewVec(float64(px), float64(py))), w, h), screen)
			return
		}
	}
	v := vs[0]
	in.looker, _ = v.input.(Looker)
	px, py, _, _ := v.at.ToPixels(s.x, s.y, s.z)
	target := v.box.TopLeft.Add(geom.NewVec(float64(px), float64(py)))
	vw, vh := size(v.box)
	middle := v.box.TopLeft.Add(geom.NewVec(vw/2, vh/2))
	switch p.off {
	case PointAtIt:
		edge := geom.NewVec(max(vw/2-w/2-arrowLength, 0), max(vh/2-h/2-arrowLength, 0))
		at := towards(middle, target, edge)
		in.box = clamp(geom.NewAABBAt(at.Sub(geom.NewVec(w/2, h/2)), w, h), screen)
		in.arrow = arrow(in.box, target)
	case ShowIt:
		in.box = geom.NewAABBAt(middle.Sub(geom.NewVec(w/2, h/2)), w, h)
		in.show = in.looker != nil
	case GoToIt:
		in.box = geom.NewAABBAt(middle.Sub(geom.NewVec(w/2, h/2)), w, h)
		if in.looker != nil && !p.gone[s.id] {
			p.gone[s.id] = true
			in.looker.LookAt(s.id)
		}
	}
}

// by is the box of a w by h element standing by the screen point a as the pin says.
func (p *pin) by(a geom.Vec, w, h float64) geom.AABB {
	var tl geom.Vec
	switch p.placing {
	case below:
		tl = geom.NewVec(a.X-w/2, a.Y)
	case beside:
		tl = geom.NewVec(a.X, a.Y-h/2)
	default:
		tl = geom.NewVec(a.X-w/2, a.Y-h)
	}
	return geom.NewAABBAt(tl.Add(geom.NewVec(p.dx, p.dy)), w, h)
}

// point is where on its entity a pinned element stands by: the top of its box for Above, the
// bottom for Below, the right side for Beside — in a world with heights its top, its foot or its
// middle over its centre.
func (p *pin) point(b entity.Base, z *entity.Z) (x, y, alt float32) {
	box := b.Pos.AABB
	c := b.Pos.Center()
	if z != nil && z.Height > 0 {
		switch p.placing {
		case below:
			return float32(c.X), float32(c.Y), float32(z.Altitude)
		case beside:
			return float32(c.X), float32(c.Y), float32(z.Altitude + z.Height/2)
		default:
			return float32(c.X), float32(c.Y), float32(z.Top())
		}
	}
	switch p.placing {
	case below:
		return float32(c.X), float32(box.BottomRight.Y), 0
	case beside:
		return float32(box.BottomRight.X), float32(c.Y), 0
	default:
		return float32(c.X), float32(box.TopLeft.Y), 0
	}
}

const arrowLength = 14

// towards is the point from middle towards target, no further than reach along either axis.
func towards(middle, target, reach geom.Vec) geom.Vec {
	d := target.Sub(middle)
	scale := 1.0
	if d.X != 0 {
		scale = min(scale, reach.X/math.Abs(d.X))
	}
	if d.Y != 0 {
		scale = min(scale, reach.Y/math.Abs(d.Y))
	}
	return middle.Add(geom.NewVec(d.X*scale, d.Y*scale))
}

// arrow is a triangle on box's edge pointing towards target.
func arrow(box geom.AABB, target geom.Vec) []geom.Vec {
	w, h := size(box)
	c := box.TopLeft.Add(geom.NewVec(w/2, h/2))
	d := target.Sub(c)
	n := math.Hypot(d.X, d.Y)
	if n == 0 {
		return nil
	}
	ux, uy := d.X/n, d.Y/n
	reach := math.Min(w/2/math.Max(math.Abs(ux), 1e-9), h/2/math.Max(math.Abs(uy), 1e-9))
	base := c.Add(geom.NewVec(ux*reach, uy*reach))
	tip := base.Add(geom.NewVec(ux*arrowLength, uy*arrowLength))
	side := geom.NewVec(-uy*arrowLength/2, ux*arrowLength/2)
	return []geom.Vec{tip, base.Add(side), base.Sub(side)}
}

// clamp moves box inside screen where it fits.
func clamp(box geom.AABB, screen geom.AABB) geom.AABB {
	w, h := size(box)
	x := math.Min(math.Max(box.TopLeft.X, screen.TopLeft.X), screen.BottomRight.X-w)
	y := math.Min(math.Max(box.TopLeft.Y, screen.TopLeft.Y), screen.BottomRight.Y-h)
	return geom.NewAABBAt(geom.NewVec(math.Max(x, screen.TopLeft.X), math.Max(y, screen.TopLeft.Y)), w, h)
}

// each lays the element for every instance in turn — where its parent laid it for one with no
// place — and calls fn.
func (p *pin) each(e *Element, parent geom.AABB, fn func(in *instance)) {
	defer func() { e.parent = parent }()
	for k := range p.instances {
		in := &p.instances[k]
		if in.box == (geom.AABB{}) {
			e.lay(parent)
		} else {
			e.lay(in.box)
		}
		if in.show {
			bw, bh := p.show.needs()
			p.show.lay(geom.NewAABBAt(geom.NewVec(e.box.TopLeft.X, e.box.BottomRight.Y+4), bw, bh))
		}
		fn(in)
	}
}

// paintArrow draws in's arrow.
func paintArrow(dst *render.Image, in *instance) {
	if len(in.arrow) != 3 {
		return
	}
	pts := make([][2]float32, 3)
	for k, v := range in.arrow {
		pts[k] = [2]float32{float32(v.X), float32(v.Y)}
	}
	render.FillPolygon(dst, pts, panelBorder)
}
