package vision_test

import (
	"math"
	"testing"

	"github.com/kjkrol/gram/plugins/vision"
)

// A forest 60 deep at τ = 0.5 costs 60 of reach on top of its depth, so the target 235 ahead
// costs 295: out of a 290 reach, in a 300 one. The forest itself is seen either way.
func forestAhead(observer *look) []spawn {
	return []spawn{
		{x: 0, y: 0, sight: observer, outline: true},
		{x: 100, y: 0, size: 60, tau: 0.5, layers: 1},
		{x: 240, y: 0},
	}
}

func TestScan_AVeilShortensTheReachAndIsSeen(t *testing.T) {
	_, seen, outlines := scene(t, forestAhead(eastward(math.Pi/8, 290))...)
	if seen[0].Count != 1 || seen[0].Dists[0] > 100 { // the forest's near edge, not the target
		t.Errorf("saw %v at %v through the forest at 290, want the forest alone", seen[0].IDs[:seen[0].Count], seen[0].Dists[:seen[0].Count])
	}
	mid := outlines[0].Depths[outlines[0].Count/2]
	if math.Abs(float64(mid)-230) > 2 { // 290 less the 60 the forest took
		t.Errorf("outline straight ahead reaches %.1f, want about 230", mid)
	}

	_, seen, _ = scene(t, forestAhead(eastward(math.Pi/8, 300))...)
	if seen[0].Count != 2 {
		t.Errorf("saw %d through the forest at 300, want the forest and the target", seen[0].Count)
	}
}

func TestScan_BlockersLookOverWhatIsOnOtherLayers(t *testing.T) {
	observer := eastward(math.Pi/8, 300)
	observer.Blockers = 2 // the forest is on layer 1
	_, seen, outlines := scene(t, forestAhead(observer)...)
	if seen[0].Count != 2 {
		t.Errorf("saw %d over the forest, want the forest and the target", seen[0].Count)
	}
	if mid := outlines[0].Depths[outlines[0].Count/2]; math.Abs(float64(mid)-235) > 1 {
		t.Errorf("outline straight ahead reaches %.1f, want the target at 235", mid)
	}
}

func TestScan_AWallCutsSightOnlyOnTheObserversBlockers(t *testing.T) {
	for name, tc := range map[string]struct {
		wall, blockers vision.Sight
		want           int
	}{
		"blockers zero, every wall cuts":       {want: 1},
		"wall on a blocking layer":             {blockers: vision.Sight{Blockers: 2}, want: 1},
		"wall on another layer is looked over": {blockers: vision.Sight{Blockers: 2}, want: 2},
	} {
		t.Run(name, func(t *testing.T) {
			observer := eastward(math.Pi/8, 300)
			observer.Blockers = tc.blockers.Blockers
			wallLayers := tc.blockers.Blockers
			if tc.want == 2 {
				wallLayers = 1
			}
			_, seen, _ := scene(t,
				spawn{x: 0, y: 0, sight: observer},
				spawn{x: 100, y: 0, size: 60, layers: wallLayers}, // tau 0: a wall
				spawn{x: 240, y: 0},
			)
			if int(seen[0].Count) != tc.want || seen[0].Dists[0] > 100 {
				t.Errorf("saw %d at %v, want %d with the wall first", seen[0].Count, seen[0].Dists[:seen[0].Count], tc.want)
			}
		})
	}
}
