package board_test

import (
	"image/color"
	"testing"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/internal/boardtest"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

func TestMiniature_IsTheBoardFlatFromAbove(t *testing.T) {
	gputest.Need(t)
	bw, wall := boardtest.SquareWorld(t)
	if img, _ := bw.Board.Write().Miniature(); img != nil {
		t.Error("a miniature before the board's atlas is closed")
	}
	green := color.RGBA{G: 200, A: 255}
	grey := color.RGBA{R: 90, G: 90, B: 90, A: 255}
	kinds := bw.Board.Read(0).CellKinds()
	kinds.Define("grass", cell.Kind{Pace: 1, Allows: cell.Land})
	kinds.Define("wall", boardtest.Wall)
	brd := bw.Board.Read(0).Board()
	brdW := bw.Board.Write().Board()
	brdW.SetAll(kinds.Named("grass").Kind())
	brdW.Set(wall, kinds.Named("wall").Kind())
	a := bw.Board.NewAtlas(boardtest.CellSize)
	a.Add("grass", render.Solid(green))
	a.Add("wall", render.Solid(grey))
	a.Close()
	img, perUnit := bw.Board.Write().Miniature()
	if img == nil || perUnit != 1 {
		t.Fatalf("the miniature is %v at %v a unit, want one at 1", img, perUnit)
	}
	b := img.Bounds()
	if b.Dx() != 6*boardtest.CellSize || b.Dy() != 16*boardtest.CellSize {
		t.Fatalf("the miniature is %v, want the board's 192 by 512", b)
	}
	pix := make([]byte, 4*b.Dx()*b.Dy())
	img.ReadPixels(pix)
	at := func(x, y float32) [3]byte {
		k := 4 * (int(y)*b.Dx() + int(x))
		return [3]byte{pix[k], pix[k+1], pix[k+2]}
	}
	middle := brd.Grid.CellCenter(wall)
	if got := at(middle.X, middle.Y); got != [3]byte{grey.R, grey.G, grey.B} {
		t.Errorf("the wall's cell shows %v, want its grey", got)
	}
	if got := at(16, 16); got != [3]byte{green.R, green.G, green.B} {
		t.Errorf("a grass cell shows %v, want its green", got)
	}
}
