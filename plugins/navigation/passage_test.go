package navigation

import (
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/world"
)

// weatheredRig is the groups' board under the weather's effects on its cells: the wind's sway,
// which turns how the ground is drawn alone, and the ice, which turns it to ground a walker takes
type weatheredRig struct {
	*groupRig
	sway, ice effect.Effect
}

func newWeatheredRig(t *testing.T, starts []cell.ID) *weatheredRig {
	t.Helper()
	wr := &weatheredRig{}
	wr.groupRig = newGroupRigWith(t, nil, starts, func(w *world.Plugin, _ *board.Plugin) {
		fx := w.Write().Effects()
		fx.Define("sway", effect.Spec{effect.Alter(func(g *cell.Ground) { g.Kind.Sway = 0.6 })})
		fx.Define("ice", effect.Spec{effect.Alter(func(g *cell.Ground) { g.Kind = ice })})
		wr.sway, wr.ice = fx.Named("sway"), fx.Named("ice")
	})
	return wr
}

// weather has fx begin on the cell c and the board count what it changed; the units stand, the
// routing asked nothing
func (wr *weatheredRig) weather(t *testing.T, c cell.ID, fx effect.Effect) {
	t.Helper()
	id, ok := wr.nw.board.CellEntity(c)
	if !ok {
		t.Fatalf("cell %v has no entity", c)
	}
	put := ceremony.Put(fx).On(ceremony.ID(id))
	if !wr.nw.nav.worldPlugin.Write().Carrier().Put(command.Nobody, put) {
		t.Fatal("the world carries no command about an effect")
	}
	for range 10 {
		wr.nw.ecs.Tick(time.Second / 60)
		if wr.nw.board.States(c).Has(fx.Mark()) {
			wr.nw.ecs.Tick(time.Second / 60) // the board's pass may come before the effects'
			return
		}
	}
	t.Fatalf("the effect never began on cell %v", c)
}

// a sway turns the board's Version and leaves its Passage
func (wr *weatheredRig) sways(t *testing.T, c cell.ID) {
	t.Helper()
	version, passage := wr.nw.board.Version(), wr.nw.board.Passage()
	wr.weather(t, c, wr.sway)
	if wr.nw.board.Version() == version || wr.nw.board.Passage() != passage {
		t.Fatalf("the sway moved the Version %v and the Passage %v, want the Version alone",
			wr.nw.board.Version() != version, wr.nw.board.Passage() != passage)
	}
}

func TestKept_ASwayKeepsTheAnswers(t *testing.T) {
	wr := newWeatheredRig(t, westOf(3))
	wr.send(1, wr.nw.grid.At(26, 16), every(3)...)
	wr.update()
	wr.routed(t, every(3)...)
	searches := wr.coord.Searches()
	wr.sways(t, wr.nw.grid.At(10, 7))
	wr.ask(every(3)...)
	wr.update()
	if searched := wr.coord.Searches() - searches; searched != 0 {
		t.Errorf("%d searches for the routes asked again under the sway, want the answers kept",
			searched)
	}
	wr.routed(t, every(3)...)
}

// a unit of the group bumped off its route walks the field kept through the sway, unsearched
func TestGroup_ASwayKeepsTheField(t *testing.T) {
	wr := newWeatheredRig(t, westOf(8))
	goal := wr.nw.grid.At(26, 16)
	wr.send(1, goal, every(8)...)
	wr.update()
	bumped := slices.IndexFunc(every(8), func(u int) bool {
		o, _ := wr.order(wr.nw.ids[u])
		return o.Target == goal
	})
	if bumped < 0 {
		t.Fatal("no unit sent to the goal itself")
	}
	wr.sways(t, wr.nw.grid.At(10, 7))
	fills, searches, walks := wr.coord.Fills(), wr.coord.Searches(), wr.coord.Walks()
	wr.walk(bumped, 5)
	wr.ask(bumped)
	wr.update()
	if wr.coord.Fills() != fills || wr.coord.Searches() != searches ||
		wr.coord.Walks() != walks+1 {
		t.Errorf("%d fills, %d searches and %d walks more, want the route walked off the field "+
			"kept, once", wr.coord.Fills()-fills, wr.coord.Searches()-searches,
			wr.coord.Walks()-walks)
	}
	wr.routed(t, bumped)
}

// the gap in the wall is water till the ice: the routes asked again are searched anew, over it
func TestNavigation_AnIceDropsTheAnswers(t *testing.T) {
	wr := newWeatheredRig(t, westOf(3))
	gap := wr.nw.grid.At(16, 8)
	wr.nw.board.Set(gap, water)
	wr.send(1, wr.nw.grid.At(26, 16), every(3)...)
	wr.update()
	wr.routed(t, every(3)...)
	over := func(u int) bool {
		o, _ := wr.order(wr.nw.ids[u])
		return slices.Contains(o.Path.Steps[:o.Path.Length], gap)
	}
	for _, u := range every(3) {
		if over(u) {
			t.Fatalf("unit %d routed over the water in the gap", u)
		}
	}
	searches, passage := wr.coord.Searches(), wr.nw.board.Passage()
	wr.weather(t, gap, wr.ice)
	if wr.nw.board.Passage() == passage {
		t.Fatal("the ice moved no Passage")
	}
	wr.ask(every(3)...)
	wr.update()
	if searched := wr.coord.Searches() - searches; searched != 3 {
		t.Errorf("%d searches for the routes asked again over the ice, want one a unit", searched)
	}
	wr.routed(t, every(3)...)
	for _, u := range every(3) {
		if !over(u) {
			t.Errorf("unit %d goes round the wall, want it over the ice in the gap", u)
		}
	}
}
