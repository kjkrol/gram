package forest_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/forest"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/world"
)

// copse records the parts the Stage runs of it, mixing in the kind of tree mixed
type copse struct {
	mixed string
	ran   []string
}

var (
	_ forest.Figure   = (*copse)(nil)
	_ forest.Stateful = (*copse)(nil)
	_ forest.Ruled    = (*copse)(nil)
)

func (c *copse) Effects(p *forest.Plugin) {
	c.ran = append(c.ran, "effects")
	p.World().Write().Effects().Define("burnt", effect.Spec{})
}

func (c *copse) Rules(p *forest.Plugin) {
	c.ran = append(c.ran, "rules")
	p.Stands().Define("copse", forest.Stand{Mix: []forest.Share{{Tree: c.mixed}}})
}

func (c *copse) Define(p *forest.Plugin) {
	c.ran = append(c.ran, "define")
	p.Types().Define("oak", oak)
}

func (c *copse) Look(*world.Atlas, *forest.Plugin) { c.ran = append(c.ran, "look") }

func newCopse(mixed string) (*world.Plugin, *copse) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 320, Height: 320},
		Entities: world.EntitiesCfg{MaxCount: 64, MinSize: 1, MaxSize: cellSize},
		Heights:  true,
	})
	m := motion.NewPlugin(w)
	brd := board.NewPlugin(grid.DefaultGrids{}.Square(10, 10, cellSize), &cell.MultipleOccupancy{},
		w).WithMotion(m, cell.DefaultHiking)
	p := forest.NewPlugin(w, brd)
	c := &copse{mixed: mixed}
	p.Figures(c)
	for _, part := range []section.Part{section.Effects, section.Rules, section.Kinds} {
		w.FiguresIn(part)
	}
	return w, c
}

func TestFigures_RunTheirPartsOnThePluginAndLookOnTheWorldsAtlas(t *testing.T) {
	w, c := newCopse("oak")
	if want := []string{"effects", "rules", "define"}; !slices.Equal(c.ran, want) {
		t.Errorf("the figure ran %v, want %v", c.ran, want)
	}
	if _, _, _, ok := w.Kinds().Lookup("oak"); !ok {
		t.Error("the oak is not the world's kind")
	}
	w.NewAtlas().Looks()
	if c.ran[len(c.ran)-1] != "look" {
		t.Errorf("the world's Looks: the figure ran %v, want it looked", c.ran)
	}
}

func TestStands_AMixOfNoKindOfTreePanicsAsTheLooksAreDeclared(t *testing.T) {
	w, _ := newCopse("elm")
	defer func() {
		if r, _ := recover().(string); !strings.Contains(r, `"elm"`) {
			t.Errorf("the looks declared panic with %q, want it to name the elm", r)
		}
	}()
	w.NewAtlas().Looks()
}
