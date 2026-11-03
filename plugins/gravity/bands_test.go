package gravity_test

import (
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
)

// two observers looking at (x, y) over a world w by h: the one a pixel a world unit, the other a
// quarter of a pixel seeing twice as far; under a ladder seeing from half a pixel a world unit,
// L0 within its sight of the point, L1 within twice that, L4 beyond
type observers [2]camera.Camera

var _ sector.Observers = (*observers)(nil)

func newObservers(w, h uint32, x, y float32) *observers {
	o := &observers{cameratest.NewFromSpace(w, h, 0), cameratest.NewFromSpace(w, h, 0)}
	for _, cam := range o {
		cam.SetViewport(2, 2)
	}
	o[1].ZoomOut(4, 0, 0)
	o[1].(interface{ See(factor float32) }).See(2)
	o.look(x, y)
	return o
}

// has both observers look at (x, y)
func (o *observers) look(x, y float32) {
	for _, cam := range o {
		cam.CenterOn(x, y, 0)
	}
}

func (o *observers) Observers(each func(cam camera.Camera)) {
	for _, cam := range o {
		each(cam)
	}
}

// sectors 32 a side: the observers' own at L0, the two beside it at L1, the corner asleep
func TestFall_AtL1ComesDownAsFarInAsLong(t *testing.T) {
	r := newRig(t, scene{
		bodies:    []body{{x: 15, y: 15, alt: 10}, {x: 36, y: 15, alt: 10}},
		sectors:   world.SectorsCfg{Buckets: 1},
		ladder:    lod.Ladder{Scales: [lod.Levels - 1]float32{0.5}, Sight: 15},
		observers: newObservers(40, 40, 5, 5),
	})
	for range 8 {
		r.tick()
	}
	near, far := r.of(0).z.Altitude, r.of(1).z.Altitude
	if near >= 10 || f32.Abs(near-far) > f32.Eps(near) {
		t.Errorf("fallen to %v at L0 and %v at L1, want the same below 10", near, far)
	}
}

// sectors 32 a side: the observers' own at L0, the corner asleep
func TestGive_ABodyAsleepAtL4IsGivenNoDrift(t *testing.T) {
	r := newRig(t, scene{
		bodies:    []body{{x: 15, y: 15}, {x: 36, y: 36}},
		sectors:   world.SectorsCfg{Buckets: 1},
		ladder:    lod.Ladder{Scales: [lod.Levels - 1]float32{0.5}, Sight: 15},
		observers: newObservers(40, 40, 5, 5),
	})
	for range 2 {
		r.tick()
	}
	drifts := func(k int) bool {
		if !r.q.Seek(r.ids[k]) {
			t.Fatalf("body %d is gone", k)
		}
		return r.drift.At(r.q.Cursor()) != nil
	}
	if !drifts(0) || drifts(1) {
		t.Errorf("a drift at L0 %v, asleep %v; want only the first", drifts(0), drifts(1))
	}
}

// sectors 32 a side: the observers' own at L0, the one beside it at L1
func TestPlane_AtL1ABodyComesDownAsAtL0(t *testing.T) {
	r := newRig(t, scene{
		level:     room,
		bodies:    []body{{x: 15, y: 13}, {x: 36, y: 13}},
		sectors:   world.SectorsCfg{Buckets: 1},
		ladder:    lod.Ladder{Scales: [lod.Levels - 1]float32{0.5}, Sight: 15},
		observers: newObservers(40, 40, 5, 5),
	})
	for range tps {
		r.tick()
	}
	near, far := r.of(0).at.Y, r.of(1).at.Y
	// motion moves a body at L1 two steps in one, by the drift of the first: a second's pull
	// of 10 leaves them less than a step's run apart
	if near <= 13 || f32.Abs(near-far) > 10.0/tps {
		t.Errorf("come down to %v at L0 and %v at L1, want the same below 13", near, far)
	}
}
