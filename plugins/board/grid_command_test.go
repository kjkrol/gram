package board_test

import (
	"image/color"
	"testing"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/render"
)

func TestGrid_ShowsAndHidesTheGrid(t *testing.T) {
	bw, _ := boardtest.SquareWorld(t)
	if err := bw.World.Carry(bw.Board); err != nil { // as the engine does for every plugin used
		t.Fatal(err)
	}
	state := &look.RenderState{ShowGridLines: true}
	bw.Board.Res.Render = state
	for i, want := range []bool{false, true} {
		if !bw.World.Carrier().Put(command.Nobody, board.Grid{Shown: want}) {
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
	if err := bw.World.Carry(bw.Board); err != nil {
		t.Fatal(err)
	}
	bw.Board.NewAtlas(boardtest.CellSize).Close()
	view := bw.Board.HoldsView()
	cam := cameratest.NewFromSpace(6*boardtest.CellSize, 16*boardtest.CellSize, 0)
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
		if !bw.World.Carrier().Put(command.Nobody, board.Holds{Shown: want > 0}) {
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
	if !bw.Board.Read(0).GridShown() {
		t.Error("the grid starts hidden, want shown")
	}
	bw.Board.Write().ShowGrid(false)
	a := bw.Board.NewAtlas(boardtest.CellSize)
	a.Grid(board.Line{Color: color.RGBA{R: 255, A: 255}, Width: 2})
	a.Close()
	if bw.Board.Read(0).GridShown() || bw.Board.Res.Render.ShowGridLines {
		t.Error("the grid hidden before the atlas closed is drawn after it")
	}
	bw.Board.Write().ShowGrid(true)
	if _, width, shown := bw.Board.Res.Render.Grid(); !shown || width != 2 {
		t.Errorf("ShowGrid(true) after the atlas closed draws the grid %v, %v pixels wide; want "+
			"it in the atlas's line, 2", shown, width)
	}
}
