package terrain_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/world"
)

func testWorldConfig() world.Config {
	return world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 100},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
		Heights:  true,
	}
}

type boardSaveLoadTestGame struct {
	worldPlugin *world.Plugin
	boardPlugin *board.Plugin
	grid        grid.Grid
	load        bool // entered from the quicksave

	stack game.Scenes
}

func (g *boardSaveLoadTestGame) Name() string { return "stage" }
func (g *boardSaveLoadTestGame) Init(ctx game.Initializer) error {
	g.worldPlugin = ctx.UseWorld(testWorldConfig())
	g.boardPlugin = board.NewPlugin(g.grid, &cell.SingleOccupancy{}, g.worldPlugin)
	return ctx.Use(g.boardPlugin)
}
func (g *boardSaveLoadTestGame) Loaded() error                     { return nil }
func (g *boardSaveLoadTestGame) Spawn() error                      { return nil }
func (g *boardSaveLoadTestGame) Update(goke.RunCtx, time.Duration) {}
func (g *boardSaveLoadTestGame) Stack() game.Scenes {
	if g.stack == nil {
		g.stack, _ = game.NewStack()
	}
	return g.stack
}

func TestPlugin_SaveLoad_TerrainRoundTripsAsCellEntities(t *testing.T) {
	t.Chdir(t.TempDir())
	grid := grid.DefaultGrids{}.Square(5, 5, 10)
	wall := cell.Kind{Name: cell.Named("wall"), Pace: 1, Solid: true}
	here, ok := grid.CellAt(geom.NewVec(21.0, 21.0))
	if !ok {
		t.Fatal("expected (21,21) to land inside the 5x5 grid")
	}

	stage := &boardSaveLoadTestGame{grid: grid}
	eng := engine.NewEngine(boardtest.OneStageGame{Stage: stage, GameProps: game.Props{}})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	stage.boardPlugin.Write().Board().Set(here, wall)
	way := cell.Way{
		Kind:  cell.Kind{Name: cell.Named("stream"), Pace: 0.5, Allows: cell.Land},
		Width: 5,
		Links: 1 << 3,
	}
	stage.boardPlugin.Write().Board().SetWay(here, way)
	id, _ := stage.boardPlugin.Read(0).Board().CellEntity(here)

	if err := eng.Save(""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	game2 := &boardSaveLoadTestGame{grid: grid, load: true}
	eng2 := engine.NewEngine(boardtest.OneStageGame{Stage: game2, GameProps: game.Props{}})
	if err := eng2.Load("", ""); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := eng2.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if got := game2.boardPlugin.Read(0).Board().Kind(here); got != way.Over(wall) {
		t.Errorf("Board().Kind(cell) after Load = %+v, want %+v", got, way.Over(wall))
	}
	if got := game2.boardPlugin.Read(0).Board().Way(here); got != way {
		t.Errorf("Way(cell) after Load = %+v, want %+v", got, way)
	}
	if got, _ := game2.boardPlugin.Read(0).Board().CellEntity(here); got != id {
		t.Errorf("the cell's entity after Load is %d, want %d, the one saved — none spawned "+
			"anew", got, id)
	}
}

// a load makes the cells out of the save with every cell's passage counted at once, each as the
// save holds it: a Set changing a loaded wall's look alone counts no passage
func TestCells_ALoadStampsEveryCellsPassage(t *testing.T) {
	t.Chdir(t.TempDir())
	grid := grid.DefaultGrids{}.Square(5, 5, 10)
	wall := cell.Kind{Name: cell.Named("wall"), Pace: 1, Solid: true}
	here := grid.At(2, 2)
	saved := &boardSaveLoadTestGame{grid: grid}
	eng := engine.NewEngine(boardtest.OneStageGame{Stage: saved, GameProps: game.Props{}})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	saved.boardPlugin.Write().Board().Set(here, wall)
	if err := eng.Save(""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded := &boardSaveLoadTestGame{grid: grid, load: true}
	eng2 := engine.NewEngine(boardtest.OneStageGame{Stage: loaded, GameProps: game.Props{}})
	if err := eng2.Load("", ""); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := eng2.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	brd := loaded.boardPlugin.Read(0).Board()
	passage := brd.Passage()
	if passage == 0 {
		t.Fatal("the load counted no passage")
	}
	brd.EachCell(func(c cell.ID) {
		if got := brd.CellPassage(c); got != passage {
			t.Errorf("cell %d's passage %d after the load, want every cell's %d", c, got, passage)
		}
	})
	painted := wall
	painted.SpriteID++
	version := brd.Version()
	loaded.boardPlugin.Write().Board().Set(here, painted)
	if brd.Version() == version || brd.Passage() != passage {
		t.Errorf("a loaded wall painted anew moved the Version %v and the Passage %v, want the "+
			"Version alone", brd.Version() != version, brd.Passage() != passage)
	}
}
