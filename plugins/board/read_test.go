package board_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/world"
)

// what a goroutine reads of a cell and of the step east from it
type reading struct {
	kind, ground, deck, bare cell.Kind
	built                    cell.Built
	way                      cell.Way
	along, reaches, fenced   bool
	lane                     float32
	decks                    int
	top                      [4]float32
	level                    float32
	veiled                   int
}

func readAll(r *board.Read, g grid.Grid, out []reading) {
	brd := r.Board()
	g.EachCell(func(c cell.ID) {
		i, _ := brd.Ordinal(c)
		east, _ := g.Toward(c, 0)
		at := brd.CellCenter(c)
		rd := reading{kind: brd.Kind(c), ground: brd.KindAt(c, false), deck: brd.KindAt(c, true),
			bare: brd.Bare(c), built: brd.Built(c), way: brd.Way(c), along: brd.Along(c, east),
			reaches: brd.Reaches(c, east), fenced: brd.Fenced(c, east, cell.Land, true),
			lane: brd.Lane(c, east, cell.Land, true)}
		brd.DecksOver(at, func(cell.ID, cell.Floor) { rd.decks++ })
		rd.top, rd.level = brd.Map().Top(c)
		r.Cover().Walk(at, geom.NewVec(1, 0), 64, world.Layers(cell.Land),
			func(_, _, _, _, _ float32) bool { rd.veiled++; return true })
		out[i] = rd
	})
}

func TestRead_TheWorkersReadTheBoardAtOnceAsTheGameDoes(t *testing.T) {
	g := grid.DefaultGrids{}.Square(8, 8, 16)
	rows := band(g, [2]int{1, 0}, 6, 4, g.At(1, 3), g.At(2, 3), g.At(3, 3), g.At(4, 3), g.At(5, 3))
	d := newDecked(t, g, 128, 128, board.Bridges{}, rows...)
	write := d.p.Write().Board()
	write.Set(g.At(6, 6), boardtest.Wall)
	write.SetWay(g.At(2, 6), cell.Way{Kind: water, Width: 4, Links: 1})
	d.tick()
	d.tick()
	want := make([]reading, g.CellCount())
	readAll(d.p.Read(0), g, want)
	pool := boardtest.Pool
	got := make([][]reading, 2*pool.Size())
	for i := range got {
		got[i] = make([]reading, g.CellCount())
	}
	b := pool.Begin(func(w, i int) {
		for range 20 {
			readAll(d.p.Read(w), g, got[i])
		}
	}, nil)
	b.Add(len(got))
	b.Wait()
	for i := range got {
		for c := range got[i] {
			if got[i][c] != want[c] {
				t.Fatalf("task %d read cell %d as %+v, the game's goroutine as %+v", i, c,
					got[i][c], want[c])
			}
		}
	}
	if want[g.CellCount()/2].decks == 0 && want[3*8+2].decks == 0 {
		t.Error("no deck read anywhere: the board proves nothing")
	}
}
