package navigation

import "testing"

func TestSpacing_AutoKeepsBodiesApartForUnitsAThirdOfACellOrLess(t *testing.T) {
	for _, c := range []struct {
		largest, cell float64
		want          Spacing
	}{{3, 32, BodySpacing}, {10, 32, BodySpacing}, {22, 32, CellSpacing}, {24, 48, CellSpacing}, {0, 32, CellSpacing}} {
		if got := AutoSpacing.resolve(c.largest, c.cell); got != c.want {
			t.Errorf("units %v on cells %v keep apart by %v, want %v", c.largest, c.cell, got, c.want)
		}
	}
	if got := CellSpacing.resolve(3, 32); got != CellSpacing {
		t.Errorf("asked for cells, small units keep apart by %v", got)
	}
}
