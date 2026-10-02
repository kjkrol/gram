package comp_test

import (
	"testing"

	"github.com/kjkrol/gram/entity/kind/comp"
)

type row struct{ hp int }

type stat struct{ HP int }

func TestConst_IsTheSameForEveryRow(t *testing.T) {
	c := comp.Const(stat{HP: 3})

	for _, r := range []any{nil, row{hp: 99}} {
		if got := c.Resolve(r); got.HP != 3 {
			t.Errorf("Const resolved to %+v for row %v, want HP 3 whatever the row", got, r)
		}
	}
}

func TestLoad_ReadsTheRow(t *testing.T) {
	c := comp.Load(func(r row) stat { return stat{HP: r.hp} })

	if got := c.Resolve(row{hp: 9}); got.HP != 9 {
		t.Errorf("Load resolved to %+v, want the row's 9", got)
	}
}
