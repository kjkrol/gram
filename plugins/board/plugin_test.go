package board

import (
	"slices"
	"strings"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

func TestNewPlugin_SetsEachGridAxisFromTheWorldsEdges(t *testing.T) {
	for _, tc := range []struct {
		edges        aabbworld.Edges
		wrapX, wrapY bool
	}{
		{0, false, false},
		{aabbworld.Torus, true, true},
		{aabbworld.WrapX | aabbworld.OpenY, true, false},
		{aabbworld.WrapY, false, true},
	} {
		g := grid.DefaultGrids{}.Square(5, 5, 10)
		worldPlugin := world.NewPlugin(world.Config{
			Space:    world.SpaceCfg{Width: 100, Height: 100, Edges: tc.edges},
			Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
			Heights:  true,
		})
		NewPlugin(g, &cell.SingleOccupancy{}, worldPlugin)

		if sq, _ := grid.ShapeOf(g); sq.WrapX != tc.wrapX || sq.WrapY != tc.wrapY {
			t.Errorf("edges %04b: grid wraps x=%v y=%v, want x=%v y=%v", tc.edges, sq.WrapX, sq.WrapY, tc.wrapX, tc.wrapY)
		}
	}
}

func TestNewPlugin_RefusesAFlatWorld(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 50, Height: 50},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	defer func() {
		if msg, _ := recover().(string); !strings.Contains(msg, "world.Config.Heights") {
			t.Errorf("panic %q, want one asking for world.Config.Heights", msg)
		}
	}()
	NewPlugin(grid.DefaultGrids{}.Square(5, 5, 10), &cell.SingleOccupancy{}, w)
	t.Error("a board in a flat world: want a panic")
}

func newSeedTestPlugin(t *testing.T) (*Plugin, cell.ID) {
	t.Helper()
	grid := grid.DefaultGrids{}.Square(5, 5, 10)
	worldPlugin := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 50, Height: 50},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
		Heights:  true,
	})
	p := NewPlugin(grid, &cell.SingleOccupancy{}, worldPlugin)
	p.CellKinds().Define("grass", cell.Kind{Cost: 1, Allows: cell.Land})
	p.CellKinds().Define("wall", cell.Kind{Cost: 1, Solid: true})
	cell := grid.At(2, 2)
	return p, cell
}

func TestNewPlugin_RequiresACellAndAMoverOfEveryUnit(t *testing.T) {
	w := world.NewPlugin(world.Config{Space: world.SpaceCfg{Width: 100, Height: 100}, Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 10, MaxSize: 10}, Heights: true})
	NewPlugin(grid.DefaultGrids{}.Square(2, 2, 32), &cell.MultipleOccupancy{}, w)
	defer func() {
		msg, _ := recover().(string)
		for _, want := range []string{"board requires unit.At (the cell it starts in)", "board requires unit.Mover (the domains it moves in)"} {
			if !strings.Contains(msg, want) {
				t.Errorf("panic %q does not mention %q", msg, want)
			}
		}
	}()

	w.Roster().Spec(motion.UnitTemplate, comp.Const(world.Position{}))
	t.Error("the roster accepted a unit without an unit.At and a unit.Mover")
}

func TestPlugin_SeedPopulate_AppliesLayout(t *testing.T) {
	p, wallCell := newSeedTestPlugin(t)
	other := p.Board().At(0, 0)
	p.Seed(Layout{Default: "grass", Cells: []cell.Entry{{Kind: "wall", Cell: wallCell}}})

	if err := p.Populate(); err != nil {
		t.Fatalf("Populate: %v", err)
	}
	if got := p.Board().Kind(wallCell).Name.String(); got != "wall" {
		t.Errorf("Kind(wallCell) = %q, want %q", got, "wall")
	}
	if got := p.Board().Kind(other).Name.String(); got != "grass" {
		t.Errorf("Kind(other) = %q, want %q (Default)", got, "grass")
	}
}

func TestPlugin_Populate_UnknownKindChangesNothing(t *testing.T) {
	p, lava := newSeedTestPlugin(t)
	p.Seed(Layout{Default: "grass", Cells: []cell.Entry{{Kind: "lava", Cell: lava}}})

	if err := p.Populate(); err == nil {
		t.Fatal("Populate: expected an error for unknown kind, got nil")
	}
	if got := p.Board().Kind(lava).Name.String(); got != "" {
		t.Errorf("Kind(lava) = %q, want untouched terrain", got)
	}
}

func TestPlugin_DrawsFromTheGamesAtlasAlone(t *testing.T) {
	worldPlugin := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
		Heights:  true,
	})
	p := NewPlugin(grid.DefaultGrids{}.Square(5, 5, 10), &cell.SingleOccupancy{}, worldPlugin)
	if p.Atlas() != nil {
		t.Fatal("an atlas before the renderer")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("a renderer without an atlas: want a panic")
			}
		}()
		p.SetAtlas(nil)
	}()
	atlas := p.NewAtlas(10)
	atlas.Close()
	if p.Atlas() != atlas {
		t.Error("the atlas closed is not the board's")
	}
}

type drawsCtx struct {
	plugin.Installer
	drawn []render.Source
	looks int
}

func (*drawsCtx) UseModule(goke.Module)      {}
func (*drawsCtx) Hosts(...host.Host)         {}
func (c *drawsCtx) Draws(s ...render.Source) { c.drawn = append(c.drawn, s...) }
func (c *drawsCtx) Looks(func())             { c.looks++ }

type stillSky struct{}

func (stillSky) Sun() sky.Sun     { return sky.DefaultSun }
func (stillSky) Air() air.Weather { return air.Weather{} }

func TestPlugin_DrawsItsGroundAndItsHoldsAndDressesTheGroundUnderASky(t *testing.T) {
	worldPlugin := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
		Heights:  true,
	})
	p := NewPlugin(grid.DefaultGrids{}.Square(5, 5, 10), &cell.SingleOccupancy{}, worldPlugin)
	ctx := &drawsCtx{}
	if err := p.Install(ctx); err != nil {
		t.Fatal(err)
	}
	want := []render.Source{p.renderer, p.holdsView}
	if !slices.Equal(ctx.drawn, want) || ctx.looks != 1 {
		t.Fatalf("drew %v with %d looks, want the ground and the holds, one look", ctx.drawn,
			ctx.looks)
	}
	if _, ok := p.Map().Dressing().(look.GroundDressing); ok {
		t.Error("under no sky the ground is dressed")
	}
	p.WithAtmosphere(stillSky{})
	if _, ok := p.Map().Dressing().(look.GroundDressing); !ok {
		t.Error("under a sky the ground is not dressed with the clouds' shadows")
	}
}
