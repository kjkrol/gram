package view

import (
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
)

type row struct{ at [5]lod.Level }

func (row) Len() int  { return 5 }
func (row) Cols() int { return 5 }

func (row) Box(i int) geom.AABB {
	return geom.NewAABBAt(geom.NewVec(float32(i)*200, 0), 200, 1000)
}

func (row) Of(x, _ float32) int                      { return int(x / 200) }
func (r row) Level(_ camera.Camera, i int) lod.Level { return r.at[i] }
func (row) Ladder() lod.Ladder                       { return lod.Ladder{} }

func TestSeen_TheFrameBudgetsShiftLeavesASectorSeenAtL2Full(t *testing.T) {
	space, err := aabbworld.NewSpace(aabbworld.Config{Width: 1000, Height: 1000, BucketSize: 64})
	if err != nil {
		t.Fatal(err)
	}
	space.Rebuild(nil)
	cam := cameratest.NewFromSpace(1000, 1000, 0)
	cam.(camera.Shifted).SetLevelShift(2)
	v := NewSeen(cam, row{[5]lod.Level{lod.L2, lod.L3, lod.L4, lod.L5, lod.L6}})
	v.see(space)
	want := [5]lod.Level{lod.L2, lod.L5, lod.L5, lod.L5, lod.L6}
	for i, w := range want {
		if got := v.Level(float32(i)*200+100, 500); got != w {
			t.Errorf("a sector seen at L%d shifted by two is drawn at L%d, want L%d", i, got, w)
		}
	}
}
