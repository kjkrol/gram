package collision_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/internal/collisiontest"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
)

type laid struct{ x, side float32 }

// What an impulse takes from a steered unit stays taken: its steering goes on from the speed
// left, not from the one it held, the ground's pace kept apart.
func TestImpulse_TheSpeedItTakesFromASteeredUnitStaysTaken(t *testing.T) {
	steer := steering.Steering{MaxSpeed: 96, Accel: 192, Brake: 384, V0: 48}
	for name, c := range map[string]struct {
		wall bool
		pace float32 // 0, none
	}{
		"into one standing, four times its side": {},
		"into one standing, on half pace":        {pace: 0.5},
		"into the ground's wall":                 {wall: true},
	} {
		t.Run(name, func(t *testing.T) {
			w := world.NewPlugin(world.Config{
				Space:    world.SpaceCfg{Width: 1000, Height: 1000},
				Entities: world.EntitiesCfg{MaxCount: 2, MinSize: 6, MaxSize: 28},
			})
			m := motion.NewPlugin(w)
			coll := collision.NewPlugin(w, m)
			if c.wall {
				coll.WithField(&wallField{
					box:  geom.NewAABBAt(geom.NewVec(200, 0), 32, 1000),
					band: collision.Everywhere,
				})
			}
			walker := kind.Spec{
				comp.Load(func(r laid) world.Position { return posAt(r.x, 500, r.side, r.side) }),
				comp.Const(world.Velocity{Dir: geom.NewVec(1, 0), Value: 96}),
				comp.Const(collision.Collider{}),
				comp.Const(collision.Physics{}),
				comp.Const(steer),
				comp.Const(steering.Course{Want: geom.NewVec(1, 0), WantSpeed: 96, Speed: 96}),
			}
			share := float32(1)
			if c.pace > 0 {
				walker = append(walker, comp.Const(steering.Pace{Share: c.pace}))
				share = c.pace
			}
			kind.Define[laid](w.Kinds(), "walker", walker)
			kind.Define[laid](w.Kinds(), "standing", kind.Spec{
				comp.Load(func(r laid) world.Position { return posAt(r.x, 490, r.side, r.side) }),
				comp.Const(world.Velocity{}),
				comp.Const(collision.Collider{}),
				comp.Const(collision.Physics{}),
				comp.Const(steer),
			})
			entries := []kind.Entry{
				kind.Named[laid](w.Kinds(), "walker").Entry(laid{x: 180, side: 6}),
			}
			if !c.wall {
				standing := kind.Named[laid](w.Kinds(), "standing")
				entries = append(entries, standing.Entry(laid{x: 197, side: 28}))
			}
			w.Write().Seed(entries...)
			if err := w.Populate(); err != nil {
				t.Fatal(err)
			}
			var base goke.Comp[world.Base]
			var course goke.Comp[steering.Course]
			var colliders goke.Comp[collision.Collider]
			var q *goke.Query
			ecs := collisiontest.Start(t, w, m, coll, goke.SystemFn{OnInit: func(si *goke.SysInit) {
				q = si.NewQueryBuilder(&base, &course, &colliders).Build()
			}})
			for tick := range 60 {
				ecs.Tick(time.Second / 60)
				for q.All(); q.Next(); {
					cur := q.Cursor()
					for i, b := range base.Slice(cur) {
						if b.Pos.Size.X != 6 {
							continue
						}
						speed := course.Slice(cur)[i].Speed
						if !struckNow(colliders.Slice(cur)[i].Contacts()) {
							if speed != 96 {
								t.Fatalf("tick %d, nothing struck: the course holds %v, want 96",
									tick, speed)
							}
							continue
						}
						if speed > 48 || f32.Abs(speed*share-b.Vel.Value) > f32.Eps(96) {
							t.Errorf("tick %d, struck: the course holds %v and the unit goes at "+
								"%v, want the speed left, %v of the course", tick, speed,
								b.Vel.Value, share)
						}
						return
					}
				}
			}
			t.Fatal("the unit struck nothing in a second")
		})
	}
}

func struckNow(contacts []collision.Contact) bool {
	for _, c := range contacts {
		if !c.Resting && !c.Sensed {
			return true
		}
	}
	return false
}
