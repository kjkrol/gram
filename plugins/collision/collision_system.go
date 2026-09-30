package collision

import (
	"math"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/collide"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugin/host"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*CollisionSystem)(nil)
var _ collide.Handler = (*handler)(nil)
var _ collide.FieldHandler = (*handler)(nil)

// solverIterations caps the passes one tick spends separating chained overlaps.
const solverIterations = 16

// CollisionSystem runs one tick of collisions: what each entity struck last tick, who really
// overlaps now, the bounce, the push apart, and the contacts left behind for triggers.
type CollisionSystem struct {
	space    *aabbworld.Space
	engine   collide.Engine
	commands *control.Carrier // the world's, for the triggers

	// walk is the pass over every Collider: its triggers and its capabilities.
	walk     *goke.Query
	base     goke.Comp[world.Base]
	collider goke.Comp[Collider]
	physics  goke.OptComp[Physics]
	each     *host.EachHost[Struck]
	walking  struct {
		ids       []uid.UID64
		colliders []Collider
	}
	struckAt func(i int) Struck

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
	between  *host.PairHost[Meeting]
	outside  goke.CompID // world.Outside, for whoever the solver pushes out by an open edge
	shapes   ShapeTest

	// fieldOf resolves the solid ground when the engine is built; ground is the side it shows a
	// contact, immovable and still.
	fieldOf func() Field
	field   solidField
	ground  struct {
		base    world.Base
		physics Physics
	}

	tick  plugin.Tick
	stale bool
}

// handler is the CollisionSystem as the engine talks to it.
type handler CollisionSystem

func (h *handler) Touch(a, b uid.UID64, pen geom.Vec) (geom.Vec, bool) {
	return (*CollisionSystem)(h).resolve(a, b, pen)
}
func (h *handler) Contact(_, _ uid.UID64, pen geom.Vec) { (*CollisionSystem)(h).contact(pen) }
func (h *handler) Moved(id uid.UID64, box plane.AABB)   { (*CollisionSystem)(h).moved(id, box) }
func (h *handler) TouchField(id uid.UID64, _ uint64, pen geom.Vec) (geom.Vec, bool) {
	_, _, ok := (*CollisionSystem)(h).side(id)
	return pen, ok
}
func (h *handler) ContactField(id uid.UID64, cell uint64, pen geom.Vec) {
	(*CollisionSystem)(h).contactGround(id, cell, pen)
}

// solidField is the Field as the engine asks for it: for an entity, on its Layers.
type solidField struct {
	field Field
	d     *CollisionSystem
}

func (f *solidField) Solid(id uid.UID64, box geom.AABB, visit func(collide.FieldBox) bool) {
	if f.d.seek(id) {
		f.field.Solid(world.LayersOf(f.d.lookupLayers.At(f.d.lookup.Cursor())), box, visit)
	}
}

// sought is the one query the system offers its hosted triggers.
const sought = 0

// NewCollisionSystem builds the collision system over space.
func NewCollisionSystem(space *aabbworld.Space) *CollisionSystem {
	return newCollisionSystem(space, &host.PairHost[Meeting]{}, &host.EachHost[Struck]{}, nil, nil)
}

func newCollisionSystem(space *aabbworld.Space, between *host.PairHost[Meeting], each *host.EachHost[Struck], shapes ShapeTest, fieldOf func() Field) *CollisionSystem {
	d := &CollisionSystem{space: space, between: between, each: each, shapes: shapes, fieldOf: fieldOf}
	d.struckAt = d.struck
	d.ground.physics = Physics{Mass: math.Inf(1)}
	return d
}

// Init builds the engine too, once the board has given collision its solid ground.
func (d *CollisionSystem) Init(si *goke.SysInit) {
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

func (d *CollisionSystem) Update(cb *goke.CmdBuf, dt time.Duration) {
	d.tick = plugin.Tick{CmdBuf: cb, Now: time.Now(), Dt: dt, Commands: d.commands}
	if d.mark() {
		d.rebuild()
	}

	d.contacts = d.contacts[:0]
	d.lookupHot, d.stale = false, false
	d.engine.Tick()
	for _, id := range d.engine.Left() {
		cb.AddOne(id, d.outside, world.Outside{})
	}
	if d.stale {
		d.rebuild()
	}

	for i := range d.contacts {
		s := &d.contacts[i]
		d.between.DispatchEitherWay(d.tick, s.tagsA, s.tagsB,
			Meeting{Self: s.A.Entity, Other: s.B.Entity, Impact: s.impact, Normal: s.normal},
			Meeting{Self: s.B.Entity, Other: s.A.Entity, Impact: s.impact, Normal: geom.NewVec(-s.normal.X, -s.normal.Y)})
	}
}

// mark runs the triggers over every Collider and settles its capabilities; true if changed.
func (d *CollisionSystem) mark() bool {
	changed := false
	d.walk.All()
	for d.walk.Next() {
		cursor := d.walk.Cursor()
		bases, colliders, physics := d.base.Slice(cursor), d.collider.Slice(cursor), d.physics.Slice(cursor)

		d.walking.ids, d.walking.colliders = cursor.IDs, colliders
		d.each.Run(d.tick, cursor, d.struckAt)
		for i := range cursor.IDs {
			colliders[i].clearContacts()
			caps := aabbworld.CanCollide
			switch {
			case physics == nil:
				caps |= aabbworld.Sensor
			case physics[i].Immovable():
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
func (d *CollisionSystem) rebuild() {
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

// struck is what the hosted triggers are told about the i-th entity of the chunk being walked.
func (d *CollisionSystem) struck(i int) Struck {
	return Struck{ID: d.walking.ids[i], Contacts: d.walking.colliders[i].Contacts()}
}

// pairSides is who the two boxes of a contact belong to, what they carry, and how it went.
type pairSides struct {
	A, B         contactSide
	tagsA, tagsB plugin.Marks

	impact float64
	normal geom.Vec
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

// resolve looks both sides of an overlapping pair up and asks the shapes; a lost Collider vetoes,
// and so do two sides on no common plane.
func (d *CollisionSystem) resolve(a, b uid.UID64, pen geom.Vec) (geom.Vec, bool) {
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
	if d.shapes == nil {
		return pen, true
	}
	return d.shapes(d.tick, Contactee{ID: a, Base: sideA.Base}, Contactee{ID: b, Base: sideB.Base}, pen)
}

// side looks one entity up, refusing one that no longer carries a Collider.
func (d *CollisionSystem) side(id uid.UID64) (contactSide, plugin.Marks, bool) {
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

func (d *CollisionSystem) seek(id uid.UID64) bool {
	ok := d.lookupHot && d.lookup.SeekH(id)
	if !ok {
		ok = d.lookup.Seek(id)
		d.lookupHot = ok
	}
	return ok
}

// contact settles the pair resolve just confirmed: the bounce, and a Contact on each side.
func (d *CollisionSystem) contact(pen geom.Vec) {
	sides := d.pair

	normal, aligned := normalOf(pen)
	var impact float64
	if aligned && !sides.detectOnly() {
		impact = bounce(sides.A, sides.B, normal)
	}
	sides.impact, sides.normal = impact, normal

	sides.A.Collider.addContact(sides.B.Entity, impact, normal)
	sides.B.Collider.addContact(sides.A.Entity, impact, geom.NewVec(-normal.X, -normal.Y))
	d.contacts = append(d.contacts, sides)
}

// contactGround settles a contact of id with the solid ground: a bounce off something of
// infinite mass, and a Contact on id alone.
func (d *CollisionSystem) contactGround(id uid.UID64, cell uint64, pen geom.Vec) {
	self, _, ok := d.side(id)
	if !ok {
		return
	}
	ground := contactSide{Base: &d.ground.base, Physics: &d.ground.physics}
	normal, aligned := normalOf(pen)
	var impact float64
	if aligned && self.Physics != nil {
		impact = bounce(self, ground, normal)
	}
	self.Collider.add(Contact{Impact: impact, Normal: normal, Terrain: true, Cell: cell})
}

// moved writes a box the engine pushed back to its entity.
func (d *CollisionSystem) moved(id uid.UID64, box plane.AABB) {
	if d.seek(id) {
		d.lookupBase.At(d.lookup.Cursor()).Pos.AABB = box
	}
}

// bounce trades the contact's impulse between two physical sides and returns it.
func bounce(a, b contactSide, normal geom.Vec) float64 {
	deltaA, deltaB := a.Base.Vel.Delta(), b.Base.Vel.Delta()
	impact := impactOf(*a.Physics, *b.Physics, deltaA, deltaB, normal)
	if impact == 0 {
		return 0
	}
	if inv := inverseMass(*a.Physics); inv != 0 {
		a.Base.Vel.SetDelta(geom.NewVec(deltaA.X+impact*inv*normal.X, deltaA.Y+impact*inv*normal.Y))
	}
	if inv := inverseMass(*b.Physics); inv != 0 {
		b.Base.Vel.SetDelta(geom.NewVec(deltaB.X-impact*inv*normal.X, deltaB.Y-impact*inv*normal.Y))
	}
	return impact
}
