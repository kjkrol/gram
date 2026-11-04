package bullet_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/bullet"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/uid"
)

// the world's one observer, a camera from above looking at (x, y)
type eye struct{ cam camera.Camera }

var _ sector.Observers = eye{}

func lookingAt(w, h uint32, x, y float32) eye {
	cam := cameratest.NewFromSpace(w, h, 0)
	cam.SetViewport(2, 2)
	cam.CenterOn(x, y, 0)
	return eye{cam}
}

func (e eye) Observers(each func(cam camera.Camera)) { each(e.cam) }

func TestFlight_AShotKeepsItsSectorsKeenTillItComesToRest(t *testing.T) {
	r := newRig(t, scene{pieces: []piece{{x: 100, y: 100, size: 20, facing: east}}})
	r.arms.Define("short", bullet.Body{Size: 4, Speed: 600, Range: 100, Lands: true})
	ammo := r.arms.Named("short")
	r.start()
	r.w.Write().Carrier().PutFrom(r.ids[0], bullet.Shoot{Ammo: ammo})
	r.tick(1)
	if k := r.theShot().keen; k == nil || *k != (sector.Keen{Level: lod.L2}) {
		t.Fatalf("the shot flying is keen %v, want at L2", k)
	}
	r.tick(11)
	if shot := r.theShot(); !shot.flight.Landed || shot.keen != nil {
		t.Errorf("the shot landed %v is keen %v, want it lying, keen no more", shot.flight.Landed,
			shot.keen)
	}
}

func TestShoot_APlayerFiresFromNoSelectedUnitAsleepAtL4(t *testing.T) {
	r := newRig(t, scene{
		width:  2000,
		height: 400,
		pieces: []piece{
			{x: 50, y: 50, size: 20, facing: east, selectable: true, selected: true, owner: 1},
			{x: 1900, y: 50, size: 20, facing: east, selectable: true, selected: true, owner: 1},
		},
		sectors:   world.SectorsCfg{Buckets: 1},
		ladder:    lod.Ladder{Sight: 300},
		observers: lookingAt(2000, 400, 60, 60),
	})
	ammo := round(r, true)
	r.start()
	r.tick(2)
	r.w.Write().Carrier().Put(1, bullet.Shoot{Ammo: ammo})
	r.tick(1)
	if shot := r.theShot(); shot.flight.Shooter != r.ids[0] {
		t.Errorf("the shot is %v's, want the unit at L0's, %v", shot.flight.Shooter, r.ids[0])
	}
}

func TestShoot_AShotAimedAtAnAggregateKeepsItsSectorsKeenAtL1(t *testing.T) {
	r := newRig(t, scene{pieces: []piece{
		{x: 100, y: 100, size: 20, facing: east},
		{x: 100, y: 300, size: 20, facing: east},
	}})
	kind.Define[piece](r.w.Kinds(), "whole", kind.Spec{
		comp.Load(func(p piece) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(p.x, p.y), p.size, p.size)}
		}),
		comp.Const(world.Velocity{}),
		comp.Const(world.Aggregate{}),
	})
	r.w.Write().Seed(kind.Named[piece](r.w.Kinds(), "whole").Entry(piece{x: 300, y: 100, size: 20}))
	r.arms.Define("short", bullet.Body{Size: 4, Speed: 60, Range: 300, Lands: true})
	ammo := r.arms.Named("short")
	r.start()
	whole := r.read(func(f found) bool { return f.base.Pos.TopLeft.X == 300 })[0].id
	for _, c := range []struct {
		at   uid.UID64
		want lod.Level
	}{
		{whole, lod.L1},
		{r.ids[1], lod.L2},
	} {
		shoot := bullet.Shoot{Ammo: ammo}
		shoot.Aim(c.at)
		r.w.Write().Carrier().PutFrom(r.ids[0], shoot)
		r.tick(1)
		shots := r.shots()
		if k := shots[len(shots)-1].keen; k == nil || *k != (sector.Keen{Level: c.want}) {
			t.Errorf("a shot aimed at %v is keen %v, want at %v", c.at, k, c.want)
		}
	}
}
