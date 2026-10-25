package ui

import (
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/render"
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

type placing int

const (
	above placing = iota
	below
	beside
)

type pin struct {
	under     *effect.Effect
	on        ceremony.Whom
	where     Pin
	placing   placing
	dx, dy    float32
	off       OffScreen
	instances []instance
	gone      map[uid.UID64]bool
	show      *Element
}

type instance struct {
	id     uid.UID64
	box    geom.AABB
	arrow  []geom.Vec
	show   bool
	looker Looker
}

// Looker is an Input that can move its camera onto an entity.
type Looker interface{ LookAt(entity uid.UID64) }

func (e *Element) pinned() *pin {
	if e.pin == nil {
		e.pin = &pin{gone: map[uid.UID64]bool{}}
	}
	return e.pin
}

// Under shows the element for every entity ef is on, pinned to it. One with no place in the
// world has it where its parent lays it.
func (e *Element) Under(ef effect.Effect) *Element {
	e.pinned().under = &ef
	return e
}

// On shows the element for every entity whom names, pinned to it as Under does.
func (e *Element) On(whom ceremony.Whom) *Element {
	e.pinned().on = whom
	return e
}

// Where shows the element for every entity p holds, pinned to it as Under does.
func (e *Element) Where(p Pin) *Element {
	e.pinned().where = p
	return e
}

// Pin is the entities an element is shown for: Tagged.
type Pin interface {
	bind(si *goke.SysInit)
	each(fn func(id uid.UID64, base *entity.Base, z *entity.Z, label *ceremony.Label))
}

// Tagged is the entities carrying t, a tag of any family.
func Tagged[F any](t tag.Tag[F]) Pin { return &tagged[F]{t: t} }

type tagged[F any] struct {
	t     tag.Tag[F]
	query *goke.Query
	tags  goke.Comp[tag.Tags[F]]
	base  goke.OptComp[entity.Base]
	z     goke.OptComp[entity.Z]
	label goke.OptComp[ceremony.Label]
}

func (g *tagged[F]) bind(si *goke.SysInit) {
	g.query = si.NewQueryBuilder(&g.tags).Optional(&g.base).Optional(&g.z).Optional(&g.label).Build()
}

func (g *tagged[F]) each(fn func(id uid.UID64, base *entity.Base, z *entity.Z, label *ceremony.Label)) {
	for g.query.All(); g.query.Next(); {
		cur := g.query.Cursor()
		tags, bases, zs, labels := g.tags.Slice(cur), g.base.Slice(cur), g.z.Slice(cur), g.label.Slice(cur)
		for i, id := range cur.IDs {
			if tags[i].Has(g.t) {
				fn(id, at(bases, i), at(zs, i), at(labels, i))
			}
		}
	}
}

func at[T any](column []T, i int) *T {
	if column == nil {
		return nil
	}
	return &column[i]
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
func (e *Element) Offset(dx, dy float32) *Element {
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

type spot struct {
	id     uid.UID64
	placed bool
	x, y   float32
	z      float32
}

type locator interface {
	ToPixels(x, y, z float32) (px, py, depth float32, visible bool)
}

type view struct {
	pic   *Element
	box   geom.AABB
	at    locator
	input Input
}

func views(root *Element) []view {
	var out []view
	var visit func(e *Element)
	visit = func(e *Element) {
		if e.shut() {
			return
		}
		if p, ok := e.content.(*picture); ok {
			if l, ok := p.src.(locator); ok {
				out = append(out, view{pic: e, box: shrink(e.box, e.padding), at: l,
					input: p.input})
			}
		}
		for _, c := range e.children {
			visit(c)
		}
	}
	visit(root)
	return out
}

// Owner is an Input that says which entities its picture shows elements pinned to. A picture
// without one shows them all.
type Owner interface{ Shows(entity uid.UID64) bool }

func (v view) shows(id uid.UID64) bool {
	o, ok := v.input.(Owner)
	if !ok {
		return true
	}
	if f := v.pic.front(); f != nil {
		if shown, ok := f.shows[showing{v.pic, id}]; ok {
			return shown
		}
	}
	return o.Shows(id)
}

func (p *pin) stand(e *Element, spots []spot, vs []view, screen geom.AABB) {
	p.instances = p.instances[:0]
	for _, s := range spots {
		if !s.placed || len(vs) == 0 {
			p.instances = append(p.instances, instance{id: s.id})
		} else {
			p.place(e, s, vs, screen)
		}
		if e.holds && len(p.instances) > 0 {
			p.instances = p.instances[:1] // a modal element: one at a time
			break
		}
	}
}

func (p *pin) place(e *Element, s spot, vs []view, screen geom.AABB) {
	e.drawnFor(s.id, true)
	w, h := e.needs()
	var first *view
	seen := false
	for k := range vs {
		v := &vs[k]
		if !v.shows(s.id) {
			continue
		}
		if first == nil {
			first = v
		}
		if px, py, _, ok := v.at.ToPixels(s.x, s.y, s.z); ok {
			box := clamp(p.by(v.box.TopLeft.Add(geom.NewVec(px, py)), w, h), screen)
			p.instances = append(p.instances, instance{id: s.id, box: box})
			seen = true
		}
	}
	if seen || first == nil {
		return
	}
	in := instance{id: s.id}
	p.offScreen(e, *first, s, w, h, screen, &in)
	p.instances = append(p.instances, in)
}

func (p *pin) offScreen(e *Element, v view, s spot, w, h float32, screen geom.AABB,
	in *instance) {
	in.looker, _ = v.input.(Looker)
	px, py, _, _ := v.at.ToPixels(s.x, s.y, s.z)
	target := v.box.TopLeft.Add(geom.NewVec(px, py))
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
			e.lookAt(in.looker, s.id)
		}
	}
}

// lookAt has looker move its camera onto id: at its scene's next Publish once one was, now before
func (e *Element) lookAt(looker Looker, id uid.UID64) {
	if e.front() == nil {
		looker.LookAt(id)
		return
	}
	e.d.wished = append(e.d.wished, look{looker, id})
}

func (p *pin) by(a geom.Vec, w, h float32) geom.AABB {
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

func (p *pin) point(b entity.Base, z *entity.Z) (x, y, alt float32) {
	box := b.Pos.AABB
	c := b.Pos.Center()
	if z != nil && z.Height > 0 {
		switch p.placing {
		case below:
			return c.X, c.Y, z.Altitude
		case beside:
			return c.X, c.Y, z.Altitude + z.Height/2
		default:
			return c.X, c.Y, z.Top()
		}
	}
	switch p.placing {
	case below:
		return c.X, box.BottomRight.Y, 0
	case beside:
		return box.BottomRight.X, c.Y, 0
	default:
		return c.X, box.TopLeft.Y, 0
	}
}

const arrowLength = 14

func towards(middle, target, reach geom.Vec) geom.Vec {
	d := target.Sub(middle)
	scale := float32(1)
	if d.X != 0 {
		scale = min(scale, reach.X/f32.Abs(d.X))
	}
	if d.Y != 0 {
		scale = min(scale, reach.Y/f32.Abs(d.Y))
	}
	return middle.Add(geom.NewVec(d.X*scale, d.Y*scale))
}

func arrow(box geom.AABB, target geom.Vec) []geom.Vec {
	w, h := size(box)
	c := box.TopLeft.Add(geom.NewVec(w/2, h/2))
	d := target.Sub(c)
	n := f32.Hypot(d.X, d.Y)
	if n == 0 {
		return nil
	}
	ux, uy := d.X/n, d.Y/n
	reach := min(w/2/max(f32.Abs(ux), 1e-9), h/2/max(f32.Abs(uy), 1e-9))
	base := c.Add(geom.NewVec(ux*reach, uy*reach))
	tip := base.Add(geom.NewVec(ux*arrowLength, uy*arrowLength))
	side := geom.NewVec(-uy*arrowLength/2, ux*arrowLength/2)
	return []geom.Vec{tip, base.Add(side), base.Sub(side)}
}

func clamp(box geom.AABB, screen geom.AABB) geom.AABB {
	w, h := size(box)
	x := min(max(box.TopLeft.X, screen.TopLeft.X), screen.BottomRight.X-w)
	y := min(max(box.TopLeft.Y, screen.TopLeft.Y), screen.BottomRight.Y-h)
	return geom.NewAABBAt(geom.NewVec(max(x, screen.TopLeft.X), max(y, screen.TopLeft.Y)), w, h)
}

func (p *pin) each(e *Element, parent geom.AABB, fn func(in *instance)) {
	defer func() { e.parent = parent }()
	for k := range p.instances {
		in := &p.instances[k]
		e.drawnFor(in.id, true)
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

func paintArrow(dst *render.Image, in *instance, color color.RGBA) {
	if len(in.arrow) != 3 {
		return
	}
	pts := make([][2]float32, 3)
	for k, v := range in.arrow {
		pts[k] = [2]float32{v.X, v.Y}
	}
	dst.FillPolygon(pts, color)
}
