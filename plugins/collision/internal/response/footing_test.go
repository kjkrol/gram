package response

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/world"
)

// edge is ground that takes nothing left of x: the area of a box lying there.
type edge struct{ x float64 }

func (e edge) Overhang(_ world.Layers, box geom.AABB) float64 {
	over := min(box.BottomRight.X, e.x) - box.TopLeft.X
	return max(over, 0) * (box.BottomRight.Y - box.TopLeft.Y)
}

func body(x float64) Body { return Body{Box: geom.NewAABBAt(geom.NewVec(x, 0), 10, 10)} }

// A side the push would put further over the edge holds, and the other one goes the whole way.
func TestFooting_TheSideAtTheEdgeHoldsAndTheOtherGoesTheWholeWay(t *testing.T) {
	g := edge{x: 100}
	push, holdA, holdB := Footing(g, body(100), body(108), geom.NewVec(-2, 0))
	if !holdA || holdB {
		t.Fatalf("holds %v, %v; want the side at the edge alone", holdA, holdB)
	}
	if push != geom.NewVec(-4, 0) {
		t.Errorf("push %v, want {-4,0}: the whole way for the other", push)
	}
}

// Away from the edge both sides take their half; an immovable side holds nothing and doubles
// nothing.
func TestFooting_AwayFromTheEdgeOrAgainstAWallThePushStays(t *testing.T) {
	g := edge{x: 0}
	if push, holdA, holdB := Footing(g, body(100), body(108), geom.NewVec(-2, 0)); holdA || holdB || push != geom.NewVec(-2, 0) {
		t.Errorf("away from the edge: %v, %v, %v; want {-2,0} and no hold", push, holdA, holdB)
	}
	wall := body(108)
	wall.Immovable = true
	g = edge{x: 100}
	if push, holdA, holdB := Footing(g, body(100), wall, geom.NewVec(-2, 0)); !holdA || holdB || push != geom.NewVec(-2, 0) {
		t.Errorf("against a wall: %v, %v, %v; want {-2,0}, A held", push, holdA, holdB)
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
