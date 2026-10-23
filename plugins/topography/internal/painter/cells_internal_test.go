package painter

import (
	"image/color"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/gpu/gputest"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/render"
)

// the cells' blocks as the bake's shaders read them, laid whole and then a changed cell alone
func TestCells_HoldWhatTheBakeReadsOfEachCell(t *testing.T) {
	gputest.Need(t)
	st := map[cell.Name]Style{}
	g := grid.DefaultGrids{}.Square(3, 3, 32)
	brd, brdW := newBoard(g)
	land := styled(st, cell.Kind{Name: cell.Named("c1"), Allows: cell.Land, SpriteID: 1},
		Style{Spread: 0.3})
	sea := styled(st, cell.Kind{Name: cell.Named("c2"), Allows: cell.Water, SpriteID: 2},
		Style{Under: true, Shine: 0.8})
	river := styled(st, cell.Kind{Name: cell.Named("c3"), Allows: cell.Land | cell.Water,
		SpriteID: 3}, Style{Shine: 0.9, Flow: 22, MixWith: "c2"})
	brdW.SetAll(land)
	brdW.Set(g.At(2, 2), sea)
	brdW.SetWay(g.At(1, 1), cell.Way{Kind: river, Width: 7.5, Links: 0x03, Fade: 0.2, Mix: 0.6})
	reliefFor(brd).SetHeights(func(p geom.Vec) float64 { return 10 + p.X/4 })
	atlas := render.NewAtlas()
	for id := range render.SpriteID(4) {
		atlas.Add(id, 8, render.Solid(color.RGBA{A: 255}))
	}
	atlas.Close()
	d := New(brd, reliefFor(brd), true, st)
	d.kinds = kindsOf{land, sea, river}
	if !d.refresh(atlas) || d.refresh(atlas) {
		t.Fatal("the cells laid at first and again unchanged, want laid once")
	}
	pix := make([]byte, 4*12*6)
	read := func(x, y int) []byte {
		d.cells.img.ReadPixels(pix)
		out := make([]byte, 0, cellBytes)
		for row := range 2 {
			at := 4 * ((2*y+row)*12 + 4*x)
			out = append(out, pix[at:at+16]...)
		}
		return out
	}
	b := read(1, 1)
	if b[cellSprite] != 1 || b[cellSpread] != 153 || b[cellFlags] != flagRuns|flagMixes {
		t.Errorf("the middle cell holds sprite %d, spread %d, flags %b; want land's 1, 0.3 as "+
			"153 and a way that mixes", b[cellSprite], b[cellSpread], b[cellFlags])
	}
	if b[cellWaySprite] != 3 || b[cellLinks] != 0x03 || b[cellWidth] != 224 ||
		b[cellWidth+1] != 1 || b[cellFade] != 51 || b[cellMix] != 153 {
		t.Errorf("the middle cell's way is %v, want the river's sprite 3, links 3, 7.5 wide as "+
			"480, faded 51 and mixed 153", b[cellWaySprite:cellMixShine+1])
	}
	if b[cellWayShine] != 230 || b[cellWayFlow] != 96 || b[cellWayFlow+1] != 1 ||
		b[cellMixSprite] != 2 || b[cellMixShine] != 204 {
		t.Errorf("the way shines %d, flows %d, %d and mixes into %d shining %d; want 230, 352 "+
			"and the sea's 2 at 204", b[cellWayShine], b[cellWayFlow], b[cellWayFlow+1],
			b[cellMixSprite], b[cellMixShine])
	}
	low, high := reliefFor(brd).Extent()
	corner := func(b []byte, k int) float32 {
		v := float32(b[cellGround+2*k]) + 256*float32(b[cellGround+2*k+1])
		return float32(low) + v/65535*float32(high-low)
	}
	if h := corner(b, 3); h < 25.9 || h > 26.1 {
		t.Errorf("the middle cell's bottom-right corner stands at %v, want 26", h)
	}
	if s := read(2, 2); s[cellFlags] != flagUnder || s[cellShine] != 204 {
		t.Errorf("the sea holds flags %b and shine %d, want under and 204", s[cellFlags],
			s[cellShine])
	}
	brdW.Set(g.At(0, 0), sea)
	if !d.refresh(atlas) || len(d.cells.changed) != 1 || d.cells.changed[0] != 0 {
		t.Fatalf("a cell changed, the cells laid %v anew, want it alone", d.cells.changed)
	}
	if s := read(0, 0); s[cellSprite] != 2 || s[cellFlags] != flagUnder {
		t.Errorf("the changed cell holds sprite %d and flags %b, want the sea's", s[cellSprite],
			s[cellFlags])
	}
}
