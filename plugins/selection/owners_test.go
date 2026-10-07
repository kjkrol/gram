package selection

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/control"
)

// everywhere is a box over the whole test world.
var everywhere = geom.AABB{TopLeft: geom.NewVec(0, 0), BottomRight: geom.NewVec(1000, 1000)}

// A player's drag over everything selects its own units alone: not another player's, not those
// nobody owns.
func TestSelect_APlayerSelectsItsOwnUnitsAlone(t *testing.T) {
	h := newHarness(t)
	mine := h.seed(100, 100, 10)
	theirs := h.seedOwned(200, 200, 10, 2)
	nobodys := h.seedOwned(300, 300, 10, control.Nobody)
	h.start()
	h.drag(0, 0, 999, 999, false)
	if !h.isSelected(*mine) || h.isSelected(*theirs) || h.isSelected(*nobodys) {
		t.Errorf("mine %v, another player's %v, nobody's %v selected; want mine alone",
			h.isSelected(*mine), h.isSelected(*theirs), h.isSelected(*nobodys))
	}
}

// A player's new selection unselects its own units alone: another player's selection stays.
func TestSelect_AnotherPlayersSelectionStays(t *testing.T) {
	h := newHarness(t)
	mine := h.seed(100, 100, 10)
	theirs := h.seedOwned(200, 200, 10, 2)
	h.start()
	h.sel.selects.Put(2, Select{Box: everywhere})
	h.ecs.Tick(time.Second)
	if !h.isSelected(*theirs) || h.isSelected(*mine) {
		t.Fatalf("player 2 selected its own %v and mine %v; want its own alone", h.isSelected(*theirs), h.isSelected(*mine))
	}
	h.click(105, 105, false)
	if !h.isSelected(*mine) || !h.isSelected(*theirs) {
		t.Errorf("after my click mine %v, player 2's %v selected; want both", h.isSelected(*mine), h.isSelected(*theirs))
	}
}

// A Select nobody gave — the game's code, a script — selects the units nobody owns alone.
func TestSelect_NobodySelectsTheOwnerlessAlone(t *testing.T) {
	h := newHarness(t)
	mine := h.seed(100, 100, 10)
	nobodys := h.seedOwned(300, 300, 10, control.Nobody)
	h.start()
	h.sel.selects.Put(control.Nobody, Select{Box: everywhere})
	h.ecs.Tick(time.Second)
	if !h.isSelected(*nobodys) || h.isSelected(*mine) {
		t.Errorf("nobody's %v, mine %v selected; want nobody's alone", h.isSelected(*nobodys), h.isSelected(*mine))
	}
}
