package dialog_test

import (
	"testing"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/lod"
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

func TestConversation_AsleepAtL6IsLeftAsItStandsTillItWakes(t *testing.T) {
	cfg := plain
	cfg.Sectors = world.SectorsCfg{Buckets: 1}
	cfg.LOD = lod.Ladder{Sight: 400}
	at := lookingAt(1000, 1000, 100, 100)
	r := newRigIn(t, cfg, at)
	r.begin(r.host, r.guest)
	at.cam.CenterOn(990, 990, 0)
	r.tick(2)
	r.w.Write().Carrier().PutFrom(r.guest, world.Despawn{})
	r.tick(2)
	if got := r.line(r.host); got != "Hello, traveller!" {
		t.Errorf("asleep, its listener gone, the host says %q; want the greeting still", got)
	}
	at.cam.CenterOn(100, 100, 0)
	r.tick(2)
	if got := r.line(r.host); got != "" {
		t.Errorf("woken, the host says %q; want the conversation over, its listener gone", got)
	}
}
