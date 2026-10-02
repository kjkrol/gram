package kind_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/render"
)

type row struct{ hp int }

type stat struct{ HP int }

// shelf is the least a Registry can be: it remembers what it was given.
type shelf struct {
	name string
	row  reflect.Type
	spec kind.Spec
}

func (s *shelf) Register(name string, row reflect.Type, spec kind.Spec) (kind.ID, render.SpriteID) {
	s.name, s.row, s.spec = name, row, spec
	return 3, 7
}

func TestDefine_RegistersTheSpecAndHandsBackTheKind(t *testing.T) {
	reg := &shelf{}
	spec := kind.Spec{comp.Load(func(r row) stat { return stat{HP: r.hp} }), comp.Const(struct{}{})}

	unit := kind.Define[row](reg, "unit", spec)

	if reg.name != "unit" || reg.row != reflect.TypeFor[row]() || len(reg.spec) != 2 {
		t.Errorf("the registry was given (%q, %v, %d comps), want (unit, row, 2)", reg.name, reg.row, len(reg.spec))
	}
	if unit.Name() != "unit" || unit.ID() != 3 || unit.SpriteID() != 7 {
		t.Errorf("the kind says (%q, %d, %d), want what the registry assigned: (unit, 3, 7)", unit.Name(), unit.ID(), unit.SpriteID())
	}

	entry := unit.Entry(row{hp: 5})
	if entry.Kind() != "unit" || entry.Row() != (row{hp: 5}) {
		t.Errorf("entry = (%q, %v), want (unit, {5})", entry.Kind(), entry.Row())
	}
}

func TestDefine_RefusesALoadThatReadsAnotherRowType(t *testing.T) {
	defer func() {
		msg, _ := recover().(string)
		for _, want := range []string{`"unit"`, "kind_test.row", "int"} {
			if !strings.Contains(msg, want) {
				t.Errorf("panic %q does not mention %s", msg, want)
			}
		}
	}()

	kind.Define[row](&shelf{}, "unit", kind.Spec{comp.Load(func(hp int) stat { return stat{HP: hp} })})
	t.Error("Define accepted a Load over int for a kind whose rows are row")
}
