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

// collisionSystem runs one tick of collisions: what each entity struck last tick, who really
// overlaps now, the bounce, the push apart, and the contacts left behind for rules.
type collisionSystem struct {
	space  *aabbworld.Space
	engine collide.Engine
	tickOf plugin.TickSource // the world's, for the rules

	// walk is the pass over every Collider: its rules and its capabilities.
	walk     *goke.Query
	base     goke.Comp[world.Base]
	collider goke.Comp[Collider]
	physics  goke.OptComp[Physics]
	each     *plugin.Rules[Struck]
	walking  struct {
		ids       []uid.UID64
		colliders []Collider
	}
	struckAt func(i int) Struck
	hitAt    func(i int) bool

	// all is every entity of the world, for rebuilding the space and retiring lost Colliders.
	all     *goke.Query
	allBase goke.Comp[world.Base]
	items   []aabbworld.Item

	// lookup resolves a side of a contact from its id.
	lookup         *goke.Query
	lookupBase     goke.Comp[world.Base]
	lookupCollider goke.Comp[Collider]
	lookupPhysics  goke.OptComp[Physics]
	lookupLayers   goke.OptComp[world.Layers]
	lookupHot      bool

	// pair is the contact being settled; contacts is what this tick confirmed.
	pair     pairSides
	contacts []pairSides
	between  *plugin.PairRules[Meeting]
	outside  goke.CompID // world.Outside, for whoever the solver pushes out by an open edge

	// fieldOf resolves the solid ground when the engine is built; ground is the side it shows a
	// contact, immovable and still.
	fieldOf func() Field
	stats   *ContactStats // counted into, nil for none
	log     *log.Logger   // a line per contact, nil for none
	field   solidField
	ground  struct {
		base    world.Base
		physics Physics
	}

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
	_, _, ok := (*collisionSystem)(h).side(id)
	return pen, ok
}
func (h *handler) ContactField(id uid.UID64, cell uint64, pen geom.Vec) {
	(*collisionSystem)(h).contactGround(id, cell, pen)
}

// solidField is the Field as the engine asks for it: for an entity, on its Layers.
type solidField struct {
	field Field
	d     *collisionSystem
}

func (f *solidField) Solid(id uid.UID64, box geom.AABB, visit func(collide.FieldBox) bool) {
	if f.d.seek(id) {
		f.field.Solid(world.LayersOf(f.d.lookupLayers.At(f.d.lookup.Cursor())), box, visit)
	}
}

// sought is the one query the system offers its hosted rules.
const sought = 0

// newCollisionSystem builds the collision system over space, its rules hosted by between and each.
func newCollisionSystem(space *aabbworld.Space, between *plugin.PairRules[Meeting], each *plugin.Rules[Struck], fieldOf func() Field) *collisionSystem {
	d := &collisionSystem{space: space, between: between, each: each, fieldOf: fieldOf}
	d.struckAt, d.hitAt = d.struck, d.hit
	d.ground.physics = Physics{Mass: math.Inf(1)}
	return d
}

// Init builds the engine too, once the board has given collision its solid ground.
func (d *collisionSystem) Init(si *goke.SysInit) {
	cfg := collide.Config{Reach: world.StepReach, Iterations: solverIterations}
	if d.fieldOf != nil {
		if f := d.fieldOf(); f != nil {
			d.field = solidField{field: f, d: d}
			cfg.Field = &d.field
		}
	}
	d.engine = d.space.CollideEngine((*handler)(d), cfg)
	d.outside = si.RegComp[world.Outside]()
	qb := si.NewQueryBuilder(&d.base, &d.collider).Optional(&d.physics)
	d.each.Bind(qb)
	d.walk = qb.Build()

	d.all = si.NewQueryBuilder(&d.allBase).Build()

	seek := si.NewQueryBuilder(&d.lookupBase, &d.lookupCollider).Optional(&d.lookupPhysics, &d.lookupLayers)
	d.between.Bind(seek)
	d.lookup = seek.Build()
}

func (d *collisionSystem) Update(cb *goke.CmdBuf, dt time.Duration) {
	d.tick = d.tickOf.Of(cb, dt)
	if d.mark() {
		d.rebuild()
	}

	d.contacts = d.contacts[:0]
	d.lookupHot, d.stale = false, false
	d.engine.Tick()
	for _, l := range d.engine.Left() {
		if d.seek(l.ID) {
			d.lookupBase.At(d.lookup.Cursor()).Pos.AABB = l.Box // where the push left it, past the edge
		}
		cb.AddOne(l.ID, d.outside, world.Outside{})
	}
	if d.stale {
		d.rebuild()
	}

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

// mark runs the rules over every Collider and settles its capabilities; true if changed.
func (d *collisionSystem) mark() bool {
	changed := false
	d.walk.All()
	for d.walk.Next() {
		cursor := d.walk.Cursor()
		bases, colliders, physics := d.base.Slice(cursor), d.collider.Slice(cursor), d.physics.Slice(cursor)

		d.walking.ids, d.walking.colliders = cursor.IDs, colliders
		d.each.RunWhere(d.tick, cursor, d.hitAt, d.struckAt)
		for i := range cursor.IDs {
			colliders[i].clearContacts()
			caps := aabbworld.CanCollide
			switch {
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

// rebuild hands the space every entity again, capabilities as they stand now.
func (d *collisionSystem) rebuild() {
	d.items = d.items[:0]
	d.all.All()
	for d.all.Next() {
		cursor := d.all.Cursor()
		for i, b := range d.allBase.Slice(cursor) {
			d.items = append(d.items, aabbworld.Item{ID: cursor.IDs[i], Box: b.Pos.AABB, Caps: b.Caps})
		}
	}
	d.space.Rebuild(d.items)
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
	// holdA and holdB say a side stays where it is: the push would put it over ground that does
	// not take it
	holdA, holdB bool
}

// detectOnly reports a pair in which either side takes no part in the physical world.
func (p *pairSides) detectOnly() bool { return p.A.Physics == nil || p.B.Physics == nil }

type contactSide struct {
	Entity   uid.UID64
	Base     *world.Base
	Collider *Collider
	// Physics is nil for a side that is only ever detected.
	Physics *Physics
	Layers  world.Layers
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
// on no common plane.
func (d *collisionSystem) resolve(a, b uid.UID64, pen geom.Vec) (geom.Vec, bool) {
	sideA, tagsA, ok := d.side(a)
	if !ok {
		return pen, false
	}
	sideB, tagsB, ok := d.side(b)
	if !ok {
		return pen, false
	}
	if !sideA.Layers.Meets(sideB.Layers) {
		return pen, false
	}
	d.pair = pairSides{A: sideA, B: sideB, tagsA: tagsA, tagsB: tagsB}
	return d.footing(pen), true
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

// side looks one entity up, refusing one that no longer carries a Collider.
func (d *collisionSystem) side(id uid.UID64) (contactSide, plugin.Marks, bool) {
	if !d.seek(id) {
		if d.all.Seek(id) {
			d.allBase.At(d.all.Cursor()).Caps, d.stale = aabbworld.Plain, true
		}
		return contactSide{}, plugin.Marks{}, false
	}
	cur := d.lookup.Cursor()
	return contactSide{
		Entity: id, Base: d.lookupBase.At(cur),
		Collider: d.lookupCollider.At(cur), Physics: d.lookupPhysics.At(cur),
		Layers: world.LayersOf(d.lookupLayers.At(cur)),
	}, d.between.At(sought, cur), true
}

func (d *collisionSystem) seek(id uid.UID64) bool {
	ok := d.lookupHot && d.lookup.SeekH(id)
	if !ok {
		ok = d.lookup.Seek(id)
		d.lookupHot = ok
	}
	return ok
}

// contact settles the pair resolve just confirmed: the bounce, and a Contact on each side.
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

	sides.A.Collider.addContact(sides.B.Entity, impact, normal)
	sides.B.Collider.addContact(sides.A.Entity, impact, geom.NewVec(-normal.X, -normal.Y))
	d.contacts = append(d.contacts, sides)
}

// contactGround settles a contact of id with the solid ground: a bounce off something of
// infinite mass, and a Contact on id alone.
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
	self.Collider.add(Contact{Impact: impact, Normal: normal, Terrain: true, Cell: cell})
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
