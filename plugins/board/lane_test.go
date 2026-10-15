package board_test

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/world"
)

// a band 8 wide along the middle row of cells 10 wide, its railings 10/32 thick on either side
func TestBoard_ALaneIsTheBandLessItsRailingsOnItsOwnLevel(t *testing.T) {
	inf := math.Inf(1)
	railed := 8 - 2*10.0/32
	for _, tc := range []struct {
		name         string
		height       float64
		keeps        cell.Domain
		ground, deck float64
	}{
		{name: "on the ground", height: 0, keeps: cell.Land, ground: railed, deck: inf},
		{name: "raised", height: 10, keeps: cell.Land, ground: inf, deck: railed},
		{name: "railed against boats", height: 0, keeps: cell.Water, ground: 8, deck: inf},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := grid.DefaultGrids{}.Square(5, 3, 10)
			brd := banded(t, g, tc.height, tc.keeps)
			along := [2]cell.ID{g.At(1, 1), g.At(2, 1)}
			if got := brd.Lane(along[0], along[1], cell.Land, false); got != tc.ground {
				t.Errorf("along the band on the ground's level: %v, want %v", got, tc.ground)
			}
			if got := brd.Lane(along[1], along[0], cell.Land, true); got != tc.deck {
				t.Errorf("along the band on the decks' level: %v, want %v", got, tc.deck)
			}
			if got := brd.Lane(g.At(2, 0), g.At(2, 1), cell.Land, tc.height > 0); got != inf {
				t.Errorf("across the band: %v, want no lane", got)
			}
			if got := brd.Lane(along[0], along[1], cell.Water, tc.height > 0); got != inf {
				t.Errorf("for a boat the band does not carry: %v, want no lane", got)
			}
		})
	}
}

func TestBoard_AUnitBiggerThanACellsSidePanics(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 50, Height: 30},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 1, MaxSize: 10},
		Heights:  true,
	})
	p := board.NewPlugin(grid.DefaultGrids{}.Square(5, 3, 10), &cell.MultipleOccupancy{}, w)
	at := func(v geom.Vec) geom.Vec { return v }
	board.NewUnits(p, board.Shape{Size: 10}, at)
	defer func() {
		if recover() == nil {
			t.Error("a unit 11 across on cells 10 wide did not panic")
		}
	}()
	board.NewUnits(p, board.Shape{Size: 6, Width: 11}, at)
}

func banded(t *testing.T, g grid.Grid, height float64, keeps cell.Domain) *board.Board {
	t.Helper()
	wd := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 50, Height: 30},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 1, MaxSize: 10},
		Heights:  true,
	})
	p := board.NewPlugin(g, &cell.MultipleOccupancy{}, wd)
	p.Board().SetAll(cell.Kind{Cost: 1, Allows: cell.Water})
	west, _ := grid.Direction(g, -1, 0)
	east, _ := grid.Direction(g, 1, 0)
	floor := cell.Floor{Allows: cell.Land, Height: height, Links: 1<<west | 1<<east, Width: 8}
	if height > 0 {
		floor.Thickness = 1
	}
	decks := board.NewEntities(p, board.Shape{Size: 10}, func(c cell.ID) cell.ID { return c })
	decks.Define("deck", comp.Const(floor), comp.Const(cell.Rail{Keeps: keeps, Height: 1}))
	for x := range uint32(5) {
		wd.Seed(decks.Named("deck").Entry(g.At(x, 1)))
	}
	ctx := boardtest.NewInstallCtx()
	if err := wd.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := p.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if err := wd.Populate(); err != nil {
		t.Fatal(err)
	}
	ctx.ECS().Setup(ctx.Systems()...)
	ctx.ECS().SetPlan(func(rc goke.RunCtx, d time.Duration) {
		wd.RunPlan(rc, d)
		p.RunPlan(rc, d)
		rc.Sync()
		wd.Clock().Replay(rc, d)
	})
	ctx.ECS().Tick(time.Second / 60)
	return p.Board()
}
