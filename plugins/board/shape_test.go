package board_test

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
)

func TestShape_BoxIsSquareBySizeAndWideByWidth(t *testing.T) {
	square := board.Shape{Size: 20}.Box(geom.NewVec(100, 50))
	if square.TopLeft != geom.NewVec(90, 40) || square.BottomRight != geom.NewVec(110, 60) {
		t.Errorf("a square box of 20 at (100, 50) is %v..%v", square.TopLeft, square.BottomRight)
	}
	tall := board.Shape{Size: 32, Width: 16}.Box(geom.NewVec(100, 50))
	if tall.TopLeft != geom.NewVec(92, 34) || tall.BottomRight != geom.NewVec(108, 66) {
		t.Errorf("a box 16 across and 32 down at (100, 50) is %v..%v", tall.TopLeft, tall.BottomRight)
	}
}
