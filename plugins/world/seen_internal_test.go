package world

import (
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world/view"
)

// a camera from above over the rig's world showing x 0 to 1600, its middle at (800, 256), the
// world's bands following it
func seeing(t *testing.T, r *sectorRig, d camera.Detail) (camera.Camera, *view.View) {
	t.Helper()
	cam := cameratest.NewFromSpace(4096, 512, 0)
	cam.SetViewport(1600, 512)
	cam.MoveTo(0, 0)
	cam.(interface{ SetDetail(camera.Detail) }).SetDetail(d)
	r.anchor.at[0] = geom.NewVec(800, 256)
	v := r.p.ViewFor(cam)
	r.run()
	return cam, v
}

// the level of each of the rig's eight sectors, left to right
func levels(v *view.View) [8]view.Level {
	var out [8]view.Level
	v.Sectors(func(_ int, box geom.AABB, l view.Level) { out[int(box.TopLeft.X)/512] = l })
	return out
}

func TestSeen_LaysTheSectorsAtTheirLevelsWithinTheSight(t *testing.T) {
	r := sectorWorld(t, twoBands...)
	_, v := seeing(t, r, camera.Detail{Sight: 1000, Fog: 300})
	// the fourth is 736 off, in the fog; the fifth 1248, past the sight
	e, f, h := view.Exact, view.Fogged, view.Hidden
	if got, want := levels(v), [8]view.Level{e, e, e, f, h, h, h, h}; got != want {
		t.Errorf("levels %v, want %v", got, want)
	}
	reported := map[string]string{}
	r.p.Seen().Report(func(label, value string) { reported[label] = value })
	if got, want := reported["Sectors seen"], "exact 3\ncoarse 0\nfog 1"; got != want {
		t.Errorf("the panel says %q, want %q", got, want)
	}
	r2 := sectorWorld(t, twoBands...)
	_, coarse := seeing(t, r2, camera.Detail{Sight: 1000, Fog: 300, Coarse: 2})
	c := view.Coarse
	if got, want := levels(coarse), [8]view.Level{c, c, c, f, h, h, h, h}; got != want {
		t.Errorf("a pixel a unit under a threshold of 2: levels %v, want %v", got, want)
	}
}

func TestSeen_WithoutASightIsWhatThePictureCoversAndNeverBeyondTheBands(t *testing.T) {
	r := sectorWorld(t, twoBands...)
	_, v := seeing(t, r, camera.Detail{})
	// the window reaches 1600, into the fourth; the bands round 800 end before the fifth
	e, h := view.Exact, view.Hidden
	if got, want := levels(v), [8]view.Level{e, e, e, e, h, h, h, h}; got != want {
		t.Errorf("levels %v, want %v", got, want)
	}
}

func TestSeen_HoldsTheEntitiesOfTheSectorsSeenInThePictureFoggedOnTheWayToTheSight(t *testing.T) {
	r := sectorWorld(t, twoBands...)
	r.spawn(100, 900, 1550, 1700, 3000)
	_, v := seeing(t, r, camera.Detail{Sight: 1000, Fog: 300})
	seen := map[float64]bool{}
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
	if l := v.Level(1555, 255); l != view.Fogged {
		t.Errorf("the one at 1555 is %v, want fogged", l)
	}
	if fog := v.Fog(905, 255); fog != 0 {
		t.Errorf("105 off the fog is %v, want none", fog)
	}
	if fog := v.Fog(1555, 255); fog <= 0 || fog >= 1 {
		t.Errorf("755 off, into the fog from 700 to 1000, the fog is %v, want some", fog)
	}
}

func TestSeen_HoldsTheEntitiesAcrossTheSeamOfATorus(t *testing.T) {
	r := sectorWorldOf(t, aabbworld.Torus, twoBands...)
	r.spawn(100, 3900, 2000)
	cam := cameratest.NewFromSpace(4096, 512, aabbworld.Torus)
	cam.SetViewport(600, 512)
	cam.MoveTo(3800, 0)
	r.anchor.at[0] = geom.NewVec(4000, 256)
	v := r.p.ViewFor(cam)
	r.run()
	seen := map[float64]bool{}
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
