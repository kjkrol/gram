package comp_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
	"github.com/kjkrol/uid"
)

type row struct{ hp int }

type stat struct{ HP int }

func TestConst_IsTheSameForEveryRow(t *testing.T) {
	c := comp.Const(stat{HP: 3})

	for _, r := range []any{nil, row{hp: 99}} {
		if got := c.Resolve(r, 0); got.HP != 3 {
			t.Errorf("Const resolved to %+v for row %v, want HP 3 whatever the row", got, r)
		}
	}
}

func TestLoad_ReadsTheRow_AndRunsItsEffectWithTheValue(t *testing.T) {
	var seen stat
	var seenID uid.UID64
	c := comp.Load(func(r row) stat { return stat{HP: r.hp} }).
		WithEffect(func(v stat, id uid.UID64) { seen, seenID = v, id })

	got := c.Resolve(row{hp: 9}, uid.UID64(42))

	if got.HP != 9 {
		t.Errorf("Load resolved to %+v, want the row's 9", got)
	}
	if seen.HP != 9 || seenID != 42 {
		t.Errorf("effect saw %+v for entity %v, want the value 9 and the entity 42", seen, seenID)
	}
}
