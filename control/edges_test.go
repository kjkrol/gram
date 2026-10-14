package control_test

import (
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
