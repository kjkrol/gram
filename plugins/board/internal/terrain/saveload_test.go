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
	}
}

type boardSaveLoadTestGame struct {
	worldPlugin *world.Plugin
	boardPlugin *board.Plugin
	grid        grid.Grid
	loadFrom    string

	stack game.Scenes
}

func (g *boardSaveLoadTestGame) Name() string { return "stage" }
func (g *boardSaveLoadTestGame) Init(ctx game.Initializer) error {
	g.worldPlugin = ctx.UseWorld(testWorldConfig())
	g.boardPlugin = board.NewPlugin(g.grid, &cell.SingleOccupancy{}, g.worldPlugin)
	return ctx.Use(g.boardPlugin)
}
func (g *boardSaveLoadTestGame) Restore(p game.Persistence) (bool, error) {
	if g.loadFrom == "" {
		return false, nil
	}
	if err := p.Load(g.loadFrom, ""); err != nil {
		return false, err
	}
	return true, nil
}
func (g *boardSaveLoadTestGame) Spawn() error                      { return nil }
func (g *boardSaveLoadTestGame) Update(goke.RunCtx, time.Duration) {}
func (g *boardSaveLoadTestGame) Stack() game.Scenes {
	if g.stack == nil {
		g.stack, _ = game.NewStack()
	}
	return g.stack
}

func TestPlugin_SaveLoad_TerrainRoundTripsAsCellEntities(t *testing.T) {
	basePath := t.TempDir() + "/save"
	grid := grid.DefaultGrids{}.Square(5, 5, 10)
	wall := cell.Kind{Name: cell.Named("wall"), Cost: 1, Solid: true}
	here, ok := grid.CellAt(geom.NewVec(21.0, 21.0))
	if !ok {
		t.Fatal("expected (21,21) to land inside the 5x5 grid")
	}

	stage := &boardSaveLoadTestGame{grid: grid}
	eng := engine.NewEngine(boardtest.OneStageGame{Stage: stage, GameProps: game.Props{}})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	stage.boardPlugin.Res.Logic.Board.Set(here, wall)
	way := cell.Way{Kind: cell.Kind{Name: cell.Named("stream"), Cost: 2, Allows: cell.Land}, Width: 5, Links: 1 << 3}
	stage.boardPlugin.Res.Logic.Board.SetWay(here, way)
	id, _ := stage.boardPlugin.CellEntity(here)

	if err := eng.Persistence().Save(basePath, ""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	game2 := &boardSaveLoadTestGame{grid: grid, loadFrom: basePath}
	eng2 := engine.NewEngine(boardtest.OneStageGame{Stage: game2, GameProps: game.Props{}})
	if err := eng2.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if got := game2.boardPlugin.Res.Logic.Board.Kind(here); got != way.Over(wall) {
		t.Errorf("Board().Kind(cell) after Load = %+v, want %+v", got, way.Over(wall))
	}
	if got := game2.boardPlugin.Res.Logic.Board.Way(here); got != way {
		t.Errorf("Way(cell) after Load = %+v, want %+v", got, way)
	}
	if got, _ := game2.boardPlugin.CellEntity(here); got != id {
		t.Errorf("the cell's entity after Load is %d, want %d, the one saved — none spawned anew", got, id)
	}
}
