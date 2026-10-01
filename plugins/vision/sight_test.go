package vision_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/vision"
)

// Looking is the heading with Ahead and one to go by, Facing otherwise.
func TestSight_LookingFollowsTheHeadingOnlyAhead(t *testing.T) {
	east, north := geom.NewVec(1, 0), geom.NewVec(0, 1)
	for _, c := range []struct {
		sight   vision.Sight
		heading geom.Vec
		want    geom.Vec
	}{
		{vision.Sight{Facing: east}, north, east},
		{vision.Sight{Facing: east, Ahead: true}, north, north},
		{vision.Sight{Facing: east, Ahead: true}, geom.Vec{}, east},
	} {
		if got := c.sight.Looking(c.heading); got != c.want {
			t.Errorf("%+v heading %v looks %v, want %v", c.sight, c.heading, got, c.want)
		}
	}
}
