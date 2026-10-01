package collision_test

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/uid"
)

func testSpace(t *testing.T) *aabbworld.Space {
	t.Helper()
	space, err := aabbworld.NewSpace(aabbworld.Config{
		Width: 1000, Height: 1000,
		BucketSize: 64,
	})
	if err != nil {
		t.Fatalf("aabbworld.NewSpace: %v", err)
	}
	return space
}

func posAt(x, y, w, h float64) world.Position {
	return world.Position{AABB: plane.NewAABB(geom.NewVec(x, y), w, h)}
}

// seedBroadPhaseEntity spawns one entity carrying Collider.
func seedBroadPhaseEntity(t *testing.T, si *goke.SysInit, space *aabbworld.Space, pos world.Position) uid.UID64 {
	t.Helper()
	var baseComp goke.Comp[world.Base]
	var collComp goke.Comp[collision.Collider]
	f := si.NewFactory(&baseComp, &collComp)
	f.Create(1)
	f.Next()
	baseComp.Slice(&f.Cursor)[0].Pos = pos
	return f.IDs[0]
}

// seedNonCollidableEntity is seedBroadPhaseEntity without a Collider.
func seedNonCollidableEntity(t *testing.T, si *goke.SysInit, space *aabbworld.Space, pos world.Position) uid.UID64 {
	t.Helper()
	var baseComp goke.Comp[world.Base]
	f := si.NewFactory(&baseComp)
	f.Create(1)
	f.Next()
	baseComp.Slice(&f.Cursor)[0].Pos = pos
	return f.IDs[0]
}

// seedMovingCollidableEntity is seedBroadPhaseEntity plus a starting Velocity.
func seedMovingCollidableEntity(t *testing.T, si *goke.SysInit, space *aabbworld.Space, pos world.Position, vel world.Velocity) uid.UID64 {
	t.Helper()
	var baseComp goke.Comp[world.Base]
	var collComp goke.Comp[collision.Collider]
	f := si.NewFactory(&baseComp, &collComp)
	f.Create(1)
	f.Next()
	baseComp.Slice(&f.Cursor)[0] = world.Base{Pos: pos, Vel: vel}
	return f.IDs[0]
}

type pair struct{ A, B uid.UID64 }

// contactsOf lists every pair the last tick confirmed a contact between, once.
func contactsOf(q *goke.Query, comp *goke.Comp[collision.Collider]) []pair {
	var found []pair
	for q.All(); q.Next(); {
		cursor := q.Cursor()
		for i, c := range comp.Slice(cursor) {
			for _, contact := range c.Contacts() {
				if cursor.IDs[i].Index() < contact.Other.Index() {
					found = append(found, pair{cursor.IDs[i], contact.Other})
				}
			}
		}
	}
	return found
}

// broadTick seeds a world, runs ticks of movement and detection, and lists who touched on the last.
func broadTick(t *testing.T, ticks int, seed func(si *goke.SysInit, space *aabbworld.Space)) []pair {
	t.Helper()
	space := testSpace(t)
	ecs := goke.New()
	var coll goke.Comp[collision.Collider]
	var q *goke.Query
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		seed(si, space)
		q = si.NewQueryBuilder(&coll).Build()
	}})

	move := ecs.RegSys(world.NewMoveSystem(space))
	detect := ecs.RegSys(collision.NewCollisionSystem(space))
	ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) {
		ctx.Run(move, d)
		ctx.Run(detect, d)
		ctx.Sync()
	})
	for range ticks {
		ecs.Tick(time.Second)
	}
	return contactsOf(q, &coll)
}

// paired reports whether found names a and b as a pair, either way round.
func paired(found []pair, a, b uid.UID64) bool {
	for _, c := range found {
		if (c.A == a && c.B == b) || (c.A == b && c.B == a) {
			return true
		}
	}
	return false
}

func TestPairs_NamesOverlappingNeighborsOnce(t *testing.T) {
	var idA, idB uid.UID64
	found := broadTick(t, 1, func(si *goke.SysInit, space *aabbworld.Space) {
		idA = seedBroadPhaseEntity(t, si, space, posAt(0, 0, 10, 10))
		idB = seedBroadPhaseEntity(t, si, space, posAt(5, 0, 10, 10))
	})

	if len(found) != 1 || !paired(found, idA, idB) {
		t.Errorf("found %v, want the one pair (%v, %v)", found, idA, idB)
	}
}

func TestPairs_FarApart_NothingNamed(t *testing.T) {
	found := broadTick(t, 1, func(si *goke.SysInit, space *aabbworld.Space) {
		seedBroadPhaseEntity(t, si, space, posAt(0, 0, 10, 10))
		seedBroadPhaseEntity(t, si, space, posAt(900, 900, 10, 10))
	})

	if len(found) != 0 {
		t.Errorf("found %v between two entities a world apart, want nothing", found)
	}
}

func TestPairs_SingleEntity_NeverPairsWithItself(t *testing.T) {
	found := broadTick(t, 1, func(si *goke.SysInit, space *aabbworld.Space) {
		seedBroadPhaseEntity(t, si, space, posAt(0, 0, 10, 10))
	})

	if len(found) != 0 {
		t.Errorf("found %v for a lone entity, want nothing", found)
	}
}

func TestPairs_IgnoresNonCollidableNeighbor(t *testing.T) {
	found := broadTick(t, 1, func(si *goke.SysInit, space *aabbworld.Space) {
		seedBroadPhaseEntity(t, si, space, posAt(0, 0, 10, 10))
		seedNonCollidableEntity(t, si, space, posAt(5, 0, 10, 10))
	})

	if len(found) != 0 {
		t.Errorf("found %v, want nothing — the neighbor cannot collide", found)
	}
}

func TestPairs_FollowsAnEntityMovedByMoveSystem(t *testing.T) {
	var idA, idB uid.UID64
	found := broadTick(t, 1, func(si *goke.SysInit, space *aabbworld.Space) {
		idA = seedMovingCollidableEntity(t, si, space, posAt(88, 0, 10, 10), world.Velocity{Dir: geom.NewVec(1, 0), Value: 1000})
		idB = seedBroadPhaseEntity(t, si, space, posAt(100, 0, 10, 10))
	})

	if !paired(found, idA, idB) {
		t.Errorf("found %v, want (%v, %v) once MoveSystem brought A into B", found, idA, idB)
	}
}
