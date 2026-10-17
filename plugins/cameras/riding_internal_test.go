package cameras

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

func TestRiding_ReadsAndWritesTheEntityTheKeeperFollows(t *testing.T) {
	ecs := goke.New()
	var id uid.UID64
	var base goke.Comp[world.Base]
	var z goke.Comp[world.Z]
	var eye goke.Comp[world.Eye]
	var driven goke.Comp[steering.Driven]
	ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		f := si.NewFactory(&base, &z, &eye, &driven)
		f.Create(1)
		f.Next()
		id = f.Cursor.IDs[0]
		base.Slice(&f.Cursor)[0] = world.Base{
			Pos: world.Position{AABB: plane.NewAABB(geom.NewVec(100, 200), 10, 20)},
			Vel: world.Velocity{Dir: geom.NewVec(3, 4)},
		}
		z.Slice(&f.Cursor)[0] = world.Z{Altitude: 5, Height: 2}
		eye.Slice(&f.Cursor)[0] = world.Eye{Height: 1, Angle: 0.5, Tilt: 0.1}
		driven.Slice(&f.Cursor)[0] = steering.Driven{Turn: -1}
	}})
	r := &riding{}
	ecs.RegSys(r)

	got, ok := r.Of(id)
	if !ok || got.X != 105 || got.Y != 210 || got.Alt != 5 || got.Level != 6 {
		t.Errorf("the entity is %+v (%v), want its middle (105, 210) at 5, its eye at 6", got, ok)
	}
	if math.Abs(got.Dir.X-0.6) > 1e-9 || math.Abs(got.Dir.Y-0.8) > 1e-9 {
		t.Errorf("it faces %v, want (0.6, 0.8): its way of length 1", got.Dir)
	}
	if !got.Eyed || got.Across != 0.5 || got.Tilt != 0.1 || !got.Turning {
		t.Errorf("its eye is %+v, want an eye 0.5 across tilted 0.1, a hand turning it", got)
	}
	r.Drive(id, geom.NewVec(1, 0), true, 0.25)
	r.Tilt(id, -0.3)
	got, _ = r.Of(id)
	cur := func() *goke.Cursor {
		q := r.query
		q.Seek(id)
		return q.Cursor()
	}()
	d := r.driven.At(cur)
	if d.Look != geom.NewVec(1, 0) || !d.Flown || d.Climb != 0.25 || d.Turn != -1 {
		t.Errorf("driven %+v, want the eye's part written and the hand's kept", *d)
	}
	if got.Tilt != -0.3 {
		t.Errorf("its eye is tilted %v, want -0.3", got.Tilt)
	}
	if _, ok := r.Of(id + 1000); ok {
		t.Error("an entity that is not there is followed")
	}
}
