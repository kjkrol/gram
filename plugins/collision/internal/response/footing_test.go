package response

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/world"
)

type edge struct{ x float32 }

func (e edge) Overhang(_ world.Layers, box geom.AABB) float32 {
	over := min(box.BottomRight.X, e.x) - box.TopLeft.X
	return max(over, 0) * (box.BottomRight.Y - box.TopLeft.Y)
}

func body(x float32) Body { return Body{Box: geom.NewAABBAt(geom.NewVec(x, 0), 10, 10)} }

func TestFooting_TheSideAtTheEdgeHoldsAndTheOtherGoesTheWholeWay(t *testing.T) {
	g := edge{x: 100}
	share, holdA, holdB := Footing(g, body(100), body(108), geom.NewVec(-2, 0), 0.5)
	if !holdA || holdB {
		t.Fatalf("holds %v, %v; want the side at the edge alone", holdA, holdB)
	}
	if share != 0 {
		t.Errorf("share %v, want 0: the whole way for the other", share)
	}
}

func TestFooting_AwayFromTheEdgeOrAgainstAWallTheShareStays(t *testing.T) {
	g := edge{x: 0}
	if share, holdA, holdB := Footing(g, body(100), body(108), geom.NewVec(-2, 0), 0.25); holdA ||
		holdB || share != 0.25 {
		t.Errorf("away from the edge: %v, %v, %v; want 0.25 and no hold", share, holdA, holdB)
	}
	wall := body(108)
	wall.Immovable = true
	g = edge{x: 100}
	if share, holdA, holdB := Footing(g, body(100), wall, geom.NewVec(-2, 0), 1); !holdA ||
		holdB || share != 1 {
		t.Errorf("against a wall: %v, %v, %v; want 1, A held", share, holdA, holdB)
	}
}

func TestWorse(t *testing.T) {
	g := edge{x: 100}
	if !Worse(g, 0, body(100).Box, body(95).Box) {
		t.Error("moving over the edge is not worse")
	}
	if Worse(g, 0, body(95).Box, body(100).Box) || Worse(g, 0, body(100).Box, body(100).Box) {
		t.Error("moving back, or staying, is worse")
	}
}
