package collision

import (
	"log"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/collide"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	cgoke "github.com/kjkrol/ceremony/goke"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/collision/internal/response"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*collisionSystem)(nil)
var _ collide.Handler = (*handler)(nil)
var _ collide.FieldHandler = (*handler)(nil)
var _ collide.WallField = (*solidField)(nil)

const solverIterations = 16

// how deep a swept entity is said to lie in what it struck: enough for the normal, never a push
const sweptPen = 1e-3

type collisionSystem struct {
	space  *aabbworld.Space
	engine collide.Engine
	tickOf host.TickSource
	pace   goke.Opt

	walk      *goke.Query
	base      goke.Comp[world.Base]
	collider  goke.Comp[Collider]
	physics   goke.OptComp[Physics]
	walkSweep goke.OptComp[Sweep]
	each      *host.Rules[Struck]
	cols      *cgoke.Columns
	walking   struct {
		ids       []uid.UID64
		colliders []Collider
	}
	struckAt func(i int) Struck
	hitAt    func(i int) bool

	all       *goke.Query
	allBase   goke.Comp[world.Base]
	allSweep  goke.OptComp[Sweep]
	runs      []aabbworld.Run
	stretches [][]aabbworld.Item // a swept chunk's stretched boxes, in the chunk's place

	lookup         *goke.Query
	lookupBase     goke.Comp[world.Base]
	lookupCollider goke.Comp[Collider]
	lookupPhysics  goke.OptComp[Physics]
	lookupLayers   goke.OptComp[world.Layers]
	lookupZ        goke.OptComp[world.Z]
	lookupSweep    goke.OptComp[Sweep]
	lookupDriven   goke.OptComp[steering.Driven]
	lookupCourse   goke.OptComp[steering.Course]
	lookupHot      bool

	pair     pairSides
	pending  []pendingContact
	resting  []restingPair            // the pairs of the tick before, to keep while they touch
	met      map[restingPair]struct{} // the pairs in contacts this tick, a < b; emptied after rest
	nearests []nearest
	contacts []pairSides
	between  *host.PairRules[Meeting]
	seekCols *cgoke.Columns
	leave    func(cb *goke.CmdBuf, id uid.UID64)

	fieldOf func() Field
	heights bool
	stats   *ContactStats
	log     *log.Logger
	field   solidField
	ground  struct {
		base    world.Base
		physics Physics
	}

	sweeping bool
	wraps    bool

	// the boxes as the tick found them, by entity index: the engine pushes them where they lie
	priors  []prior
	ticks   uint32
	keeping bool

	tick  host.Tick
	stale bool
}

// an entity's box as the tick found it, kept the first time the engine asks of the entity,
// which it does before any push of it; moved takes a push back through base
type prior struct {
	box    plane.AABB
	base   *world.Base
	layers world.Layers
	tick   uint32
}

type handler collisionSystem

func (h *handler) Touch(a, b uid.UID64, pen geom.Vec) (geom.Vec, float32, bool) {
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

type solidField struct {
	field Field
	d     *collisionSystem

	swept          bool
	from, to, half geom.Vec
	along          float32
	normal         geom.Vec
	inner          func(collide.FieldBox) bool
	hit            func(collide.FieldBox) bool // the method value, bound once

	held       bool
	stopped    bool
	best       collide.FieldBox
	bestAlong  float32
	bestNormal geom.Vec

	innerWall func(collide.FieldWall) bool
	hitWall   func(collide.FieldWall) bool
	bestWall  collide.FieldWall
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
	f.held, f.stopped = false, false
	f.field.Solid(layers, band, box, f.hit)
	if f.held && !f.stopped {
		f.tell()
	}
	f.inner = nil
}

func (f *solidField) Walls(id uid.UID64, box geom.AABB, visit func(collide.FieldWall) bool) {
	if !f.d.seek(id) {
		return
	}
	cur := f.d.lookup.Cursor()
	layers, band := world.LayersOf(f.d.lookupLayers.At(cur)), f.d.bandAt(cur)
	f.swept, f.along = false, 1
	sw := f.d.lookupSweep.At(cur)
	if sw == nil {
		f.field.Walls(layers, band, box, visit)
		return
	}
	pos := f.d.lookupBase.At(cur).Pos
	f.swept, f.from, f.to, f.half, f.innerWall = true, sw.From, pos.Center(), halfOf(pos), visit
	f.held, f.stopped = false, false
	f.field.Walls(layers, band, box, f.hitWall)
	if f.held && !f.stopped {
		f.tellWall()
	}
	f.innerWall = nil
}

func (f *solidField) segmentWall(w collide.FieldWall) bool {
	along, normal, ok := response.SweepWall(f.from, f.to, f.half, w.A, w.B, w.Half)
	if !ok {
		return true
	}
	if f.held && w.Cell != f.bestWall.Cell && !f.tellWall() {
		return false
	}
	if !f.held || along < f.bestAlong {
		f.held, f.bestWall, f.bestAlong, f.bestNormal = true, w, along, normal
	}
	return true
}

func (f *solidField) tellWall() bool {
	f.held = false
	f.along, f.normal = f.bestAlong, f.bestNormal
	if !f.innerWall(f.bestWall) {
		f.stopped = true
		return false
	}
	return true
}

// a Field hands a cell's boxes one after another: the cell is told of at its nearest as the walk
// moves on
func (f *solidField) segment(fb collide.FieldBox) bool {
	along, normal, ok := response.Sweep(f.from, f.to, f.half, fb.Box)
	if !ok {
		return true
	}
	if f.held && fb.Cell != f.best.Cell && !f.tell() {
		return false
	}
	if !f.held || along < f.bestAlong {
		f.held, f.best, f.bestAlong, f.bestNormal = true, fb, along, normal
	}
	return true
}

func (f *solidField) tell() bool {
	f.held = false
	f.along, f.normal = f.bestAlong, f.bestNormal
	if !f.inner(f.best) {
		f.stopped = true
		return false
	}
	return true
}

func (d *collisionSystem) bandAt(cur *goke.Cursor) Band {
	if !d.heights {
		return Everywhere
	}
	return BandOf(d.lookupZ.At(cur))
}

func halfOf(pos world.Position) geom.Vec { return geom.NewVec(pos.Size.X/2, pos.Size.Y/2) }

// the one query the system offers its hosted rules
const sought = 0

func newCollisionSystem(space *aabbworld.Space, between *host.PairRules[Meeting], each *host.Rules[Struck], fieldOf func() Field) *collisionSystem {
	d := &collisionSystem{space: space, between: between, each: each, fieldOf: fieldOf}
	d.met = make(map[restingPair]struct{}, 256)
	d.struckAt, d.hitAt = d.struck, d.hit
	d.ground.physics = Physics{Mass: f32.Inf(1)}
	d.field.d = d
	d.field.hit = d.field.segment
	d.field.hitWall = d.field.segmentWall
	return d
}

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
	qb := si.NewQueryBuilder(&d.base, &d.collider).Optional(&d.physics)
	d.cols = cgoke.NewColumns(qb)
	d.cols.Own(&d.walkSweep)
	d.each.Bind(d.cols)
	if d.pace != nil {
		qb.With(d.pace)
	}
	d.walk = qb.Build()

	d.all = si.NewQueryBuilder(&d.allBase).Optional(&d.allSweep).
		Exclude(goke.Exclude[sector.Off]()).
		Build()

	seek := si.NewQueryBuilder(&d.lookupBase, &d.lookupCollider).
		Optional(&d.lookupPhysics, &d.lookupLayers, &d.lookupZ, &d.lookupSweep, &d.lookupDriven).
		Optional(&d.lookupCourse)
	d.seekCols = cgoke.NewColumns(seek)
	d.between.Bind(d.seekCols)
	d.lookup = seek.Build()
}

func (d *collisionSystem) Update(cb *goke.CmdBuf, dt time.Duration) {
	d.tick = d.tickOf.Of(dt)
	d.sweeping = false
	if d.mark() || d.sweeping {
		d.rebuild(d.sweeping)
	}

	d.pending = d.pending[:0]
	d.lookupHot, d.stale = false, false
	d.ticks++
	d.keeping = d.field.field != nil || d.sweeping
	d.engine.Tick()
	for _, l := range d.engine.Left() {
		d.leave(cb, l.ID)
	}
	if d.sweeping {
		d.nearest()
	}
	if d.stale || d.sweeping {
		d.rebuild(false)
	}
	d.record()
	if d.stats != nil {
		d.stats.Counter += len(d.contacts)
	}
	d.rest()

	for i := range d.contacts {
		s := &d.contacts[i]
		toward := between(s.A.Base, s.B.Base)
		d.between.DispatchEitherWay(d.tick, s.tagsA, s.tagsB,
			Meeting{Self: s.A.Entity, Other: s.B.Entity, Impact: s.impact, Normal: s.normal,
				Heading: s.A.Base.Vel.Dir, Toward: toward},
			Meeting{Self: s.B.Entity, Other: s.A.Entity, Impact: s.impact,
				Normal:  geom.NewVec(-s.normal.X, -s.normal.Y),
				Heading: s.B.Base.Vel.Dir, Toward: geom.NewVec(-toward.X, -toward.Y)})
		if d.log != nil {
			d.log.Printf("collision: %v <-> %v (impact %.2f)", s.A.Entity, s.B.Entity, s.impact)
		}
	}
}

func (d *collisionSystem) mark() bool {
	changed := false
	d.walk.All()
	for d.walk.Next() {
		cursor := d.walk.Cursor()
		bases, colliders, physics := d.base.Slice(cursor), d.collider.Slice(cursor), d.physics.Slice(cursor)
		if !d.walk.Due() {
			changed = d.still(bases, colliders) || changed
			continue
		}
		sweeps := d.walkSweep.Slice(cursor)

		d.walking.ids, d.walking.colliders = cursor.IDs, colliders
		d.each.RunWhere(d.tick, d.cols.Row(cursor), len(cursor.IDs), d.hitAt, d.struckAt)
		for i, id := range cursor.IDs {
			for _, c := range colliders[i].Contacts() {
				if !c.Terrain && !c.Sensed && id < c.Other { // each pair once
					d.resting = append(d.resting, restingPair{a: id, b: c.Other})
				}
			}
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

// far off a box collides with nothing, out of the pairs and the push from the ground
func (d *collisionSystem) still(bases []world.Base, colliders []Collider) bool {
	changed := false
	for i := range bases {
		colliders[i].clearContacts()
		if bases[i].Caps != aabbworld.Plain {
			bases[i].Caps, changed = aabbworld.Plain, true
		}
	}
	return changed
}

// the space reads the boxes where they lie, a run a chunk; a swept one's stretched boxes are
// copies, which the engine never pushes: a swept entity is a sensor
func (d *collisionSystem) rebuild(stretch bool) {
	d.runs = d.runs[:0]
	swept := 0
	for d.all.All(); d.all.Next(); {
		cursor := d.all.Cursor()
		bases, sweeps := d.allBase.Slice(cursor), d.allSweep.Slice(cursor)
		if !stretch || sweeps == nil {
			d.runs = append(d.runs, world.RunOf(cursor.IDs, bases))
			continue
		}
		if swept == len(d.stretches) {
			d.stretches = append(d.stretches, nil)
		}
		items := d.stretches[swept][:0]
		for i, b := range bases {
			box := stretched(sweeps[i].From, b.Pos)
			items = append(items, aabbworld.Item{ID: cursor.IDs[i], Box: box, Caps: b.Caps})
		}
		d.stretches[swept] = items
		swept++
		d.runs = append(d.runs, aabbworld.RunOf(cursor.IDs, items, itemBox, itemCaps))
	}
	d.space.RebuildRuns(d.runs, nil)
}

func itemBox(it *aabbworld.Item) *plane.AABB { return &it.Box }

func itemCaps(it *aabbworld.Item) *aabbworld.Capability { return &it.Caps }

func stretched(from geom.Vec, pos world.Position) plane.AABB {
	half := halfOf(pos)
	lo := geom.NewVec(min(from.X-half.X, pos.TopLeft.X), min(from.Y-half.Y, pos.TopLeft.Y))
	hi := geom.NewVec(max(from.X+half.X, pos.BottomRight.X), max(from.Y+half.Y, pos.BottomRight.Y))
	return plane.NewAABB(lo, hi.X-lo.X, hi.Y-lo.Y)
}

func (d *collisionSystem) struck(i int) Struck {
	return Struck{ID: d.walking.ids[i], Contacts: d.walking.colliders[i].Contacts()}
}

func (d *collisionSystem) hit(i int) bool {
	for _, c := range d.walking.colliders[i].Contacts() {
		if !c.Resting {
			return true
		}
	}
	return false
}

type restingPair struct{ a, b uid.UID64 }

func ordered(a, b uid.UID64) restingPair {
	if b < a {
		return restingPair{a: b, b: a}
	}
	return restingPair{a: a, b: b}
}

// what rests a hair apart still touches
const restGap = 1e-3

// touching reports whether the boxes meet or lie within restGap of each other.
func touching(a, b geom.AABB) bool {
	gapX := max(a.TopLeft.X-b.BottomRight.X, b.TopLeft.X-a.BottomRight.X)
	gapY := max(a.TopLeft.Y-b.BottomRight.Y, b.TopLeft.Y-a.BottomRight.Y)
	return max(gapX, gapY) <= restGap
}

// rest keeps the pairs of the tick before that still touch though nothing pushed them this
// tick: a contact at rest on both sides and a Meeting, with no impulse.
func (d *collisionSystem) rest() {
	for i := range d.contacts {
		c := &d.contacts[i]
		d.met[ordered(c.A.Entity, c.B.Entity)] = struct{}{}
	}
	for _, p := range d.resting {
		if _, ok := d.met[p]; ok {
			continue
		}
		a, tagsA, okA := d.side(p.a)
		b, tagsB, okB := d.side(p.b)
		if !okA || !okB || a.Sweep != nil || b.Sweep != nil || !a.Layers.Meets(b.Layers) ||
			!touching(a.Base.Pos.AABB.AABB, b.Base.Pos.AABB.AABB) {
			continue
		}
		if d.heights && !a.Band.Meets(b.Band) {
			continue
		}
		normal := faceOf(b.Base, a.Base)
		a.Collider.add(Contact{Other: b.Entity, Normal: normal, Along: 1, Resting: true})
		b.Collider.add(Contact{Other: a.Entity, Normal: geom.NewVec(-normal.X, -normal.Y),
			Along: 1, Resting: true})
		d.contacts = append(d.contacts, pairSides{A: a, B: b, tagsA: tagsA, tagsB: tagsB,
			normal: normal, along: 1})
		d.met[p] = struct{}{}
	}
	d.resting = d.resting[:0]
	clear(d.met)
}

type pairSides struct {
	A, B         contactSide
	tagsA, tagsB host.Marks

	impact float32
	normal geom.Vec
	along  float32 // where along a swept side's step, 1 for none swept
	// a held side stays where it is: the push would put it over ground that does not take it
	holdA, holdB bool
}

func (p *pairSides) detectOnly() bool { return p.A.Physics == nil || p.B.Physics == nil }

func (p *pairSides) swept() (uid.UID64, bool) {
	switch {
	case p.A.Sweep != nil:
		return p.A.Entity, true
	case p.B.Sweep != nil:
		return p.B.Entity, true
	}
	return 0, false
}

type pendingContact struct {
	sides   pairSides
	terrain bool
	cell    uint64
	sensed  bool
	dropped bool
}

type nearest struct {
	id    uid.UID64
	at    int
	along float32
}

type contactSide struct {
	Entity   uid.UID64
	Base     *world.Base
	Collider *Collider
	Physics  *Physics // nil for a side only ever detected, a swept one among them
	Layers   world.Layers
	Band     Band
	Sweep    *Sweep
	Steered  bool
	Course   *steering.Course
}

func (s contactSide) side() response.Side {
	side := response.Side{
		InvMass: s.Physics.inverseMass(s.Base.Pos.Size),
		Bounce:  s.Physics.bounce(),
		Vel:     &s.Base.Vel,
		Steered: s.Steered,
	}
	if s.Course != nil {
		side.Speed = &s.Course.Speed
	}
	return side
}

func (s contactSide) body() response.Body {
	return response.Body{Layers: s.Layers, Box: s.Base.Pos.AABB.AABB, Immovable: s.Physics.immovable()}
}

// the share is a's part of the push apart; a pair with a swept side is never pushed
func (d *collisionSystem) resolve(a, b uid.UID64, pen geom.Vec) (geom.Vec, float32, bool) {
	sideA, tagsA, ok := d.side(a)
	if !ok {
		return pen, 0.5, false
	}
	sideB, tagsB, ok := d.side(b)
	if !ok {
		return pen, 0.5, false
	}
	if !sideA.Layers.Meets(sideB.Layers) || !sideA.Band.Meets(sideB.Band) {
		return pen, 0.5, false
	}
	d.pair = pairSides{A: sideA, B: sideB, tagsA: tagsA, tagsB: tagsB, along: 1}
	if sideA.Sweep == nil && sideB.Sweep == nil {
		return pen, d.footing(pen), true
	}
	if sideA.Sweep != nil && sideB.Sweep != nil {
		return pen, 0.5, false
	}
	mover, other, moverIsA := sideA, sideB, true
	if sideB.Sweep != nil {
		mover, other, moverIsA = sideB, sideA, false
	}
	if mover.Sweep.Ignoring && mover.Sweep.Ignore == other.Entity {
		return pen, 0.5, false
	}
	along, normal, hit := response.Sweep(mover.Sweep.From, mover.Base.Pos.Center(),
		halfOf(mover.Base.Pos), d.before(other.Entity))
	if !hit {
		return pen, 0.5, false
	}
	d.pair.along = along
	if !moverIsA {
		normal = geom.NewVec(-normal.X, -normal.Y)
	}
	return geom.NewVec(normal.X*sweptPen, normal.Y*sweptPen), 0.5, true
}

// a's part of the push apart by weight, none for a side it would put over worse ground
func (d *collisionSystem) footing(pen geom.Vec) float32 {
	p := &d.pair
	if p.detectOnly() {
		return 0.5
	}
	share := response.Share(p.A.Physics.inverseMass(p.A.Base.Pos.Size),
		p.B.Physics.inverseMass(p.B.Base.Pos.Size))
	if d.field.field == nil {
		return share
	}
	a, b := p.A.body(), p.B.body()
	a.Box, b.Box = d.before(p.A.Entity), d.before(p.B.Entity)
	share, p.holdA, p.holdB = response.Footing(d.field.field, a, b, pen, share)
	return share
}

func (d *collisionSystem) side(id uid.UID64) (contactSide, host.Marks, bool) {
	if !d.seek(id) {
		if d.all.Seek(id) {
			d.allBase.At(d.all.Cursor()).Caps, d.stale = aabbworld.Plain, true
		}
		return contactSide{}, host.Marks{}, false
	}
	cur := d.lookup.Cursor()
	s := contactSide{
		Entity: id, Base: d.lookupBase.At(cur),
		Collider: d.lookupCollider.At(cur), Physics: d.lookupPhysics.At(cur),
		Layers: world.LayersOf(d.lookupLayers.At(cur)), Band: d.bandAt(cur),
		// a hand's unit keeps its heading, the keys held or not; one under orders glances
		// along a wall, as navigation needs to come round its end
		Steered: d.lookupDriven.At(cur) != nil,
		Course:  d.lookupCourse.At(cur),
	}
	if s.Sweep = d.lookupSweep.At(cur); s.Sweep != nil {
		s.Physics = nil
	}
	if d.keeping {
		d.keep(id, s.Base, s.Layers)
	}
	return s, d.between.Read(sought, d.seekCols.Row(cur), int(cur.Slot)), true
}

func (d *collisionSystem) keep(id uid.UID64, base *world.Base, layers world.Layers) {
	i := int(id.Index())
	if i >= len(d.priors) {
		d.priors = append(d.priors, make([]prior, i+1-len(d.priors))...)
	}
	if p := &d.priors[i]; p.tick != d.ticks {
		*p = prior{box: base.Pos.AABB, base: base, layers: layers, tick: d.ticks}
	}
}

// the box the entity stood in as the tick began; it was kept, the engine having asked of it
func (d *collisionSystem) before(id uid.UID64) geom.AABB { return d.priors[id.Index()].box.AABB }

func (d *collisionSystem) seek(id uid.UID64) bool {
	ok := d.lookupHot && d.lookup.SeekH(id)
	if !ok {
		ok = d.lookup.Seek(id)
		d.lookupHot = ok
	}
	return ok
}

func (d *collisionSystem) contact(pen geom.Vec) {
	sides := d.pair

	normal, aligned := response.Normal(pen)
	var impact float32
	if aligned && !sides.detectOnly() {
		a, b := sides.A, sides.B
		if sides.holdA {
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

func (d *collisionSystem) contactGround(id uid.UID64, cell uint64, pen geom.Vec) {
	self, _, ok := d.side(id)
	if !ok {
		return
	}
	ground := contactSide{Base: &d.ground.base, Physics: &d.ground.physics}
	normal, aligned := response.Normal(pen)
	var impact float32
	if aligned && self.Physics != nil {
		impact = bounce(self, ground, normal)
	}
	sides := pairSides{A: self, impact: impact, normal: normal, along: 1}
	if self.Sweep != nil {
		sides.along = d.field.along
	}
	d.pending = append(d.pending, pendingContact{sides: sides, terrain: true, cell: cell, sensed: self.Physics == nil})
}

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

// the engine pushed the box where it lies; its later passes ask no handler, so a box they left
// further over ground that does not take it goes back where it stood
func (d *collisionSystem) moved(id uid.UID64, box plane.AABB) {
	if d.field.field == nil {
		return
	}
	p := &d.priors[id.Index()]
	if p.tick == d.ticks && response.Worse(d.field.field, p.layers, p.box.AABB, box.AABB) {
		p.base.Pos.AABB = p.box
	}
}

func bounce(a, b contactSide, normal geom.Vec) float32 {
	return response.Exchange(a.side(), b.side(), normal)
}

// faceOf is the way from a to b along the axis they touch on, as a struck pair's normal is; not
// the line between their middles, which a small box off the middle of a big one's face slants
func faceOf(a, b *world.Base) geom.Vec {
	ba, bb := a.Pos.AABB.AABB, b.Pos.AABB.AABB
	gapX := max(ba.TopLeft.X-bb.BottomRight.X, bb.TopLeft.X-ba.BottomRight.X)
	gapY := max(ba.TopLeft.Y-bb.BottomRight.Y, bb.TopLeft.Y-ba.BottomRight.Y)
	ca, cb := a.Pos.Center(), b.Pos.Center()
	if gapX >= gapY {
		return geom.NewVec(f32.Copysign(1, cb.X-ca.X), 0)
	}
	return geom.NewVec(0, f32.Copysign(1, cb.Y-ca.Y))
}

// the unit vector from a's middle to b's; zero for one middle
func between(a, b *world.Base) geom.Vec {
	ca, cb := a.Pos.Center(), b.Pos.Center()
	dx, dy := cb.X-ca.X, cb.Y-ca.Y
	l := f32.Hypot(dx, dy)
	if l == 0 {
		return geom.Vec{}
	}
	return geom.NewVec(dx/l, dy/l)
}
