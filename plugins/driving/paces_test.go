package driving_test

import (
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/driving"
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

func TestHand_DrivesNoUnitAsleepAtL6(t *testing.T) {
	f := newFieldIn(t, nil, func(cfg *world.Config) sector.Observers {
		cfg.Sectors = world.SectorsCfg{Buckets: 1}
		cfg.LOD = lod.Ladder{Sight: 64}
		return lookingAt(320, 96, 80, 48)
	}, walker{start: 12, owner: 1, selected: true}, walker{start: 18, owner: 1, selected: true})
	near, far := f.ids[0], f.ids[1]
	f.tick(2)
	before := f.read()
	for range 3 {
		f.give(1, driving.Ahead{Camera: f.cams[1]})
		f.tick(1)
	}
	after := f.read()
	if after[near].at == before[near].at || after[near].driven == nil {
		t.Errorf("at L2 the unit stands at %v, Driven %v; want it driven on", after[near].at,
			after[near].driven)
	}
	if after[far].at != before[far].at || after[far].driven != nil {
		t.Errorf("asleep the unit went from %v to %v, Driven %v; want it left alone",
			before[far].at, after[far].at, after[far].driven)
	}
}
