package world_test

import (
	"testing"

	"github.com/kjkrol/gram/plugins/world"
)

func TestGridPlacement_Place_SquareCount(t *testing.T) {
	p := world.NewGridPlacement(100, 100, 10)

	cases := []struct {
		index int
		wantX float32
		wantY float32
	}{
		{0, 20, 20},
		{1, 70, 20},
		{2, 20, 70},
		{3, 70, 70},
	}
	for _, c := range cases {
		pos := p.Place(c.index, 4)
		if pos.TopLeft.X != c.wantX || pos.TopLeft.Y != c.wantY {
			t.Errorf("Place(%v, 4) = (%v,%v), want (%v,%v)", c.index, pos.TopLeft.X, pos.TopLeft.Y, c.wantX, c.wantY)
		}
	}
}

func TestGridPlacement_Place_NonSquareCount(t *testing.T) {
	p := world.NewGridPlacement(100, 100, 10)

	cases := []struct {
		index int
		wantX float32
		wantY float32
	}{
		{0, 11, 20},
		{4, 44, 70},
	}
	for _, c := range cases {
		pos := p.Place(c.index, 5)
		if pos.TopLeft.X != c.wantX || pos.TopLeft.Y != c.wantY {
			t.Errorf("Place(%v, 5) = (%v,%v), want (%v,%v)", c.index, pos.TopLeft.X, pos.TopLeft.Y, c.wantX, c.wantY)
		}
	}
}

func TestGridPlacement_Place_SingleEntity_Centers(t *testing.T) {
	p := world.NewGridPlacement(100, 100, 10)
	pos := p.Place(0, 1)
	if pos.TopLeft.X != 45 || pos.TopLeft.Y != 45 {
		t.Errorf("Place(0, 1) = (%v,%v), want (45,45) (single cell spanning the whole grid, entity centered)", pos.TopLeft.X, pos.TopLeft.Y)
	}
}
