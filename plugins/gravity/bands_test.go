package gravity_test

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
)

var _ sector.Anchors = anchor{}

// sectors 32 a side: the anchor's own, the two beside it in the second band, the corner beyond
func TestFall_InAFartherBandComesDownAsFarInAsLong(t *testing.T) {
	r := newRig(t, scene{
		bodies: []body{{x: 15, y: 15, alt: 10}, {x: 36, y: 15, alt: 10}},
		sectors: world.SectorsCfg{
			Buckets: 1,
			Bands: []sector.Band{
				{Reach: 1},
				{Reach: 30, Pace: 4},
			},
		},
		anchor: geom.NewVec(5, 5),
	})
	for range 8 {
		r.tick()
	}
	near, far := r.of(0).z.Altitude, r.of(1).z.Altitude
	if near >= 10 || math.Abs(near-far) > 1e-9 {
		t.Errorf("fallen to %v near and %v in the second band, want the same below 10", near, far)
	}
}
