package cell_test

import (
	"testing"

	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/rule"
)

// Cast adds roles to those a cell plays, Wire wires it to one wire, the last given; neither
// moves the terrain's Version, and other cells play nothing and are wired to nothing.
func TestTerrainMap_CastAndWireKeepRolesAndAWirePerCell(t *testing.T) {
	const trapdoor, plate tag.Tag[rule.Roles] = 0, 5
	west, east := rule.NewWire("west"), rule.NewWire("east")
	m := cell.NewTerrainMap()
	v := m.Version()

	m.Cast(3, tag.Tags[rule.Roles](0).With(trapdoor))
	m.Cast(3, tag.Tags[rule.Roles](0).With(plate))
	m.Wire(3, east)
	m.Wire(3, west)

	if got, want := m.Roles[3], tag.Tags[rule.Roles](0).With(trapdoor, plate); got != want {
		t.Errorf("cell 3 plays %b, want %b: Cast adds to the roles it plays", got, want)
	}
	if got := m.Wired[3]; got != west {
		t.Errorf("cell 3 is wired to %v, want west, the wire given last", got)
	}
	if m.Roles[4] != 0 || m.Wired[4] != nil {
		t.Errorf("cell 4 plays %b, wired to %v; want nothing", m.Roles[4], m.Wired[4])
	}
	if m.Version() != v {
		t.Errorf("Version went from %d to %d; roles and wires are no terrain", v, m.Version())
	}
}

// Stood is Trodden: whether a unit stands on the cell now.
func TestNow_StoodIsTrodden(t *testing.T) {
	if !(cell.Now{Trodden: true}).Stood() || (cell.Now{}).Stood() {
		t.Error("Stood is not Trodden")
	}
}
