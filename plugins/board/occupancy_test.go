package board_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/uid"
)

func TestSingleOccupancy_IsOnePerDomainPerCell(t *testing.T) {
	o := &board.SingleOccupancy{}
	const cell, walker, hawk, boat, witch = board.CellID(7), 1, 2, 3, 4

	o.Enter(cell, walker, board.Land)
	if o.CanEnter(cell, boat, board.Land) {
		t.Error("a second land unit may enter a cell a land unit holds")
	}
	if !o.CanEnter(cell, hawk, board.Air) {
		t.Error("a flyer may not enter a cell a land unit holds")
	}
	if !o.CanEnter(cell, walker, board.Land) {
		t.Error("the holder itself may not re-enter")
	}
	o.Enter(cell, hawk, board.Air)
	if o.CanEnter(cell, witch, board.Land|board.Water) {
		t.Error("a unit of Land and Water may enter beside a land unit")
	}
	if !o.CanEnter(cell, boat, board.Water) {
		t.Error("a boat may not enter a cell held in Land and Air only")
	}
	o.Leave(cell, walker)
	if !o.CanEnter(cell, boat, board.Land) {
		t.Error("the land layer is still held after the land unit left")
	}
	if o.CanEnter(cell, boat, board.Air) {
		t.Error("the air layer is free while the flyer is still there")
	}
}

func TestMultipleOccupancy_LetsTokensStack(t *testing.T) {
	o := &board.MultipleOccupancy{}
	const cell = board.CellID(3)
	o.Enter(cell, 1, board.Land)
	o.Enter(cell, 2, board.Land)
	if !o.CanEnter(cell, 3, board.Land) {
		t.Error("a third token may not join two on a square")
	}
	o.Leave(cell, 1)
	o.Leave(cell, 2)
	if !o.CanEnter(cell, 3, board.Air) {
		t.Error("an empty square refuses")
	}
}

// Holder tells who holds a cell in a domain, whom a step into it would meet.
func TestOccupancy_HolderTellsWhoHoldsACell(t *testing.T) {
	for name, occ := range map[string]interface {
		board.Occupancy
		Holder(board.CellID, board.Domain) (uid.UID64, bool)
	}{"single": &board.SingleOccupancy{}, "multiple": &board.MultipleOccupancy{}} {
		occ.Enter(3, 1, board.Land)
		occ.Enter(3, 2, board.Air)
		if who, ok := occ.Holder(3, board.Land); !ok || who != 1 {
			t.Errorf("%s: the Land holder of cell 3 is %d %v, want 1", name, who, ok)
		}
		if who, ok := occ.Holder(3, board.Air); !ok || who != 2 {
			t.Errorf("%s: the Air holder of cell 3 is %d %v, want 2", name, who, ok)
		}
		if _, ok := occ.Holder(3, board.Water); ok {
			t.Errorf("%s: cell 3 has a Water holder", name)
		}
		if _, ok := occ.Holder(4, board.Land); ok {
			t.Errorf("%s: cell 4 has a holder", name)
		}
	}
}

// Release lets go of every hold of the entities gone, the others' kept: a cell a gone entity held
// takes another again.
func TestOccupancy_ReleaseLetsGoOfTheGone(t *testing.T) {
	for name, occ := range map[string]board.Occupancy{"single": &board.SingleOccupancy{}, "multiple": &board.MultipleOccupancy{}} {
		gone, stays := uid.UID64(1), uid.UID64(2)
		occ.Enter(10, gone, board.Land)
		occ.Enter(11, gone, board.Land) // the cell it was stepping into
		occ.Enter(12, stays, board.Land)
		occ.Release(func(id uid.UID64) bool { return id == gone })
		if !occ.CanEnter(10, 3, board.Land) || !occ.CanEnter(11, 3, board.Land) {
			t.Errorf("%s: a cell the gone held still refuses another", name)
		}
		if h, ok := occ.(interface {
			Holder(board.CellID, board.Domain) (uid.UID64, bool)
		}); ok {
			if id, held := h.Holder(12, board.Land); !held || id != stays {
				t.Errorf("%s: cell 12 held by %v %v, want the one staying", name, id, held)
			}
			if _, held := h.Holder(10, board.Land); held {
				t.Errorf("%s: cell 10 still held by the gone", name)
			}
		}
	}
}
