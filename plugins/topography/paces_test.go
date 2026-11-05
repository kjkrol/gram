package topography_test

import (
	"testing"
	"time"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/topography/internal/topotest"
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

func TestAltitude_AUnitAsleepAtL6IsNotStoodOnTheGround(t *testing.T) {
	g := grid.DefaultGrids{}.Square(16, 2, 32)
	near, far := g.At(2, 1), g.At(14, 1)
	var walker kind.Of[topotest.Recruit]
	qw := topotest.NewQuasiWorldIn(t, g, func(cfg *world.Config) {
		cfg.Sectors = world.SectorsCfg{Buckets: 1}
		cfg.LOD = lod.Ladder{Sight: 200}
	}, false, 20, func(qw *topotest.QuasiWorld, units *board.Units[topotest.Recruit]) []kind.Entry {
		c := g.CellCenter(near)
		qw.World.Write().Observe(lookingAt(16*32, 2*32, c.X, c.Y))
		topotest.RaiseHills(qw.Topo.Write().Relief(), g, near, far)
		units.Define("walker", unit.Mover{Domain: cell.Land}, steering.Steering{MaxSpeed: 10})
		walker = units.Named("walker")
		return []kind.Entry{walker.Entry(topotest.Recruit{Start: near}),
			walker.Entry(topotest.Recruit{Start: far})}
	})
	for range 2 {
		qw.ECS.Tick(time.Second / 60)
	}
	hill := qw.Topo.Write().Relief().At(g.CellCenter(near))
	alts := map[float32]bool{}
	for _, z := range qw.Zs()[walker.ID()] {
		alts[z.Altitude] = true
	}
	if hill <= 0 || len(alts) != 2 || !alts[hill] || !alts[0] {
		t.Errorf("the walkers on the hills stand at %v, want one at L2 on its ground %v and one "+
			"asleep at 0, as spawned", alts, hill)
	}
}
