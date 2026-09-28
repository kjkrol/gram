package vision_test

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
)

// Observers enough to be scanned on several goroutines at once see what they see one by one: the
// same entities at the same distances, the same outlines — on flat ground and on a hill, with
// see-through things and heights among what they see.
func TestScan_ObserversScannedTogetherSeeWhatTheyDoOneByOne(t *testing.T) {
	for name, r := range map[string]*relief{"flat": nil, "hill": {ground: plateau{}, step: 10}} {
		var spawns []spawn
		for i := range 40 {
			s := spawn{x: float64(40 + (i%8)*50), y: float64(40 + (i/8)*50), sight: eastward(math.Pi/3, 400), outline: i%2 == 0}
			if r != nil {
				s.z, s.sight.Eye.Height = z(0, 2), 1.5
			}
			spawns = append(spawns, s)
		}
		spawns = append(spawns, spawn{x: 300, y: 90, size: 20, tau: 0.5}, spawn{x: 420, y: 140, size: 30}, spawn{x: 200, y: 200, layers: world.Layers(2)})
		if r != nil {
			spawns[40].z, spawns[41].z, spawns[42].z = z(0, 20), z(30, 5), z(0, 1)
		}
		ids1, seen1, out1 := sceneWith(t, r, 1, spawns...)
		ids, seen, out := sceneWith(t, r, 0, spawns...)
		if len(ids) != len(ids1) || len(seen) != 40 || len(out) != 20 {
			t.Fatalf("%s: %d observers with %d outlines scanned together, %d one by one", name, len(seen), len(out), len(seen1))
		}
		saw := 0
		for i := range seen {
			if ids[i] != ids1[i] {
				t.Fatalf("%s: observer %d is %v together, %v one by one", name, i, ids[i], ids1[i])
			}
			if seen[i] != seen1[i] {
				t.Errorf("%s: observer %d saw %v together, %v one by one", name, i, sighted(seen[i]), sighted(seen1[i]))
			}
			saw += int(seen[i].Count)
		}
		if saw == 0 {
			t.Errorf("%s: nobody saw anything: the scene proves nothing", name)
		}
		for i := range out {
			if out[i] != out1[i] {
				t.Errorf("%s: outline %d differs scanned together", name, i)
			}
		}
	}
}

// sighted is what a Sighted holds, as far as it counts.
func sighted(s vision.Sighted) any {
	return struct {
		ids   any
		dists any
	}{s.IDs[:s.Count], s.Dists[:s.Count]}
}
