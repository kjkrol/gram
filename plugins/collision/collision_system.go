package collision

import (
	"log"
	"math"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/collide"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/collision/internal/response"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*collisionSystem)(nil)
var _ collide.Handler = (*handler)(nil)
var _ collide.FieldHandler = (*handler)(nil)

// solverIterations caps the passes one tick spends separating chained overlaps.
const solverIterations = 16

// sweptPen is how deep a swept entity is said to lie in what it struck: enough for the normal,
// never a push.
const sweptPen = 1e-3

// collisionSystem runs one tick of collisions: what each entity struck last tick, who really
// overlaps now, the bounce, the push apart, and the contacts left behind for rules.
type collisionSystem struct {
	space  *aabbworld.Space
	engine collide.Engine
	tickOf plugin.TickSource // the world's, for the rules

	// walk is the pass over every Collider: its rules and its capabilities.
	walk      *goke.Query
	base      goke.Comp[world.Base]
	collider  goke.Comp[Collider]
	physics   goke.OptComp[Physics]
	walkSweep goke.OptComp[Sweep]
	each      *plugin.Rules[Struck]
	walking   struct {
		ids       []uid.UID64
		colliders []Collider
	}
	struckAt func(i int) Struck
	hitAt    func(i int) bool

	// all is every entity of the world, for rebuilding the space and retiring lost Colliders.
	all      *goke.Query
	allBase  goke.Comp[world.Base]
	allSweep goke.OptComp[Sweep]
	items    []aabbworld.Item

	// lookup resolves a side of a contact from its id.
	lookup         *goke.Query
	lookupBase     goke.Comp[world.Base]
	lookupCollider goke.Comp[Collider]
	lookupPhysics  goke.OptComp[Physics]
	lookupLayers   goke.OptComp[world.Layers]
	lookupZ        goke.OptComp[world.Z]
	lookupSweep    goke.OptComp[Sweep]
	lookupHot      bool

	// pair is the contact being settled; pending is every contact this tick confirmed, in
	// order, pruned for the swept to their nearest; contacts the pairs left for the rules.
	pair     pairSides
	pending  []pendingContact
	nearests []nearest
	contacts []pairSides
	between  *plugin.PairRules[Meeting]
	outside  goke.CompID // world.Outside, for whoever the solver pushes out by an open edge

	// fieldOf resolves the solid ground when the engine is built; ground is the side it shows a
	// contact, immovable and still.
	fieldOf func() Field
	heights bool          // the world has heights: a pair meets where its Bands do, the ground too
	stats   *ContactStats // counted into, nil for none
	log     *log.Logger   // a line per contact, nil for none
	field   solidField
	ground  struct {
		base    world.Base
		physics Physics
	}

	// sweeping says an entity is swept this tick, so the space is rebuilt with its stretch and
	// without it again; wraps refuses every Sweep.
	sweeping bool
	wraps    bool

	tick  plugin.Tick
	stale bool
}

// handler is the collisionSystem as the engine talks to it.
type handler collisionSystem

func (h *handler) Touch(a, b uid.UID64, pen geom.Vec) (geom.Vec, bool) {
	return (*collisionSystem)(h).resolve(a, b, pen)
}
func (h *handler) Contact(_, _ uid.UID64, pen geom.Vec) { (*collisionSystem)(h).contact(pen) }
func (h *handler) Moved(id uid.UID64, box plane.AABB)   { (*collisionSystem)(h).moved(id, box) }
func (h *handler) TouchField(id uid.UID64, _ uint64, pen geom.Vec) (geom.Vec, bool) {
	d := (*collisionSystem)(h)
	_, _, ok := d.side(id)
	if ok && d.field.swept {
		pen = geom.NewVec(d.field.normal.X*sweptPen, d.field.normal.Y*sweptPen)
	}
	return pen, ok
}
func (h *handler) ContactField(id uid.UID64, cell uint64, pen geom.Vec) {
	(*collisionSystem)(h).contactGround(id, cell, pen)
}

// solidField is the Field as the engine asks for it: for an entity, on its Layers and, in a world
// with heights, in its Band; for a swept one along its step, each solid box the segment meets
// alone, with where along it (along) and the way out (normal) for the contact.
type solidField struct {
	field Field
	d     *collisionSystem

	// the swept entity's step and box, and the visit asked, while hit walks the solid boxes
	swept          bool
	from, to, half geom.Vec
	along          float64
	normal         geom.Vec
	inner          func(collide.FieldBox) bool
	hit            func(collide.FieldBox) bool // the method value, bound once
}

func (f *solidField) Solid(id uid.UID64, box geom.AABB, visit func(collide.FieldBox) bool) {
	if !f.d.seek(id) {
		return
	}
	cur := f.d.lookup.Cursor()
	layers, band := world.LayersOf(f.d.lookupLayers.At(cur)), f.d.bandAt(cur)
	f.swept, f.along = false, 1
	sw := f.d.lookupSweep.At(cur)
	if sw == nil {
		f.field.Solid(layers, band, box, visit)
		return
	}
	pos := f.d.lookupBase.At(cur).Pos
	f.swept, f.from, f.to, f.half, f.inner = true, sw.From, pos.Center(), halfOf(pos), visit
	f.field.Solid(layers, band, box, f.hit)
	f.inner = nil
}

// segment is one solid box as a swept entity's step meets it: skipped where the segment misses
// it, else visited with where along the step and the way out noted for the contact.
func (f *solidField) segment(fb collide.FieldBox) bool {
	along, normal, ok := response.Sweep(f.from, f.to, f.half, fb.Box)
	if !ok {
		return true
	}
	f.along, f.normal = along, normal
	return f.inner(fb)
}

// bandAt is the band the entity under the lookup's cursor spans: Everywhere in a flat world.
func (d *collisionSystem) bandAt(cur *goke.Cursor) Band {
	if !d.heights {
		return Everywhere
	}
	return BandOf(d.lookupZ.At(cur))
}

// halfOf is half the sides of a box.
func halfOf(pos world.Position) geom.Vec { return geom.NewVec(pos.Size.X/2, pos.Size.Y/2) }

// sought is the one query the system offers its hosted rules.
const sought = 0

// newCollisionSystem builds the collision system over space, its rules hosted by between and each.
func newCollisionSystem(space *aabbworld.Space, between *plugin.PairRules[Meeting], each *plugin.Rules[Struck], fieldOf func() Field) *collisionSystem {
	d := &collisionSystem{space: space, between: between, each: each, fieldOf: fieldOf}
	d.struckAt, d.hitAt = d.struck, d.hit
	d.ground.physics = Physics{Mass: math.Inf(1)}
	d.field.d = d
	d.field.hit = d.field.segment
	return d
}

// Init builds the engine too, once the board has given collision its solid ground.
func (d *collisionSystem) Init(si *goke.SysInit) {
	cfg := collide.Config{Reach: world.StepReach, Iterations: solverIterations}
	if d.fieldOf != nil {
		if f := d.fieldOf(); f != nil {
			d.field.field = f
			cfg.Field = &d.field
		}
	}
	d.engine = d.space.CollideEngine((*handler)(d), cfg)
	_, _, edges := d.space.Bounds()
	d.wraps = edges.WrapsX() || edges.WrapsY()
	d.outside = si.RegComp[world.Outside]()
	qb := si.NewQueryBuilder(&d.base, &d.collider).Optional(&d.physics)
	plugin.Own(d.each, &d.walkSweep)
	d.each.Bind(qb)
	d.walk = qb.Build()

	d.all = si.NewQueryBuilder(&d.allBase).Optional(&d.allSweep).Build()

	seek := si.NewQueryBuilder(&d.lookupBase, &d.lookupCollider).Optional(&d.lookupPhysics, &d.lookupLayers, &d.lookupZ, &d.lookupSweep)
	d.between.Bind(seek)
	d.lookup = seek.Build()
}

func (d *collisionSystem) Update(cb *goke.CmdBuf, dt time.Duration) {
	d.tick = d.tickOf.Of(cb, dt)
	d.sweeping = false
	if d.mark() || d.sweeping {
		d.rebuild(d.sweeping)
	}

	d.pending = d.pending[:0]
	d.lookupHot, d.stale = false, false
	d.engine.Tick()
	for _, l := range d.engine.Left() {
		if d.seek(l.ID) {
			d.lookupBase.At(d.lookup.Cursor()).Pos.AABB = l.Box // where the push left it, past the edge
		}
		cb.AddOne(l.ID, d.outside, world.Outside{})
	}
	if d.sweeping {
		d.nearest()
	}
	if d.stale || d.sweeping {
		d.rebuild(false)
	}
	d.record()

	for i := range d.contacts {
		s := &d.contacts[i]
		d.between.DispatchEitherWay(d.tick, s.tagsA, s.tagsB,
			Meeting{Self: s.A.Entity, Other: s.B.Entity, Impact: s.impact, Normal: s.normal},
			Meeting{Self: s.B.Entity, Other: s.A.Entity, Impact: s.impact, Normal: geom.NewVec(-s.normal.X, -s.normal.Y)})
		if d.log != nil {
			d.log.Printf("collision: %v <-> %v (impact %.2f)", s.A.Entity, s.B.Entity, s.impact)
		}
	}
	if d.stats != nil {
		d.stats.Counter += len(d.contacts)
	}
}

// mark runs the rules over every Collider and settles its capabilities — a swept one is a sensor
// whatever its Physics — and notes whether any is swept; true if a capability changed.
func (d *collisionSystem) mark() bool {
	changed := false
	d.walk.All()
	for d.walk.Next() {
		cursor := d.walk.Cursor()
		bases, colliders, physics := d.base.Slice(cursor), d.collider.Slice(cursor), d.physics.Slice(cursor)
		sweeps := d.walkSweep.Slice(cursor)

		d.walking.ids, d.walking.colliders = cursor.IDs, colliders
		d.each.RunWhere(d.tick, cursor, d.hitAt, d.struckAt)
		for i := range cursor.IDs {
			colliders[i].clearContacts()
			caps := aabbworld.CanCollide
			switch {
			case sweeps != nil:
				if d.wraps {
					panic("collision: Sweep in a wrapping world; a swept entity's step must not cross a seam")
				}
				d.sweeping = true
				caps |= aabbworld.Sensor
			case physics == nil:
				caps |= aabbworld.Sensor
			case physics[i].immovable():
				caps |= aabbworld.Static
			}
			if bases[i].Caps != caps {
				bases[i].Caps, changed = caps, true
			}
		}
	}
	return changed
}

// rebuild hands the space every entity again, capabilities as they stand now — a swept one as
// the stretch of its step when stretch says, its own box otherwise.
func (d *collisionSystem) rebuild(stretch bool) {
	d.items = d.items[:0]
	d.all.All()
	for d.all.Next() {
		cursor := d.all.Cursor()
		sweeps := d.allSweep.Slice(cursor)
		for i, b := range d.allBase.Slice(cursor) {
			box := b.Pos.AABB
			if stretch && sweeps != nil {
				box = stretched(sweeps[i].From, b.Pos)
			}
			d.items = append(d.items, aabbworld.Item{ID: cursor.IDs[i], Box: box, Caps: b.Caps})
		}
	}
	d.space.Rebuild(d.items)
}

// stretched is the box covering pos and the same box centred at from: a swept entity's step.
func stretched(from geom.Vec, pos world.Position) plane.AABB {
	half := halfOf(pos)
	lo := geom.NewVec(min(from.X-half.X, pos.TopLeft.X), min(from.Y-half.Y, pos.TopLeft.Y))
	hi := geom.NewVec(max(from.X+half.X, pos.BottomRight.X), max(from.Y+half.Y, pos.BottomRight.Y))
	return plane.NewAABB(lo, hi.X-lo.X, hi.Y-lo.Y)
}

// struck is what the hosted rules are told about the i-th entity of the chunk being walked.
func (d *collisionSystem) struck(i int) Struck {
	return Struck{ID: d.walking.ids[i], Contacts: d.walking.colliders[i].Contacts()}
}

// hit reports whether the i-th entity of the chunk being walked struck anything.
func (d *collisionSystem) hit(i int) bool { return d.walking.colliders[i].StruckCount > 0 }

// pairSides is who the two boxes of a contact belong to, what they carry, and how it went.
type pairSides struct {
	A, B         contactSide
	tagsA, tagsB plugin.Marks

	impact float64
	normal geom.Vec
	// along is where along a swept side's step the contact lies, 1 for none swept
	along float64
	// holdA and holdB say a side stays where it is: the push would put it over ground that does
	// not take it
	holdA, holdB bool
}

// detectOnly reports a pair in which either side takes no part in the physical world.
func (p *pairSides) detectOnly() bool { return p.A.Physics == nil || p.B.Physics == nil }

// swept is the side of the pair that is swept, if one is.
func (p *pairSides) swept() (uid.UID64, bool) {
	switch {
	case p.A.Sweep != nil:
		return p.A.Entity, true
	case p.B.Sweep != nil:
		return p.B.Entity, true
	}
	return 0, false
}

// pendingContact is one contact confirmed this tick, kept until the swept are pruned to their
// nearest: a pair, or an entity's with the ground (Terrain, Cell, the entity A).
type pendingContact struct {
	sides   pairSides
	terrain bool
	cell    uint64
	sensed  bool
	dropped bool
}

// nearest is the nearest contact of one swept entity so far, as the pruning walks the pending.
type nearest struct {
	id    uid.UID64
	at    int
	along float64
}

type contactSide struct {
	Entity   uid.UID64
	Base     *world.Base
	Collider *Collider
	// Physics is nil for a side that is only ever detected, a swept one among them.
	Physics *Physics
	Layers  world.Layers
	Band    Band   // the heights it spans; Everywhere in a flat world
	Sweep   *Sweep // its step, for a swept side; nil otherwise
}

// side is s as the impulse takes it.
func (s contactSide) side() response.Side {
	return response.Side{InvMass: s.Physics.inverseMass(), Bounce: s.Physics.bounce(), Vel: &s.Base.Vel}
}

// body is s as the footing takes it.
func (s contactSide) body() response.Body {
	return response.Body{Layers: s.Layers, Box: s.Base.Pos.AABB.AABB, Immovable: s.Physics.immovable()}
}

// resolve looks both sides of an overlapping pair up; a lost Collider vetoes, and so do two sides
// on no common plane, two whose Bands do not meet in a world with heights, two swept ones, and a
// swept one against what it is told to pass through. A swept side is refined to its step: the
// pair holds only where the segment meets the other's box.
func (d *collisionSystem) resolve(a, b uid.UID64, pen geom.Vec) (geom.Vec, bool) {
	sideA, tagsA, ok := d.side(a)
	if !ok {
		return pen, false
	}
	sideB, tagsB, ok := d.side(b)
	if !ok {
		return pen, false
	}
	if !sideA.Layers.Meets(sideB.Layers) || !sideA.Band.Meets(sideB.Band) {
		return pen, false
	}
	d.pair = pairSides{A: sideA, B: sideB, tagsA: tagsA, tagsB: tagsB, along: 1}
	if sideA.Sweep == nil && sideB.Sweep == nil {
		return d.footing(pen), true
	}
	if sideA.Sweep != nil && sideB.Sweep != nil {
		return pen, false
	}
	mover, other, moverIsA := sideA, sideB, true
	if sideB.Sweep != nil {
		mover, other, moverIsA = sideB, sideA, false
	}
	if mover.Sweep.Ignoring && mover.Sweep.Ignore == other.Entity {
		return pen, false
	}
	along, normal, hit := response.Sweep(mover.Sweep.From, mover.Base.Pos.Center(), halfOf(mover.Base.Pos), other.Base.Pos.AABB.AABB)
	if !hit {
		return pen, false
	}
	d.pair.along = along
	if !moverIsA {
		normal = geom.NewVec(-normal.X, -normal.Y)
	}
	return geom.NewVec(normal.X*sweptPen, normal.Y*sweptPen), true
}

// footing keeps the pair's sides on their ground (response.Footing): a side held bounces as the
// ground does, and moved keeps it where it is.
func (d *collisionSystem) footing(pen geom.Vec) geom.Vec {
	p := &d.pair
	if d.field.field == nil || p.detectOnly() {
		return pen
	}
	pen, p.holdA, p.holdB = response.Footing(d.field.field, p.A.body(), p.B.body(), pen)
	return pen
}

// side looks one entity up, refusing one that no longer carries a Collider; a swept one is only
// ever detected, its Physics put by.
func (d *collisionSystem) side(id uid.UID64) (contactSide, plugin.Marks, bool) {
	if !d.seek(id) {
		if d.all.Seek(id) {
			d.allBase.At(d.all.Cursor()).Caps, d.stale = aabbworld.Plain, true
		}
		return contactSide{}, plugin.Marks{}, false
	}
	cur := d.lookup.Cursor()
	s := contactSide{
		Entity: id, Base: d.lookupBase.At(cur),
		Collider: d.lookupCollider.At(cur), Physics: d.lookupPhysics.At(cur),
		Layers: world.LayersOf(d.lookupLayers.At(cur)), Band: d.bandAt(cur),
	}
	if s.Sweep = d.lookupSweep.At(cur); s.Sweep != nil {
		s.Physics = nil
	}
	return s, d.between.At(sought, cur), true
}

func (d *collisionSystem) seek(id uid.UID64) bool {
	ok := d.lookupHot && d.lookup.SeekH(id)
	if !ok {
		ok = d.lookup.Seek(id)
		d.lookupHot = ok
	}
	return ok
}

// contact settles the pair resolve just confirmed: the bounce, and the contact kept for both sides.
func (d *collisionSystem) contact(pen geom.Vec) {
	sides := d.pair

	normal, aligned := response.Normal(pen)
	var impact float64
	if aligned && !sides.detectOnly() {
		a, b := sides.A, sides.B
		if sides.holdA { // held, it bounces as the ground does
			a.Physics = &d.ground.physics
		}
		if sides.holdB {
			b.Physics = &d.ground.physics
		}
		impact = bounce(a, b, normal)
	}
	sides.impact, sides.normal = impact, normal
	d.pending = append(d.pending, pendingContact{sides: sides, sensed: sides.detectOnly()})
}

// contactGround settles a contact of id with the solid ground: a bounce off something of
// infinite mass, and a contact kept for id alone — for a swept one where along its step it lies.
func (d *collisionSystem) contactGround(id uid.UID64, cell uint64, pen geom.Vec) {
	self, _, ok := d.side(id)
	if !ok {
		return
	}
	ground := contactSide{Base: &d.ground.base, Physics: &d.ground.physics}
	normal, aligned := response.Normal(pen)
	var impact float64
	if aligned && self.Physics != nil {
		impact = bounce(self, ground, normal)
	}
	sides := pairSides{A: self, impact: impact, normal: normal, along: 1}
	if self.Sweep != nil {
		sides.along = d.field.along
	}
	d.pending = append(d.pending, pendingContact{sides: sides, terrain: true, cell: cell, sensed: self.Physics == nil})
}

// nearest prunes the pending contacts of every swept entity to the one nearest along its step.
func (d *collisionSystem) nearest() {
	d.nearests = d.nearests[:0]
	for i := range d.pending {
		p := &d.pending[i]
		id, ok := p.sides.swept()
		if !ok {
			continue
		}
		k := 0
		for ; k < len(d.nearests); k++ {
			if d.nearests[k].id == id {
				break
			}
		}
		if k == len(d.nearests) {
			d.nearests = append(d.nearests, nearest{id: id, at: i, along: p.sides.along})
			continue
		}
		n := &d.nearests[k]
		if p.sides.along < n.along {
			d.pending[n.at].dropped = true
			n.at, n.along = i, p.sides.along
		} else {
			p.dropped = true
		}
	}
}

// record writes the contacts kept to the Colliders of their sides, in the order confirmed, and
// lists the pairs for the rules.
func (d *collisionSystem) record() {
	d.contacts = d.contacts[:0]
	for i := range d.pending {
		p := &d.pending[i]
		if p.dropped {
			continue
		}
		s := &p.sides
		if p.terrain {
			s.A.Collider.add(Contact{Impact: s.impact, Normal: s.normal, Terrain: true, Cell: p.cell, Along: s.along, Sensed: p.sensed})
			continue
		}
		s.A.Collider.addContact(s.B.Entity, s.impact, s.normal, s.along, p.sensed)
		s.B.Collider.addContact(s.A.Entity, s.impact, geom.NewVec(-s.normal.X, -s.normal.Y), s.along, p.sensed)
		d.contacts = append(d.contacts, *s)
	}
}

// moved writes a box the engine pushed back to its entity — unless the tick's pushes, the later
// passes' too, which the handler is not asked about, would leave it further over ground that does
// not take it: it stays, and the space, which has it pushed, is built again after the tick.
func (d *collisionSystem) moved(id uid.UID64, box plane.AABB) {
	if !d.seek(id) {
		return
	}
	cur := d.lookup.Cursor()
	base := d.lookupBase.At(cur)
	if d.field.field != nil && response.Worse(d.field.field, world.LayersOf(d.lookupLayers.At(cur)), base.Pos.AABB.AABB, box.AABB) {
		d.stale = true
		return
	}
	base.Pos.AABB = box
}

// bounce trades the contact's impulse between two physical sides and returns it.
func bounce(a, b contactSide, normal geom.Vec) float64 {
	return response.Exchange(a.side(), b.side(), normal)
}
