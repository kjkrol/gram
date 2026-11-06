package selection

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// bannered is the harness with ten units in a row at (600, 600) the camera draws in one banner
// over (300, 300) to (304, 304), the units' ids
func bannered(t *testing.T) (*harness, []*uid.UID64) {
	t.Helper()
	h := newHarness(t)
	var ids []*uid.UID64
	for i := range 10 {
		ids = append(ids, h.seed(600+float32(i)*12, 600, 10))
	}
	h.start()
	h.sys.banners = func(cam camera.Camera, dst []world.Banner) []world.Banner {
		if cam != h.cam {
			return dst
		}
		b := world.Banner{Count: 10, Center: geom.NewVec(302, 302),
			Box: geom.NewAABBAt(geom.NewVec(300, 300), 4, 4)}
		for _, id := range ids {
			b.IDs = append(b.IDs, *id)
		}
		return append(dst, b)
	}
	return h, ids
}

func TestSystem_Update_ClickOnABannerSelectsItsUnits(t *testing.T) {
	h, ids := bannered(t)
	h.click(305, 299, false)
	for i, id := range ids {
		if !h.isSelected(*id) {
			t.Errorf("unit %d of the banner clicked unselected, want every one selected", i)
		}
	}
	h.click(100, 100, false)
	for i, id := range ids {
		if h.isSelected(*id) {
			t.Errorf("unit %d selected after a click beside the banner, want none", i)
		}
	}
}

func TestSystem_Update_ClickWhereAUnitDrawnInABannerStandsSelectsNothing(t *testing.T) {
	h, ids := bannered(t)
	h.click(605, 605, false)
	if h.isSelected(*ids[0]) {
		t.Error("a unit drawn in a banner selected where it stands, want it hit by its banner " +
			"alone")
	}
}

func TestSystem_Update_ABannerSelectsOnlyTheUnitsItMaySelect(t *testing.T) {
	h := newHarness(t)
	mine := h.seed(600, 600, 10)
	theirs := h.seedOwned(612, 600, 10, h.local.ID+1)
	h.start()
	h.sys.banners = func(_ camera.Camera, dst []world.Banner) []world.Banner {
		return append(dst, world.Banner{Count: 2, IDs: []uid.UID64{*mine, *theirs},
			Box: geom.NewAABBAt(geom.NewVec(300, 300), 4, 4)})
	}
	h.click(302, 302, false)
	if !h.isSelected(*mine) || h.isSelected(*theirs) {
		t.Errorf("selected %v of the player's and %v of another's, want the player's alone",
			h.isSelected(*mine), h.isSelected(*theirs))
	}
}
