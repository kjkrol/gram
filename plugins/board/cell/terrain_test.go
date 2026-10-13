package cell_test

import (
	"testing"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/gram/plugins/board/cell"
)

func TestTerrainMap_CastAndLabelKeepRolesAndWhatACellIsCalled(t *testing.T) {
	const trapdoor, plate tag.Tag[cond.Roles] = 0, 5
	m := cell.NewTerrainMap()
	v := m.Version()

	m.Cast(3, tag.Tags[cond.Roles](0).With(trapdoor))
	m.Cast(3, tag.Tags[cond.Roles](0).With(trapdoor, plate))
	m.Label(3, ceremony.LabelOf("east lever", ""))
	m.Label(3, ceremony.LabelOf("west lever", "levers"))

	if got, want := m.Roles[3], tag.Tags[cond.Roles](0).With(trapdoor, plate); got != want {
		t.Errorf("cell 3 plays %b, want %b, what it was cast last", got, want)
	}
	if got, want := m.Labels[3], ceremony.LabelOf("west lever", "levers"); got != want {
		t.Errorf("cell 3 is called %v, want %v, what it was called last", got, want)
	}
	if m.Roles[4] != 0 || m.Labels[4] != (ceremony.Label{}) {
		t.Errorf("cell 4 plays %b and is called %v, want nothing of either", m.Roles[4], m.Labels[4])
	}
	if m.Version() != v {
		t.Errorf("Version moved from %d to %d: roles and names are no change of terrain", v, m.Version())
	}
}
