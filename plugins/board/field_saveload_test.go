package board_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
)

// fieldStage is a wall column on a board, saved or loaded.
type fieldStage struct {
	loadFrom string
	grid     board.Grid

	world     *world.Plugin
	collision *collision.Plugin
	board     *board.Plugin
	stack     game.Scenes
}

func (g *fieldStage) Name() string { return "stage" }

func (g *fieldStage) Init(ctx game.Initializer) error {
	g.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: 6 * cellSize, Height: 16 * cellSize},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: unitSize, MaxSize: unitSize},
	})
	g.collision = collision.NewPlugin(g.world)
	if err := ctx.Use(g.collision); err != nil {
		return err
	}
	g.board = board.NewPlugin(g.grid, &board.MultipleOccupancy{}, g.world)
	g.board.CellKindDict().Create(
		board.CellKind{Name: board.Named("grass"), Cost: 1, Allows: board.Land},
		board.CellKind{Name: board.Named("wall"), Cost: 1, Solid: true},
	)
	if err := ctx.Use(g.board); err != nil {
		return err
	}
	return nil
}

func (g *fieldStage) Restore(p game.Persistence) (bool, error) {
	if g.loadFrom == "" {
		return false, nil
	}
	return true, p.Load(g.loadFrom, "")
}

func (g *fieldStage) Spawn() error {
	var cells []board.CellEntry
	for y := uint32(1); y <= 14; y++ {
		c, _ := g.grid.CellIndex(3, y)
		cells = append(cells, board.CellEntry{Kind: "wall", Cell: c})
	}
	g.board.Seed(board.Layout{Default: "grass", Cells: cells})
	return nil
}

func (g *fieldStage) Update(ctx goke.RunCtx, d time.Duration) {
	g.world.RunPlan(ctx, d)
	g.collision.RunPlan(ctx, d)
	g.board.RunPlan(ctx, d)
	ctx.Sync()
}

func (g *fieldStage) Stack() game.Scenes {
	if g.stack == nil {
		g.stack, _ = game.NewStack()
	}
	return g.stack
}

// solidCells counts the cells the world's Field holds solid for a walker anywhere on the board.
func (g *fieldStage) solidCells() int {
	n := 0
	all := geom.NewAABBAt(geom.NewVec(0, 0), 6*cellSize, 16*cellSize)
	g.world.Field().Solid(world.Layers(board.Land), all, func(world.FieldBox) bool { n++; return true })
	return n
}

func TestField_ALoadedBoardIsSolidWhereItWasSavedWithNoEntityForIt(t *testing.T) {
	path := t.TempDir() + "/save"
	grid := board.DefaultGrids{}.Square(6, 16, cellSize)

	fresh := &fieldStage{grid: grid}
	eng := engine.NewEngine(oneStageGame{stage: fresh, props: game.Props{}})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if got := fresh.solidCells(); got != 14 {
		t.Fatalf("fresh board is solid in %d cells, want the 14 of the wall", got)
	}
	if err := eng.Persistence().Save(path, ""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded := &fieldStage{grid: grid, loadFrom: path}
	eng2 := engine.NewEngine(oneStageGame{stage: loaded, props: game.Props{}})
	if err := eng2.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	for range 3 {
		if err := eng2.Update(); err != nil {
			t.Fatalf("Update: %v", err)
		}
	}
	if got := loaded.solidCells(); got != 14 {
		t.Errorf("loaded board is solid in %d cells, want the 14 of the wall", got)
	}
	if got := loaded.world.Res.Telemetry.Count; got != 0 {
		t.Errorf("telemetry counts %d entities, want none: the wall is cells", got)
	}
}
