package board_test

import (
	"image/color"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type surfaced func(a *board.Atlas, b *board.Plugin)

var _ board.Surface = surfaced(nil)

func (s surfaced) Look(a *board.Atlas, b *board.Plugin) { s(a, b) }

// a Stage of a board over g, the surfaces noted in its Figures link, the effects in its Effects
func surfaceStage(
	t *testing.T, g grid.Grid, figures, effects func(b *board.Plugin),
) *board.Plugin {
	t.Helper()
	var b *board.Plugin
	st := stage.New("surface").
		World(world.Config{
			Space:    world.SpaceCfg{Width: 256, Height: 256},
			Entities: world.EntitiesCfg{MaxCount: 4, MinSize: 1, MaxSize: 20},
			Heights:  true,
		}).
		Plugins(func(w *world.Plugin) { b = board.NewPlugin(g, &cell.MultipleOccupancy{}, w) }).
		Figures(func() { figures(b) }).
		Effects(func() { effects(b) }).
		Update(func(goke.RunCtx, time.Duration) {})
	if err := engine.NewEngine(boardtest.OneStageGame{Stage: st}).Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return b
}

var squareOf4 = grid.DefaultGrids{}.Square(4, 4, 32)

func TestSurfaces_ALaterSurfaceTakesWhatItSetsAndLeavesTheRest(t *testing.T) {
	red := board.Line{Color: color.RGBA{R: 200, A: 255}, Width: 3}
	b := surfaceStage(t, squareOf4, func(b *board.Plugin) {
		b.Surfaces(
			surfaced(func(a *board.Atlas, _ *board.Plugin) {
				a.Grid(board.Line{Color: color.RGBA{G: 200, A: 255}, Width: 2})
				a.Holds(5)
			}),
			surfaced(func(a *board.Atlas, _ *board.Plugin) { a.Grid(red) }))
	}, func(*board.Plugin) {})
	rs := b.Res.Render
	if rs.GridLine != red.Color || rs.GridWidth != red.Width {
		t.Errorf("the grid is drawn in %v %v pixels wide, want the later surface's %v", rs.GridLine,
			rs.GridWidth, red)
	}
	if got := holdWidth(t, b); got != 5 {
		t.Errorf("a cell held is outlined %v pixels wide, want the earlier surface's 5", got)
	}
}

func TestSurfaces_NoneDrawsTheDefaults(t *testing.T) {
	for _, g := range []grid.Grid{squareOf4, grid.DefaultGrids{}.Hex(4, 4, 20)} {
		b := surfaceStage(t, g, func(*board.Plugin) {}, func(*board.Plugin) {})
		want := board.DefaultGridLine(g)
		if rs := b.Res.Render; rs.GridLine != want.Color || rs.GridWidth != want.Width {
			t.Errorf("without surfaces the grid is drawn in %v %v pixels wide, want %v",
				rs.GridLine, rs.GridWidth, want)
		}
		if got := holdWidth(t, b); got != board.DefaultHoldWidth {
			t.Errorf("without surfaces a cell held is outlined %v pixels wide, want %v", got,
				board.DefaultHoldWidth)
		}
	}
	hex := grid.DefaultGrids{}.Hex(4, 4, 20)
	if board.DefaultGridLine(squareOf4) == board.DefaultGridLine(hex) {
		t.Error("a hex grid's default line is a square one's, want it as dark as two outlines")
	}
}

func TestSurfaces_AreNotedOnlyInTheFiguresLink(t *testing.T) {
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "surfaces noted") {
			t.Errorf("surfaces noted among the effects: %v, want a panic naming them", r)
		}
	}()
	surfaceStage(t, squareOf4, func(*board.Plugin) {}, func(b *board.Plugin) {
		b.Surfaces(surfaced(func(*board.Atlas, *board.Plugin) {}))
	})
}

func TestSetAtlas_AForeignAtlasDrawsTheDefaults(t *testing.T) {
	bw, _ := boardtest.SquareWorld(t)
	a := bw.Board.NewAtlas(boardtest.CellSize)
	a.Grid(board.Line{Color: color.RGBA{R: 255, A: 255}, Width: 4})
	a.Close()
	foreign := render.NewAtlas()
	foreign.Close()
	bw.Board.Write().SetAtlas(foreign)
	want := board.DefaultGridLine(bw.Board.Read(0).Board().Grid)
	if rs := bw.Board.Res.Render; rs.GridLine != want.Color || rs.GridWidth != want.Width {
		t.Errorf("over a foreign atlas the grid is drawn in %v %v pixels wide, want %v",
			rs.GridLine, rs.GridWidth, want)
	}
}

// how wide the holds view outlines the cell a unit holds, its lines' quads being a pixel wider
func holdWidth(t *testing.T, b *board.Plugin) float32 {
	t.Helper()
	g := b.Read(0).Board().Grid
	c := g.At(1, 1)
	b.Write().Occupancy().Enter(c, false, 7, cell.Land)
	b.Write().ShowHolds(true)
	cam := cameratest.NewFromSpace(256, 256, 0)
	view := b.HoldsView()
	view.(plugin.Publisher).Publish()
	view.(plugin.Publisher).Swap()
	var f render.Frame
	f.Reset(cam)
	view.Compose(&f, cam)
	var across float64
	f.Each(func(_ render.At, v []render.Vertex) {
		across = math.Hypot(float64(v[0].DstX-v[2].DstX), float64(v[0].DstY-v[2].DstY))
	})
	if across == 0 {
		t.Fatal("the holds view drew nothing of a cell held")
	}
	return float32(math.Round(across)) - 1
}
