package bullet_test

import (
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/bullet"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
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
