package board_test

import (
	"reflect"
	"testing"

	"github.com/kjkrol/gram/control"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/render"
)

func TestGrid_ShowsAndHidesTheGrid(t *testing.T) {
	bw, _ := boardtest.SquareWorld(t)
	keys := bw.Board.DefaultBindings()
	if len(keys) == 0 || keys[0].Type() != reflect.TypeFor[board.Grid]() ||
		keys[0].Trigger != control.Trigger(control.KeyPress{Key: control.KeyB}) {
		t.Fatalf("the board's default keys are %+v, want B for Grid first", keys)
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

func TestHolds_ShowsAndHidesTheCellsHeld(t *testing.T) {
	bw, _ := boardtest.SquareWorld(t, boardtest.Mover{})
	keys := bw.Board.DefaultBindings()
	shiftO := control.KeyPress{Key: control.KeyO, Mods: control.Mods{Shift: true}}
	if len(keys) != 2 || keys[1].Type() != reflect.TypeFor[board.Holds]() ||
		keys[1].Trigger != control.Trigger(shiftO) {
		t.Fatalf("the board's default keys are %+v, want Shift+O for Holds second", keys)
	}
	if err := bw.World.Carry(bw.Board); err != nil {
		t.Fatal(err)
	}
	atlas := bw.Board.NewAtlas(boardtest.CellSize)
	atlas.Close()
	bw.Board.WithRenderer(atlas)
	view, ok := bw.Board.HoldsRenderer().(render.Source)
	if !ok {
		t.Fatal("the cells held are drawn by no render.Source")
	}
	cam := icamera.NewFromSpace(6*boardtest.CellSize, 16*boardtest.CellSize, 0)
	lines := func() int {
		var f render.Frame
		f.Reset(cam)
		view.Compose(&f, cam)
		return f.Len()
	}
	bw.Tick()
	if n := lines(); n != 0 {
		t.Errorf("the cells held start shown: %d pieces", n)
	}
	for i, want := range []int{4, 0} {
		if !bw.World.Carrier().Put(control.Nobody, board.Holds{}) {
			t.Fatal("the world carries no board.Holds")
		}
		bw.Tick()
		if n := lines(); n != want {
			t.Errorf("after %d Holds commands the view draws %d lines, want %d", i+1, n, want)
		}
	}
}
