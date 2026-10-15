package board_test

import (
	"reflect"
	"testing"

	"github.com/kjkrol/ceremony/command"
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
		if !bw.World.Carrier().Put(command.Nobody, board.Grid{}) {
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
	bw.Board.NewAtlas(boardtest.CellSize).Close()
	view := bw.Board.HoldsView()
	cam := icamera.NewFromSpace(6*boardtest.CellSize, 16*boardtest.CellSize, 0)
	var lying []render.At
	lines := func() int {
		var f render.Frame
		f.Reset(cam)
		view.Compose(&f, cam)
		f.Each(func(at render.At, _ []render.Vertex) { lying = append(lying, at) })
		return f.Len()
	}
	bw.Tick()
	if n := lines(); n != 0 {
		t.Errorf("the cells held start shown: %d pieces", n)
	}
	for i, want := range []int{4, 0} {
		if !bw.World.Carrier().Put(command.Nobody, board.Holds{}) {
			t.Fatal("the world carries no board.Holds")
		}
		bw.Tick()
		if n := lines(); n != want {
			t.Errorf("after %d Holds commands the view draws %d lines, want %d", i+1, n, want)
		}
	}
	for _, at := range lying {
		if at.Top() || at.Z() != 0.15 {
			t.Errorf("a hold lies at %v, want over its cell, among what lies there", at)
		}
	}
}

func TestShowGrid_HoldsThroughTheAtlasClosing(t *testing.T) {
	bw, _ := boardtest.SquareWorld(t)
	if !bw.Board.GridShown() {
		t.Error("the grid starts hidden, want shown")
	}
	bw.Board.ShowGrid(false)
	bw.Board.NewAtlas(boardtest.CellSize).Close()
	if bw.Board.GridShown() || bw.Board.Res.Render.ShowGridLines {
		t.Error("the grid hidden before the atlas closed is drawn after it")
	}
	bw.Board.ShowGrid(true)
	if !bw.Board.Res.Render.ShowGridLines {
		t.Error("ShowGrid(true) after the atlas closed draws no grid")
	}
}
