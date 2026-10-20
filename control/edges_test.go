package control_test

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/control"
)

func TestEdges_ACornerIsOneEdgeWithinCornerReachOfTheNext(t *testing.T) {
	ctx := control.Context{Screen: geom.NewVec(1000, 800)}
	for _, c := range []struct {
		x, y   float64
		corner control.Corner
		sx, sy float32
	}{
		{15, 100, control.TopLeft, -1, -1},
		{100, 15, control.TopLeft, -1, -1},
		{15, 400, control.NoCorner, 0, 0},
		{200, 15, control.NoCorner, 0, 0},
		{985, 785, control.BottomRight, 1, 1},
		{985, 100, control.TopRight, 1, -1},
		{100, 785, control.BottomLeft, -1, 1},
		{60, 60, control.TopLeft, -1, -1},
	} {
		ctx.Cursor = geom.NewVec(c.x, c.y)
		e := ctx.Edges()
		sx, sy := e.Corner.Signs()
		if e.Corner != c.corner || sx != c.sx || sy != c.sy {
			t.Errorf("at (%v, %v) the corner is %v with signs (%v, %v), want %v (%v, %v)",
				c.x, c.y, e.Corner, sx, sy, c.corner, c.sx, c.sy)
		}
	}
	ctx.Cursor = geom.NewVec(15, 100)
	if e := ctx.Edges(); !e.Left || e.Top {
		t.Errorf("at (15, 100) the edges are %+v, want the left strip alone, the top not within it", e)
	}
	ctx.Cursor = geom.NewVec(60, 60)
	if e := ctx.Edges(); e.Left || e.Top || !e.Any() {
		t.Errorf("at (60, 60) the edges are %+v, want no strip and yet an edge: the corner", e)
	}
	ctx.Screen = geom.NewVec(200, 200) // a small picture: a corner is a quarter of a side
	if e := ctx.Edges(); e.Corner != control.NoCorner {
		t.Errorf("at (60, 60) of a 200-pixel picture the corner is %v, want none past 50", e.Corner)
	}
	if side := control.CornerSide(200, 200); side != 50 {
		t.Errorf("the corner of a 200-pixel picture is %v a side, want 50", side)
	}
}

func TestEdges_TheMarginsAreInShownPixelsAtAResolution(t *testing.T) {
	ctx := control.Context{Screen: geom.NewVec(2000, 1600), Resolution: 2}
	for _, c := range []struct {
		x, y   float64
		left   bool
		corner control.Corner
	}{
		{15, 800, false, control.NoCorner}, // the dead zone, 20 pixels
		{50, 800, true, control.NoCorner},  // the strip, 120 pixels
		{130, 800, false, control.NoCorner},
		{230, 230, false, control.TopLeft}, // the corner, 240 a side
		{250, 230, false, control.NoCorner},
	} {
		ctx.Cursor = geom.NewVec(c.x, c.y)
		if e := ctx.Edges(); e.Left != c.left || e.Corner != c.corner {
			t.Errorf("at (%v, %v) of 2000 x 1600 at a resolution of 2 the edges are %+v, want "+
				"left %v in corner %v", c.x, c.y, e, c.left, c.corner)
		}
	}
	if px := ctx.Pixels(8); px != 16 {
		t.Errorf("8 shown pixels are %v of the camera's, want 16", px)
	}
	if s := ctx.Shown(geom.NewVec(6, 8)); s != geom.NewVec(3, 4) {
		t.Errorf("(6, 8) of the camera's pixels is %v shown, want (3, 4)", s)
	}
	plain := control.Context{}
	if plain.Pixels(8) != 8 || plain.Shown(geom.NewVec(6, 8)) != geom.NewVec(6, 8) {
		t.Error("a Context of no resolution converts as 1")
	}
}

func TestEdges_TheDepthGrowsFromTheStripsInnerBoundToTheWindowsEdge(t *testing.T) {
	ctx := control.Context{Screen: geom.NewVec(1000, 800)}
	for _, c := range []struct {
		x, y  float64
		depth float64
	}{
		{60, 400, 0},    // the strip's inner bound, no edge
		{59, 400, 0.02}, // just in
		{35, 400, 0.5},
		{10, 400, 1}, // the dead zone's edge
		{965, 400, 0.5},
		{500, 35, 0.5},
		{500, 765, 0.5},
		{500, 400, 0},
		{15, 100, 105.0 / 110}, // the corner: the deeper way, of its square 120 a side
		{100, 15, 105.0 / 110},
		{60, 60, 60.0 / 110},
	} {
		ctx.Cursor = geom.NewVec(c.x, c.y)
		if e := ctx.Edges(); math.Abs(e.Depth-c.depth) > 1e-9 {
			t.Errorf("at (%v, %v) the depth is %v, want %v", c.x, c.y, e.Depth, c.depth)
		}
	}
	ctx.FillsScreen = true
	ctx.Cursor = geom.NewVec(0, 400)
	if e := ctx.Edges(); !e.Left || e.Depth != 1 {
		t.Errorf("filling the screen, at the very edge the edges are %+v, want left at a depth of 1",
			e)
	}
}
