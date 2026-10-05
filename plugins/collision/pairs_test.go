package collision_test

import (
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/internal/collisiontest"
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

// placed is one box of a pairs test: where it stands, how it moves, and whether it carries a
// Collider.
type placed struct {
	x, y     float64
	vel      world.Velocity
	collides bool
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

// broadTick puts boxes in a world with collision, steps it ticks seconds, and lists who touched
// on the last step; the boxes' entities come back in the order given.
func broadTick(t *testing.T, ticks int, boxes ...placed) ([]pair, []uid.UID64) {
	t.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: len(boxes), MinSize: 10, MaxSize: 10},
	})
	c := collision.NewPlugin(w)
	spec := kind.Spec{
		comp.Load(func(b placed) world.Position { return posAt(b.x, b.y, 10, 10) }),
		comp.Load(func(b placed) world.Velocity { return b.vel }),
	}
	kind.Define[placed](w.Kinds(), "collider", append(spec, comp.Const(collision.Collider{})))
	colliders := kind.Named[placed](w.Kinds(), "collider")
	kind.Define[placed](w.Kinds(), "inert", spec)
	inert := kind.Named[placed](w.Kinds(), "inert")
	for _, b := range boxes {
		if b.collides {
			w.Seed(colliders.Entry(b))
		} else {
			w.Seed(inert.Entry(b))
		}
	}
	if err := w.Populate(); err != nil {
		t.Fatal(err)
	}
	var coll goke.Comp[collision.Collider]
	var q *goke.Query
	ids := make([]uid.UID64, len(boxes))
	ecs := collisiontest.Start(t, w, c, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		q = si.NewQueryBuilder(&coll).Build()
		var base goke.Comp[world.Base]
		all := si.NewQueryBuilder(&base).Build()
		for all.All(); all.Next(); {
			cur := all.Cursor()
			for i, id := range cur.IDs {
				at := base.Slice(cur)[i].Pos.TopLeft
				for k, b := range boxes {
					if at.X == b.x && at.Y == b.y {
						ids[k] = id
					}
				}
			}
		}
	}})
	for range ticks {
		ecs.Tick(time.Second)
	}
	return contactsOf(q, &coll), ids
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
	found, ids := broadTick(t, 1, placed{x: 0, collides: true}, placed{x: 5, collides: true})

	if len(found) != 1 || !paired(found, ids[0], ids[1]) {
		t.Errorf("found %v, want the one pair (%v, %v)", found, ids[0], ids[1])
	}
}

func TestPairs_FarApart_NothingNamed(t *testing.T) {
	found, _ := broadTick(t, 1, placed{x: 0, collides: true}, placed{x: 900, y: 900, collides: true})

	if len(found) != 0 {
		t.Errorf("found %v between two entities a world apart, want nothing", found)
	}
}

func TestPairs_SingleEntity_NeverPairsWithItself(t *testing.T) {
	found, _ := broadTick(t, 1, placed{x: 0, collides: true})

	if len(found) != 0 {
		t.Errorf("found %v for a lone entity, want nothing", found)
	}
}

func TestPairs_IgnoresNonCollidableNeighbor(t *testing.T) {
	found, _ := broadTick(t, 1, placed{x: 0, collides: true}, placed{x: 5})

	if len(found) != 0 {
		t.Errorf("found %v, want nothing — the neighbor cannot collide", found)
	}
}

func TestPairs_FollowsAnEntityTheWorldMoved(t *testing.T) {
	found, ids := broadTick(t, 1,
		placed{x: 88, vel: world.Velocity{Dir: geom.NewVec(1, 0), Value: 1000}, collides: true},
		placed{x: 100, collides: true})

	if !paired(found, ids[0], ids[1]) {
		t.Errorf("found %v, want (%v, %v) once the world's move brought A into B", found, ids[0], ids[1])
	}
}
