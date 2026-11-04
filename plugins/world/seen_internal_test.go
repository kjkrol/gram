package world

import (
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world/view"
)

// the rig's world of ladder with entities at xs, seen by a camera from above showing x 0 to
// 1600, its middle at (800, 256), the world's one observer
func seeing(t *testing.T, ladder lod.Ladder, xs ...float32) (*sectorRig, *view.View) {
	t.Helper()
	r := sectorWorld(t, ladder)
	cam := cameratest.NewFromSpace(4096, 512, 0)
	cam.SetViewport(1600, 512)
	cam.MoveTo(0, 0)
	r.watch.cams = []camera.Camera{cam}
	v := r.p.write.NewView(cam)
	r.spawn(xs...)
	r.run()
	return r, v
}

// the level of each of the rig's eight sectors, left to right
func levels(v *view.View) [8]lod.Level {
	out := [8]lod.Level{4, 4, 4, 4, 4, 4, 4, 4}
	v.Sectors(func(_ int, box geom.AABB, l lod.Level) { out[int(box.TopLeft.X)/512] = l })
	return out
}

func TestSeen_LaysTheSectorsAtTheirLevelsWithinTheSight(t *testing.T) {
	r, v := seeing(t, lod.Ladder{Sight: 1000, Fog: 300})
	// the fourth is 736 off, in the fog; the fifth 1248, past the sight
	if got, want := levels(v), [8]lod.Level{0, 0, 0, 0, 4, 4, 4, 4}; got != want {
		t.Errorf("levels %v, want %v", got, want)
	}
	reported := map[string]string{}
	r.p.read.Seen().Report(func(label, value string) { reported[label] = value })
	if got, want := reported["Sectors seen"], "L0 4\nL1 0\nL2 0\nL3 0"; got != want {
		t.Errorf("the panel says %q, want %q", got, want)
	}
	_, coarse := seeing(t, lod.Ladder{Scales: [lod.Levels - 1]float32{2}, Sight: 1000, Fog: 300})
	if got, want := levels(coarse), [8]lod.Level{1, 1, 1, 1, 4, 4, 4, 4}; got != want {
		t.Errorf("a pixel a unit under a threshold of 2: levels %v, want %v", got, want)
	}
}

func TestSeen_WithoutASightIsWhatThePictureCovers(t *testing.T) {
	_, v := seeing(t, lod.Ladder{})
	// the window reaches 1600, into the fourth
	if got, want := levels(v), [8]lod.Level{0, 0, 0, 0, 4, 4, 4, 4}; got != want {
		t.Errorf("levels %v, want %v", got, want)
	}
}

func TestSeen_HoldsTheEntitiesOfTheSectorsSeenInThePictureFoggedOnTheWayToTheSight(t *testing.T) {
	r, v := seeing(t, lod.Ladder{Sight: 1000, Fog: 300}, 100, 900, 1550, 1700, 3000)
	seen := map[float32]bool{}
	for r.all.All(); r.all.Next(); {
		cur := r.all.Cursor()
		for i, id := range cur.IDs {
			seen[r.base.Slice(cur)[i].Pos.TopLeft.X] = v.Contains(id)
		}
	}
	// 1700 lies in the fourth sector, seen, but past the picture's edge at 1600
	if !seen[100] || !seen[900] || !seen[1550] || seen[1700] || seen[3000] {
		t.Errorf("seen %v, want all but the one past the picture and the one past the sight", seen)
	}
	if l := v.Level(1555, 255); l != lod.L0 {
		t.Errorf("the one at 1555 is at %v, want L0", l)
	}
	if l := v.Level(3005, 255); l != lod.L4 {
		t.Errorf("the one at 3005, past the sight, is at %v, want L4", l)
	}
	if fog := v.Fog(905, 255); fog != 0 {
		t.Errorf("105 off the fog is %v, want none", fog)
	}
	if fog := v.Fog(1555, 255); fog <= 0 || fog >= 1 {
		t.Errorf("755 off, into the fog from 700 to 1000, the fog is %v, want some", fog)
	}
}

func TestSeen_TheSightIsTheLaddersTimesTheCamerasOwn(t *testing.T) {
	r, v := seeing(t, lod.Ladder{Sight: 1000, Fog: 300})
	cam := r.watch.cams[0]
	cam.(interface{ See(factor float32) }).See(2)
	r.run()
	if got := r.p.read.Sight(cam); got != 2000 {
		t.Errorf("the camera seeing twice as far sees %v, want 2000", got)
	}
	if got := v.Fogging(); got != [4]float32{800, 256, 1700, 2000} {
		t.Errorf("the fog %v, want from (800, 256), 1700 to 2000", got)
	}
}

func TestSeen_HoldsTheEntitiesAcrossTheSeamOfATorus(t *testing.T) {
	r := sectorWorldOf(t, aabbworld.Torus, 2, lod.Ladder{})
	cam := cameratest.NewFromSpace(4096, 512, aabbworld.Torus)
	cam.SetViewport(600, 512)
	cam.MoveTo(3800, 0)
	r.watch.cams = []camera.Camera{cam}
	v := r.p.write.NewView(cam)
	r.spawn(100, 3900, 2000)
	r.run()
	seen := map[float32]bool{}
	for r.all.All(); r.all.Next(); {
		cur := r.all.Cursor()
		for i, id := range cur.IDs {
			seen[r.base.Slice(cur)[i].Pos.TopLeft.X] = v.Contains(id)
		}
	}
	// the picture runs from 3800 past the seam to 304
	if !seen[100] || !seen[3900] || seen[2000] {
		t.Errorf("seen %v, want the one at 3900 and the one across the seam at 100", seen)
	}
}

func TestSeen_ASectorOffThePictureIsNothingToTheViewWhileItIsTicked(t *testing.T) {
	r, v := seeing(t, lod.Ladder{Sight: 3000}, 100, 2500)
	// the picture ends at 1600; the fifth sector, from 2048, lies within the sight
	if l := v.Level(2505, 255); l != lod.L4 {
		t.Errorf("the one at 2505, off the picture, is drawn at %v, want L4", l)
	}
	if !same(r.at(lod.L0), 100, 2500) || !r.held(2500) {
		t.Errorf("at L0 %v, the one off the picture held %v; want both ticked at L0",
			r.at(lod.L0), r.held(2500))
	}
}

func TestSeen_AShiftedCameraDrawsItsSectorsCoarserWhileTheirTickStays(t *testing.T) {
	r, v := seeing(t, lod.Ladder{Sight: 1000, Fog: 300}, 100, 900)
	cam := r.watch.cams[0]
	cam.(camera.Shifted).SetLevelShift(2)
	r.run()
	if got, want := levels(v), [8]lod.Level{0, 0, 0, 0, 4, 4, 4, 4}; got != want {
		t.Errorf("shifted by two, levels %v, want %v: L0 left full, past the sight L4", got, want)
	}
	if l := v.Level(105, 255); l != lod.L0 {
		t.Errorf("the one at 105 is drawn at %v, want L0", l)
	}
	seen := seenSectors{r.p.module.sectors}
	if !same(r.at(lod.L0), 100, 900) || seen.Level(cam, 0) != lod.L0 {
		t.Errorf("at L0 %v, the first sector seen at %v; want both ticked and seen at L0 still",
			r.at(lod.L0), seen.Level(cam, 0))
	}

	// hiding is the ladder's: a sector drawn stays drawn, at L3 at most
	for _, c := range []struct {
		scales [lod.Levels - 1]float32
		from   lod.Level
		shift  lod.Level
	}{
		{[lod.Levels - 1]float32{2}, lod.L1, 2},
		{[lod.Levels - 1]float32{8, 4, 2}, lod.L3, 1},
		{[lod.Levels - 1]float32{8, 4}, lod.L2, 2},
	} {
		r, v = seeing(t, lod.Ladder{Scales: c.scales, Sight: 1000}, 100)
		cam = r.watch.cams[0]
		cam.(camera.Shifted).SetLevelShift(c.shift)
		r.run()
		if got, want := levels(v), [8]lod.Level{3, 3, 3, 3, 4, 4, 4, 4}; got != want {
			t.Errorf("%v shifted by %v, levels %v, want %v", c.from, c.shift, got, want)
		}
		if !same(r.at(c.from), 100) || !seenAt(r, v, 100) {
			t.Errorf("%v shifted by %v: at %v %v, the one at 100 in view %v; want it ticked at "+
				"%v and drawn", c.from, c.shift, c.from, r.at(c.from), seenAt(r, v, 100), c.from)
		}
	}
}

// whether v holds the rig's entity at x
func seenAt(r *sectorRig, v *view.View, x float32) bool {
	for r.all.All(); r.all.Next(); {
		cur := r.all.Cursor()
		for i, id := range cur.IDs {
			if r.base.Slice(cur)[i].Pos.TopLeft.X == x {
				return v.Contains(id)
			}
		}
	}
	return false
}
