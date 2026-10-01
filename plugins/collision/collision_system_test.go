package collision_test

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/uid"
)

const epsilon = 1e-9

// thing is one entity of a collision fixture — a 10x10 box at y=100 — and,
// once the tick has run, what became of it.
type thing struct {
	x       float64
	delta   geom.Vec
	physics *collision.Physics // nil: only ever detected
	first   bool               // spawned before the others

	id       uid.UID64
	base     world.Base
	contacts []collision.Contact
}

func elastic(mass float64) *collision.Physics {
	return &collision.Physics{Mass: mass, Restitution: 1}
}

// detectTick spawns things, runs one collision tick over them and reads each back.
func detectTick(t *testing.T, things ...*thing) {
	t.Helper()
	space := testSpace(t)

	var base goke.Comp[world.Base]
	var coll goke.Comp[collision.Collider]
	var physics goke.Comp[collision.Physics]
	var seen goke.Comp[collision.Collider]
	var read *goke.Query

	ecs := goke.New()
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		for _, th := range things {
			comps := []goke.Addable{&base, &coll}
			if th.physics != nil {
				comps = append(comps, &physics)
			}
			f := si.NewFactory(comps...)
			f.Create(1)
			f.Next()
			th.id = f.IDs[0]
			placed := world.Base{Pos: posAt(th.x, 100, 10, 10)}
			placed.Vel.SetDelta(th.delta)
			base.Slice(&f.Cursor)[0] = placed
			if th.physics != nil {
				physics.Slice(&f.Cursor)[0] = *th.physics
			}
		}

		read = si.NewQueryBuilder(&base, &seen).Build()
	}})

	handle := ecs.RegSys(collision.NewCollisionSystem(space))
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(handle, d)
		ctx.Sync()
	})
	ecs.Tick(time.Millisecond)

	byID := map[uid.UID64]*thing{}
	for _, th := range things {
		byID[th.id] = th
	}
	for read.All(); read.Next(); {
		cursor := read.Cursor()
		bases := base.Slice(cursor)
		recorded := seen.Slice(cursor)
		for i, id := range cursor.IDs {
			th := byID[id]
			th.base = bases[i]
			th.contacts = append([]collision.Contact(nil), recorded[i].Contacts()...)
		}
	}
}

func (th *thing) left() float64 { return float64(th.base.Pos.TopLeft.X) }

func (th *thing) speedX() float64 { return th.base.Vel.Delta().X }

func TestCollisionSystem_PhysicalPair_IsPushedApart(t *testing.T) {
	a := &thing{x: 100, physics: elastic(1)}
	b := &thing{x: 105, physics: elastic(1)}

	detectTick(t, a, b)

	if a.left() >= 100 || b.left() <= 105 {
		t.Errorf("a at %v, b at %v — want both pushed out of a 5-unit overlap", a.left(), b.left())
	}
}

func TestCollisionSystem_ImmovableSide_StaysPutAndReflectsTheOther(t *testing.T) {
	ball := &thing{x: 100, delta: geom.NewVec(4, 0), physics: elastic(1)}
	wall := &thing{x: 105, physics: elastic(math.Inf(1))}

	detectTick(t, ball, wall)

	if wall.left() != 105 {
		t.Errorf("the wall moved to %v, want it left at 105", wall.left())
	}
	if ball.left() != 95 {
		t.Errorf("the ball is at %v, want it pushed the whole 5 units out, to 95", ball.left())
	}
	if math.Abs(ball.speedX()+4) > epsilon {
		t.Errorf("the ball moves at %v, want -4 — straight back off the wall", ball.speedX())
	}
	if len(ball.contacts) != 1 || math.Abs(ball.contacts[0].Impact-8) > epsilon {
		t.Errorf("contacts = %+v, want one at impact 8", ball.contacts)
	}
}

func TestCollisionSystem_ImmovableSpawnedFirst_StillStopsWhatRunsIntoIt(t *testing.T) {
	wall := &thing{x: 105, physics: elastic(math.Inf(1))}
	ball := &thing{x: 100, delta: geom.NewVec(4, 0), physics: elastic(1)}

	detectTick(t, wall, ball)

	if wall.id.Index() >= ball.id.Index() {
		t.Fatalf("fixture broken: wall index %d, ball index %d — the wall has to come first", wall.id.Index(), ball.id.Index())
	}
	if wall.left() != 105 {
		t.Errorf("the wall moved to %v, want it left at 105", wall.left())
	}
	if ball.left() != 95 || math.Abs(ball.speedX()+4) > epsilon {
		t.Errorf("the ball is at %v moving %v, want 95 and -4", ball.left(), ball.speedX())
	}
}

func TestCollisionSystem_SideWithoutPhysics_IsDetectedButNeverPushed(t *testing.T) {
	for name, town := range map[string]*thing{
		"spawned after the walker":  {x: 105},
		"spawned before the walker": {x: 105, first: true},
	} {
		t.Run(name, func(t *testing.T) {
			walker := &thing{x: 100, delta: geom.NewVec(4, 0), physics: elastic(1)}
			order := []*thing{walker, town}
			if town.first {
				order = []*thing{town, walker}
			}

			detectTick(t, order...)

			if walker.left() != 100 || town.left() != 105 {
				t.Errorf("walker at %v, town at %v — want neither pushed", walker.left(), town.left())
			}
			if math.Abs(walker.speedX()-4) > epsilon {
				t.Errorf("the walker moves at %v, want its 4 untouched", walker.speedX())
			}
			for who, th := range map[string]*thing{"walker": walker, "town": town} {
				if len(th.contacts) != 1 || th.contacts[0].Impact != 0 {
					t.Errorf("%s contacts = %+v, want exactly one, at zero impact", who, th.contacts)
				}
			}
			if len(walker.contacts) == 1 && walker.contacts[0].Other != town.id {
				t.Errorf("the walker struck %v, want the town %v", walker.contacts[0].Other, town.id)
			}
		})
	}
}

func TestCollisionSystem_KeepsIteratingWhileSeparationCreatesNewOverlap(t *testing.T) {
	a := &thing{x: 100, physics: elastic(1)}
	b := &thing{x: 102, physics: elastic(1)}
	c := &thing{x: 104, physics: elastic(1)}

	detectTick(t, a, b, c)

	for _, pair := range []struct {
		name string
		l, r *thing
	}{{"A/B", a, b}, {"B/C", b, c}, {"A/C", a, c}} {
		if d := overlapDepth(pair.l.base.Pos, pair.r.base.Pos); d > 1e-6 {
			t.Errorf("%s still overlap by %v after the solver ran", pair.name, d)
		}
	}
}

func TestCollisionSystem_SqueezedEntity_BouncesOffBothNeighboursInTurn(t *testing.T) {
	left := &thing{x: 93, delta: geom.NewVec(5, 0), physics: elastic(1)}
	middle := &thing{x: 100, physics: elastic(1)}
	right := &thing{x: 107, delta: geom.NewVec(-5, 0), physics: elastic(1)}

	detectTick(t, left, middle, right)

	for i, want := range []float64{0, -5, 5} {
		if got := []*thing{left, middle, right}[i].speedX(); math.Abs(got-want) > epsilon {
			t.Errorf("entity %d leaves the tick at %v, want %v", i, got, want)
		}
	}
	if len(middle.contacts) != 2 {
		t.Fatalf("the middle entity recorded %d contacts, want 2", len(middle.contacts))
	}
	for i, want := range []struct {
		other  uid.UID64
		impact float64
	}{{left.id, 5}, {right.id, 10}} {
		if got := middle.contacts[i]; got.Other != want.other || math.Abs(got.Impact-want.impact) > epsilon {
			t.Errorf("contact %d = (%v, impact %v), want (%v, %v)", i, got.Other, got.Impact, want.other, want.impact)
		}
	}
}

func TestCollisionSystem_EqualMasses_ExchangeVelocitiesAlongTheNormal(t *testing.T) {
	a := &thing{x: 100, delta: geom.NewVec(5, 2), physics: elastic(1)}
	b := &thing{x: 107, delta: geom.NewVec(-5, 2), physics: elastic(1)}

	detectTick(t, a, b)

	if math.Abs(a.speedX()+5) > epsilon || math.Abs(b.speedX()-5) > epsilon {
		t.Errorf("X = (%v, %v), want (-5, 5) swapped", a.speedX(), b.speedX())
	}
	if ay, by := a.base.Vel.Delta().Y, b.base.Vel.Delta().Y; math.Abs(ay-2) > epsilon || math.Abs(by-2) > epsilon {
		t.Errorf("Y = (%v, %v), want both left at 2 — nothing acts across the normal", ay, by)
	}
}

func TestCollisionSystem_HeavyAgainstLight_ConservesMomentumAndEnergy(t *testing.T) {
	const heavyMass, lightMass = 9, 1
	heavy := &thing{x: 100, delta: geom.NewVec(2, 0), physics: elastic(heavyMass)}
	light := &thing{x: 107, delta: geom.NewVec(-2, 0), physics: elastic(lightMass)}

	detectTick(t, heavy, light)

	h, l := heavy.speedX(), light.speedX()
	if got, want := heavyMass*h+lightMass*l, float64(heavyMass*2+lightMass*-2); math.Abs(got-want) > epsilon {
		t.Errorf("momentum = %v, want %v", got, want)
	}
	if got, want := heavyMass*h*h+lightMass*l*l, float64(heavyMass*4+lightMass*4); math.Abs(got-want) > epsilon {
		t.Errorf("kinetic energy (x2) = %v, want %v", got, want)
	}
	if h <= 0 || l <= 0 {
		t.Errorf("heavy moves at %v, light at %v — want the heavy one carrying on and the light one thrown back", h, l)
	}
}

// A pair bounces by the softer of its two sides, all the way down to not at all.
func TestCollisionSystem_Restitution_DampsTheBounce(t *testing.T) {
	cases := map[string]struct {
		restitutionA, restitutionB float64
		wantA, wantB               float64
	}{
		"perfectly elastic":   {1, 1, 4, -3},
		"half, from A":        {0.5, 1, 2.25, -1.25},
		"half, from B":        {1, 0.5, 2.25, -1.25},
		"perfectly inelastic": {0, 1, 0.5, 0.5},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			a := &thing{x: 107, delta: geom.NewVec(-3, 0), physics: &collision.Physics{Restitution: c.restitutionA}}
			b := &thing{x: 100, delta: geom.NewVec(4, 0), physics: &collision.Physics{Restitution: c.restitutionB}}

			detectTick(t, a, b)

			if math.Abs(a.speedX()-c.wantA) > epsilon || math.Abs(b.speedX()-c.wantB) > epsilon {
				t.Errorf("speeds = (%v, %v), want (%v, %v)", a.speedX(), b.speedX(), c.wantA, c.wantB)
			}
		})
	}
}

func TestCollisionSystem_Material_IsReadPerSide(t *testing.T) {
	a := &thing{x: 100, delta: geom.NewVec(5, 0), physics: &collision.Physics{Mass: 4, Restitution: 0.5}}
	b := &thing{x: 105, delta: geom.NewVec(-5, 0), physics: &collision.Physics{Restitution: 1}}

	detectTick(t, a, b)

	if len(a.contacts) != 1 || math.Abs(a.contacts[0].Impact-12) > epsilon {
		t.Errorf("contacts = %+v, want one at impact 12", a.contacts)
	}
}

func TestCollisionSystem_Contacts_PublishedToBothSides(t *testing.T) {
	a := &thing{x: 100, delta: geom.NewVec(5, 0), physics: elastic(1)}
	b := &thing{x: 105, delta: geom.NewVec(-5, 0), physics: elastic(1)}

	detectTick(t, a, b)

	for _, side := range []struct {
		self, other *thing
		leaves      float64
	}{{a, b, -1}, {b, a, 1}} {
		if len(side.self.contacts) != 1 {
			t.Fatalf("entity %v recorded %d contacts, want 1", side.self.id, len(side.self.contacts))
		}
		got := side.self.contacts[0]
		if got.Other != side.other.id {
			t.Errorf("entity %v recorded a contact with %v, want %v", side.self.id, got.Other, side.other.id)
		}
		if math.Abs(got.Impact-10) > epsilon {
			t.Errorf("entity %v recorded impact %v, want 10", side.self.id, got.Impact)
		}
		if got.Normal.X != side.leaves || got.Normal.Y != 0 {
			t.Errorf("entity %v was told to leave along %v, want (%v, 0) — away from the other", side.self.id, got.Normal, side.leaves)
		}
	}
}

func TestCollisionSystem_Contacts_DoNotSurviveTheNextTick(t *testing.T) {
	space := testSpace(t)
	ecs := goke.New()
	var struck goke.Comp[collision.Collider]
	var q *goke.Query

	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		seedPhysical(t, si, space, posAt(100, 100, 10, 10), geom.NewVec(5, 0))
		seedPhysical(t, si, space, posAt(105, 100, 10, 10), geom.NewVec(-5, 0))
		q = si.NewQueryBuilder(&struck).Build()
	}})

	detect := ecs.RegSys(collision.NewCollisionSystem(space))
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(detect, d)
		ctx.Sync()
	})

	ecs.Tick(time.Millisecond)
	if got := countContacts(q, struck); got != 2 {
		t.Fatalf("%d contacts recorded across both entities on the overlapping tick, want 2", got)
	}

	ecs.Tick(time.Millisecond)
	if got := countContacts(q, struck); got != 0 {
		t.Errorf("%d contacts left after a tick with no contact, want 0", got)
	}
}

// seedPhysical spawns a physical, collidable entity moving at delta.
func seedPhysical(t *testing.T, si *goke.SysInit, space *aabbworld.Space, pos world.Position, delta geom.Vec) uid.UID64 {
	t.Helper()
	var baseComp goke.Comp[world.Base]
	var collComp goke.Comp[collision.Collider]
	var physicsComp goke.Comp[collision.Physics]
	f := si.NewFactory(&baseComp, &collComp, &physicsComp)
	f.Create(1)
	f.Next()
	baseComp.Slice(&f.Cursor)[0].Pos = pos
	baseComp.Slice(&f.Cursor)[0].Vel.SetDelta(delta)
	physicsComp.Slice(&f.Cursor)[0] = collision.Physics{Restitution: 1}
	return f.IDs[0]
}

func countContacts(q *goke.Query, comp goke.Comp[collision.Collider]) int {
	var n int
	for q.All(); q.Next(); {
		for _, c := range comp.Slice(q.Cursor()) {
			n += len(c.Contacts())
		}
	}
	return n
}

// overlapDepth is how far two boxes penetrate on their shallower axis, zero or less when apart.
func overlapDepth(l, r world.Position) float64 {
	x := min(l.BottomRight.X, r.BottomRight.X) - max(l.TopLeft.X, r.TopLeft.X)
	y := min(l.BottomRight.Y, r.BottomRight.Y) - max(l.TopLeft.Y, r.TopLeft.Y)
	return min(x, y)
}
