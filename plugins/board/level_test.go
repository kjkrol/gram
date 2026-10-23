package board_test

import (
	"testing"
	"testing/fstest"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/board/unit"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
)

type standingRow struct {
	At cell.ID `yaml:"at"`
}

type unitRow struct {
	Start cell.ID `yaml:"start"`
}

func TestLay_StandsTheEntitiesOnTheBoardItLays(t *testing.T) {
	g := grid.DefaultGrids{}.Square(5, 1, boardtest.CellSize)
	level := fstest.MapFS{"level.yaml": {Data: []byte(`
board:
  default: grass
  legend: {".": grass, "_": pit}
  cells: |
    .._..
  entities:
    - {kind: trapdoor, group: trapdoors, at: [2, 0]}
units:
  - {kind: walker, name: ann, start: [0, 0]}
`)}}
	prepare := func(w *world.Plugin, brd *board.Plugin) []act.Rule {
		brd.Read(0).CellKinds().Define("grass", grass)
		brd.Read(0).CellKinds().Define("pit", pit)
		shape := board.Shape{Size: boardtest.UnitSize}
		standing := board.NewEntities(brd, shape, func(r standingRow) cell.ID { return r.At })
		standing.Define("trapdoor", comp.Const(cell.Floor{Allows: cell.Land}))
		units := board.NewUnits(brd, shape, func(r unitRow) geom.Vec {
			return g.CellCenter(r.Start)
		})
		units.Define("walker", unit.Mover{Domain: cell.Land}, steering.Steering{})
		laid, err := brd.Lay(level, "level.yaml")
		if err != nil {
			t.Fatal(err)
		}
		if err := board.Spawn[standingRow](laid, nil); err != nil {
			t.Fatal(err)
		}
		if err := board.Spawn[unitRow](laid, nil); err != nil {
			t.Fatal(err)
		}
		if err := brd.Populate(); err != nil {
			t.Fatal(err)
		}
		return nil
	}
	bw := boardtest.NewWorldWith(t, g, 5*boardtest.CellSize, boardtest.CellSize, prepare, nil)
	bw.Tick()
	brd := bw.Board.Read(0).Board()
	if got := brd.Kind(g.At(1, 0)).Name.String(); got != "grass" {
		t.Errorf("beside the pit: %q, want grass", got)
	}
	if b := brd.Built(g.At(2, 0)); b.Floor.Allows != cell.Land {
		t.Errorf("over the pit: %+v, want the trapdoor's Floor", b)
	}
	if k := brd.Kind(g.At(2, 0)); k.Name.String() != "pit" || !k.Admits(cell.Land) {
		t.Errorf("under the trapdoor: %q admitting %v, want a pit admitting Land", k.Name, k.Allows)
	}
	if n := len(bw.Snapshot()); n != 2 {
		t.Errorf("%d kinds of entity in the world, want the trapdoor and the walker", n)
	}
}
