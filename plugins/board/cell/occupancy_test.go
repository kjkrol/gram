package cell_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/uid"
)

func TestSingleOccupancy_IsOnePerDomainPerCell(t *testing.T) {
	o := &cell.SingleOccupancy{}
	const here, walker, hawk, boat, witch = cell.ID(7), 1, 2, 3, 4

	o.Enter(here, walker, cell.Land)
	if o.CanEnter(here, boat, cell.Land) {
		t.Error("a second land unit may enter a cell a land unit holds")
	}
	if !o.CanEnter(here, hawk, cell.Air) {
		t.Error("a flyer may not enter a cell a land unit holds")
	}
	if !o.CanEnter(here, walker, cell.Land) {
		t.Error("the holder itself may not re-enter")
	}
	o.Enter(here, hawk, cell.Air)
	if o.CanEnter(here, witch, cell.Land|cell.Water) {
		t.Error("a unit of Land and Water may enter beside a land unit")
	}
	if !o.CanEnter(here, boat, cell.Water) {
		t.Error("a boat may not enter a cell held in Land and Air only")
	}
	o.Leave(here, walker)
	if !o.CanEnter(here, boat, cell.Land) {
		t.Error("the land layer is still held after the land unit left")
	}
	if o.CanEnter(here, boat, cell.Air) {
		t.Error("the air layer is free while the flyer is still there")
	}
}

func TestMultipleOccupancy_LetsTokensStack(t *testing.T) {
	o := &cell.MultipleOccupancy{}
	const here = cell.ID(3)
	o.Enter(here, 1, cell.Land)
	o.Enter(here, 2, cell.Land)
	if !o.CanEnter(here, 3, cell.Land) {
		t.Error("a third token may not join two on a square")
	}
	o.Leave(here, 1)
	o.Leave(here, 2)
	if !o.CanEnter(here, 3, cell.Air) {
		t.Error("an empty square refuses")
	}
}

// Holder tells who holds a cell in a domain, whom a step into it would meet.
func TestOccupancy_HolderTellsWhoHoldsACell(t *testing.T) {
	for name, occ := range map[string]interface {
		cell.Occupancy
		Holder(cell.ID, cell.Domain) (uid.UID64, bool)
	}{"single": &cell.SingleOccupancy{}, "multiple": &cell.MultipleOccupancy{}} {
		occ.Enter(3, 1, cell.Land)
		occ.Enter(3, 2, cell.Air)
		if who, ok := occ.Holder(3, cell.Land); !ok || who != 1 {
			t.Errorf("%s: the Land holder of cell 3 is %d %v, want 1", name, who, ok)
		}
		if who, ok := occ.Holder(3, cell.Air); !ok || who != 2 {
			t.Errorf("%s: the Air holder of cell 3 is %d %v, want 2", name, who, ok)
		}
		if _, ok := occ.Holder(3, cell.Water); ok {
			t.Errorf("%s: cell 3 has a Water holder", name)
		}
		if _, ok := occ.Holder(4, cell.Land); ok {
			t.Errorf("%s: cell 4 has a holder", name)
		}
	}
}

// Release lets go of every hold of the entities gone, the others' kept: a cell a gone entity held
// takes another again.
func TestOccupancy_ReleaseLetsGoOfTheGone(t *testing.T) {
	for name, occ := range map[string]cell.Occupancy{"single": &cell.SingleOccupancy{}, "multiple": &cell.MultipleOccupancy{}} {
		gone, stays := uid.UID64(1), uid.UID64(2)
		occ.Enter(10, gone, cell.Land)
		occ.Enter(11, gone, cell.Land) // the cell it was stepping into
		occ.Enter(12, stays, cell.Land)
		occ.Release(func(id uid.UID64) bool { return id == gone })
		if !occ.CanEnter(10, 3, cell.Land) || !occ.CanEnter(11, 3, cell.Land) {
			t.Errorf("%s: a cell the gone held still refuses another", name)
		}
		if h, ok := occ.(interface {
			Holder(cell.ID, cell.Domain) (uid.UID64, bool)
		}); ok {
			if id, held := h.Holder(12, cell.Land); !held || id != stays {
				t.Errorf("%s: cell 12 held by %v %v, want the one staying", name, id, held)
			}
			if _, held := h.Holder(10, cell.Land); held {
				t.Errorf("%s: cell 10 still held by the gone", name)
			}
		}
	}
}
