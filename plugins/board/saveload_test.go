package board_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/board"
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
	grid        board.Grid
	loadFrom    string

	stack game.Scenes
}

func (g *boardSaveLoadTestGame) Name() string { return "stage" }
func (g *boardSaveLoadTestGame) Init(ctx game.Initializer) error {
	g.worldPlugin = ctx.UseWorld(testWorldConfig())
	g.boardPlugin = board.NewPlugin(g.grid, &board.SingleOccupancy{}, g.worldPlugin)
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

// oneStageGame is a minimal game.Game wrapping a single Stage.
type oneStageGame struct {
	stage game.Stage
	props game.Props
}

func (g oneStageGame) Props() game.Props { return g.props }

func (g oneStageGame) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{g.stage.Name(): g.stage}, g.stage.Name()
}

func TestPlugin_SaveLoad_TerrainRoundTripsAsCellEntities(t *testing.T) {
	basePath := t.TempDir() + "/save"
	grid := board.DefaultGrids{}.Square(5, 5, 10)
	wall := board.CellKind{Name: board.Named("wall"), Cost: 1, Solid: true}
	cell, ok := grid.CellAt(geom.NewVec(21.0, 21.0))
	if !ok {
		t.Fatal("expected (21,21) to land inside the 5x5 grid")
	}

	stage := &boardSaveLoadTestGame{grid: grid}
	eng := engine.NewEngine(oneStageGame{stage: stage, props: game.Props{}})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	stage.boardPlugin.Res.Logic.Board.Set(cell, wall)
	slope := board.Relief{Corners: [4]float32{1, 2, 3, 4}}
	stage.boardPlugin.Res.Logic.Board.SetRelief(cell, slope)
	way := board.Way{Kind: board.CellKind{Name: board.Named("stream"), Cost: 2, Allows: board.Land}, Width: 5, Links: 1 << 3}
	stage.boardPlugin.Res.Logic.Board.SetWay(cell, way)
	id, _ := stage.boardPlugin.CellEntity(cell)

	if err := eng.Persistence().Save(basePath, ""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	game2 := &boardSaveLoadTestGame{grid: grid, loadFrom: basePath}
	eng2 := engine.NewEngine(oneStageGame{stage: game2, props: game.Props{}})
	if err := eng2.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if got := game2.boardPlugin.Res.Logic.Board.Kind(cell); got != way.Over(wall) {
		t.Errorf("Board().Kind(cell) after Load = %+v, want %+v", got, way.Over(wall))
	}
	if got := game2.boardPlugin.Res.Logic.Board.Way(cell); got != way {
		t.Errorf("Way(cell) after Load = %+v, want %+v", got, way)
	}
	if got := game2.boardPlugin.Res.Logic.Board.Relief(cell); got != slope {
		t.Errorf("Relief(cell) after Load = %v, want %v", got, slope)
	}
	if got, _ := game2.boardPlugin.CellEntity(cell); got != id {
		t.Errorf("the cell's entity after Load is %d, want %d, the one saved — none spawned anew", got, id)
	}
}
