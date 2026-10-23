package traffic

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
)

func TestLookahead(t *testing.T) {
	have := geom.NewVec(0, 0)
	route := []geom.Vec{geom.NewVec(10, 0), geom.NewVec(10, 10)}
	for name, tc := range map[string]struct {
		reach float64
		want  geom.Vec
	}{
		"no reach aims at the first waypoint":     {0, geom.NewVec(10, 0)},
		"within the first segment":                {4, geom.NewVec(4, 0)},
		"past the bend":                           {14, geom.NewVec(10, 4)},
		"beyond the route ends at its last point": {40, geom.NewVec(10, 10)},
	} {
		if got := lookahead(have, route, tc.reach); got != tc.want {
			t.Errorf("%s: lookahead(reach %v) = %v, want %v", name, tc.reach, got, tc.want)
		}
	}
	if got := lookahead(have, nil, 5); got != have {
		t.Errorf("an empty route gave %v, want the position itself", got)
	}
}

func TestTurnFactor(t *testing.T) {
	east, west, north := geom.NewVec(1, 0), geom.NewVec(-1, 0), geom.NewVec(0, 1)
	for name, tc := range map[string]struct {
		heading, dir geom.Vec
		want         float64
	}{
		"straight on keeps full speed": {east, east, 1},
		"a right angle crawls":         {east, north, TurnCrawl},
		"a U-turn crawls":              {east, west, TurnCrawl},
		"no heading yet keeps full":    {geom.Vec{}, east, 1},
	} {
		if got := turnFactor(tc.heading, tc.dir); got != tc.want {
			t.Errorf("%s: turnFactor = %v, want %v", name, got, tc.want)
		}
	}
}

func TestPassed_WithinTheLookaheadCountsAsPassed(t *testing.T) {
	from, w := geom.NewVec(0, 0), geom.NewVec(10, 0)
	if passed(geom.NewVec(7, 0), w, from, 0) {
		t.Error("3 short of the plane with no reach counts as passed")
	}
	if !passed(geom.NewVec(7, 0), w, from, 4) {
		t.Error("3 short of the plane but within a reach of 4 does not count as passed")
	}
	if !passed(geom.NewVec(12, 3), w, from, 0) {
		t.Error("beyond the plane does not count as passed")
	}
}
