package board_test

import (
	"reflect"
	"testing"

	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/board/look"
)

// Grid is the board's own command, on B by default: given, it hides the grid at the board's
// next tick, and given again shows it.
func TestGrid_ShowsAndHidesTheGrid(t *testing.T) {
	bw, _ := boardtest.SquareWorld(t)
	keys := bw.Board.DefaultBindings()
	if len(keys) != 1 || keys[0].Command() != reflect.TypeFor[board.Grid]() || keys[0].Trigger != control.Trigger(control.KeyPress{Key: control.KeyB}) {
		t.Fatalf("the board's default keys are %+v, want B for Grid alone", keys)
	}
	if err := bw.World.Carry(bw.Board); err != nil { // as the engine does for every plugin used
		t.Fatal(err)
	}
	state := &look.RenderState{ShowGridLines: true}
	bw.Board.Res.Render = state
	for i, want := range []bool{false, true} {
		if !bw.World.Carrier().Put(control.Nobody, board.Grid{}) {
			t.Fatal("the world carries no board.Grid")
		}
		bw.Tick()
		if state.ShowGridLines != want {
			t.Errorf("after %d Grid commands the grid shows %v, want %v", i+1, state.ShowGridLines, want)
		}
	}
}
