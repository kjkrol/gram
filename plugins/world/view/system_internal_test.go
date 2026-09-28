package view

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/uid"
)

// viewOf fills a 1000x1000 world of the given edges with one 10x10 box per position, refreshes a
// View over cam once, and returns it with the ids in position order.
func viewOf(t *testing.T, edges aabbworld.Edges, cam camera.Camera, at ...geom.Vec) (*View, []uid.UID64) {
	t.Helper()
	space, err := aabbworld.NewSpace(aabbworld.Config{Width: 1000, Height: 1000, Edges: edges, BucketSize: 64})
	if err != nil {
		t.Fatal(err)
	}
	var items []aabbworld.Item
	var ids []uid.UID64
	for i, p := range at {
		id := uid.UID64(i + 1)
		items = append(items, aabbworld.Item{ID: id, Box: space.WrapAABB(geom.NewAABBAt(p, 10, 10))})
		ids = append(ids, id)
	}
	space.Rebuild(items)

	views := []*View{New(cam.Bounds)}
	NewSystem(space, &views, 1000, 1000).Update(nil, time.Second)
	return views[0], ids
}

func TestViewSystem_AViewOnAQuarterHoldsOnlyWhatIsInIt(t *testing.T) {
	quarters := []geom.Vec{geom.NewVec(100, 100), geom.NewVec(700, 100), geom.NewVec(100, 700), geom.NewVec(700, 700)}
	cam := icamera.NewFromSpace(1000, 1000, 0, geom.NewAABBAt(geom.NewVec(0, 0), 500, 500))

	v, ids := viewOf(t, 0, cam, quarters...)

	if !v.Culled {
		t.Fatal("a View on a quarter of the world is not culled")
	}
	for i, id := range ids {
		if got, want := v.Contains(id), i == 0; got != want {
			t.Errorf("Contains(%v at %v) = %v, want %v", id, quarters[i], got, want)
		}
	}
	if v.In.Len() != 1 {
		t.Errorf("the View holds %d entities, want 1", v.In.Len())
	}
}

func TestViewSystem_AViewOnTheWholeWorldSeesEverythingUnqueried(t *testing.T) {
	v, ids := viewOf(t, 0, icamera.NewFromSpace(1000, 1000, 0), geom.NewVec(100, 100), geom.NewVec(700, 700))
	if v.Culled {
		t.Fatal("a View on the whole world is culled")
	}
	if v.In.Len() != 0 {
		t.Errorf("the View holds %d entities, want none marked when it sees everything", v.In.Len())
	}
	for _, id := range ids {
		if !v.Contains(id) {
			t.Errorf("Contains(%v) = false on a View of the whole world", id)
		}
	}
}

func TestViewSystem_SeesABoxAcrossTheSeamFromEitherSide(t *testing.T) {
	onSeam := geom.NewVec(-5, 100) // wraps: its main piece sits at the right edge, a piece at the left
	for name, cam := range map[string]camera.Camera{
		"right edge": icamera.NewFromSpace(1000, 1000, aabbworld.Torus, geom.NewAABBAt(geom.NewVec(800, 0), 200, 200)),
		"left edge":  icamera.NewFromSpace(1000, 1000, aabbworld.Torus, geom.NewAABBAt(geom.NewVec(0, 0), 200, 200)),
	} {
		v, ids := viewOf(t, aabbworld.Torus, cam, onSeam)
		if !v.Contains(ids[0]) {
			t.Errorf("a camera at the %s does not see the box straddling the seam", name)
		}
	}
}
