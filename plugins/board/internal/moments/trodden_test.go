package moments

import (
	"slices"
	"testing"

	"github.com/kjkrol/gram/internal/hosts"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/terrain"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
)

// The standing pass marks the cells units stand on only while a rule of a cell.Now is hooked, the
// one reader of Trodden; with none, or with rules of a Standing alone, it marks nothing.
func TestTrodden_MarkedOnlyWhileACellNowRuleIsHooked(t *testing.T) {
	g := grid.DefaultGrids{}.Square(3, 1, 10)
	cells := terrain.New(g)
	cells.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	middle := g.CellIndex(1, 0)
	never := func(unit.Standing) bool { return false }
	for name, c := range map[string]struct {
		rules []rule.Rule
		want  []bool
	}{
		"no rules":             {},
		"rules of a Standing":  {rules: []rule.Rule{rule.Then[unit.Standing]("never", rule.All, rule.If(never, rule.Order(struct{}{})))}},
		"a rule of a cell.Now": {rules: []rule.Rule{rule.Then[cell.Now]("stood", rule.All, rule.If(cell.Now.Stood, rule.Order(struct{}{})))}, want: []bool{false, true, false}},
	} {
		r := New(g, cells, nil, flat)
		for _, b := range c.rules {
			if err := hosts.Deliver(r.Hosts(), b); err != nil {
				t.Fatal(err)
			}
		}
		walk(t, r, 2, walker{box: cellBox(g, middle, 4), vel: world.Velocity{}, domain: cell.Land})
		if c.want == nil {
			if slices.Contains(r.trodden, true) {
				t.Errorf("%s: trodden %v, want none marked", name, r.trodden)
			}
			continue
		}
		if !slices.Equal(r.trodden, c.want) {
			t.Errorf("%s: trodden %v, want %v", name, r.trodden, c.want)
		}
	}
}
