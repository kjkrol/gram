package cell_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/uid"
)

func TestSingleOccupancy_IsOnePerDomainPerCell(t *testing.T) {
	o := &cell.SingleOccupancy{}
	const here, walker, hawk, boat, witch = cell.ID(7), 1, 2, 3, 4

	o.Enter(here, false, walker, cell.Land)
	if o.CanEnter(here, false, boat, cell.Land) {
		t.Error("a second land unit may enter a cell a land unit holds")
	}
	if !o.CanEnter(here, false, hawk, cell.Air) {
		t.Error("a flyer may not enter a cell a land unit holds")
	}
	if !o.CanEnter(here, false, walker, cell.Land) {
		t.Error("the holder itself may not re-enter")
	}
	o.Enter(here, false, hawk, cell.Air)
	if o.CanEnter(here, false, witch, cell.Land|cell.Water) {
		t.Error("a unit of Land and Water may enter beside a land unit")
	}
	if !o.CanEnter(here, false, boat, cell.Water) {
		t.Error("a boat may not enter a cell held in Land and Air only")
	}
	o.Leave(here, walker)
	if !o.CanEnter(here, false, boat, cell.Land) {
		t.Error("the land layer is still held after the land unit left")
	}
	if o.CanEnter(here, false, boat, cell.Air) {
		t.Error("the air layer is free while the flyer is still there")
	}
}

func TestMultipleOccupancy_LetsTokensStack(t *testing.T) {
	o := &cell.MultipleOccupancy{}
	const here = cell.ID(3)
	o.Enter(here, false, 1, cell.Land)
	o.Enter(here, false, 2, cell.Land)
	if !o.CanEnter(here, false, 3, cell.Land) {
		t.Error("a third token may not join two on a square")
	}
	o.Leave(here, 1)
	o.Leave(here, 2)
	if !o.CanEnter(here, false, 3, cell.Air) {
		t.Error("an empty square refuses")
	}
}

// Holder tells who holds a cell in a domain, whom a step into it would meet.
func TestOccupancy_HolderTellsWhoHoldsACell(t *testing.T) {
	for name, occ := range map[string]interface {
		cell.Occupancy
		Holder(cell.ID, bool, cell.Domain) (uid.UID64, bool)
	}{"single": &cell.SingleOccupancy{}, "multiple": &cell.MultipleOccupancy{}} {
		occ.Enter(3, false, 1, cell.Land)
		occ.Enter(3, false, 2, cell.Air)
		if who, ok := occ.Holder(3, false, cell.Land); !ok || who != 1 {
			t.Errorf("%s: the Land holder of cell 3 is %d %v, want 1", name, who, ok)
		}
		if who, ok := occ.Holder(3, false, cell.Air); !ok || who != 2 {
			t.Errorf("%s: the Air holder of cell 3 is %d %v, want 2", name, who, ok)
		}
		if _, ok := occ.Holder(3, false, cell.Water); ok {
			t.Errorf("%s: cell 3 has a Water holder", name)
		}
		if _, ok := occ.Holder(4, false, cell.Land); ok {
			t.Errorf("%s: cell 4 has a holder", name)
		}
	}
}

// Release lets go of every hold of the entities gone, the others' kept: a cell a gone entity held
// takes another again.
func TestOccupancy_ReleaseLetsGoOfTheGone(t *testing.T) {
	for name, occ := range map[string]cell.Occupancy{"single": &cell.SingleOccupancy{}, "multiple": &cell.MultipleOccupancy{}} {
		gone, stays := uid.UID64(1), uid.UID64(2)
		occ.Enter(10, false, gone, cell.Land)
		occ.Enter(11, false, gone, cell.Land) // the cell it was stepping into
		occ.Enter(12, false, stays, cell.Land)
		occ.Release(func(id uid.UID64) bool { return id == gone })
		if !occ.CanEnter(10, false, 3, cell.Land) || !occ.CanEnter(11, false, 3, cell.Land) {
			t.Errorf("%s: a cell the gone held still refuses another", name)
		}
		if h, ok := occ.(interface {
			Holder(cell.ID, bool, cell.Domain) (uid.UID64, bool)
		}); ok {
			if id, held := h.Holder(12, false, cell.Land); !held || id != stays {
				t.Errorf("%s: cell 12 held by %v %v, want the one staying", name, id, held)
			}
			if _, held := h.Holder(10, false, cell.Land); held {
				t.Errorf("%s: cell 10 still held by the gone", name)
			}
		}
	}
}

// A cell has two levels under a raised deck: a walker on the deck and one on the ground under it
// hold it each on its own, and each level takes one walker alone.
func TestSingleOccupancy_IsOnePerDomainOnEachLevel(t *testing.T) {
	o := &cell.SingleOccupancy{}
	const here, above, below, third = cell.ID(7), 1, 2, 3
	o.Enter(here, true, above, cell.Land)
	if !o.CanEnter(here, false, below, cell.Land) {
		t.Error("a walker may not hold the ground under a deck another walker holds")
	}
	o.Enter(here, false, below, cell.Land)
	if o.CanEnter(here, true, third, cell.Land) || o.CanEnter(here, false, third, cell.Land) {
		t.Error("a third walker may hold a level another holds")
	}
	if who, ok := o.Holder(here, false, cell.Land); !ok || who != below {
		t.Errorf("the ground's holder is %d %v, want the one under the deck", who, ok)
	}
	o.Enter(here, false, above, cell.Land)
	if !o.CanEnter(here, true, third, cell.Land) {
		t.Error("the deck is still held after its walker came down under it")
	}
}

// Holders hands over every hold of a cell, each with its domains and level.
func TestOccupancy_HoldersHandsEveryHoldOfACell(t *testing.T) {
	type hold struct {
		entity uid.UID64
		domain cell.Domain
		deck   bool
	}
	for name, occ := range map[string]cell.Occupancy{
		"single":   &cell.SingleOccupancy{},
		"multiple": &cell.MultipleOccupancy{},
	} {
		occ.Enter(3, false, 1, cell.Land)
		occ.Enter(3, true, 2, cell.Land)
		occ.Enter(4, false, 3, cell.Air)
		var got []hold
		occ.Holders(3, func(e uid.UID64, d cell.Domain, deck bool) {
			got = append(got, hold{e, d, deck})
		})
		want := []hold{{1, cell.Land, false}, {2, cell.Land, true}}
		if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
			t.Errorf("%s: the holds of cell 3 are %v, want %v", name, got, want)
		}
		occ.Holders(5, func(uid.UID64, cell.Domain, bool) {
			t.Errorf("%s: cell 5 has a hold", name)
		})
	}
}
