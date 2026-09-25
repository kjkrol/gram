package camera

import (
	"bytes"
	"encoding/gob"
	"github.com/kjkrol/aabbworld"
	contract "github.com/kjkrol/gram/camera"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
)

func testViewport(x, y, w, h float64) contract.AABB {
	return geom.NewAABBAt(geom.NewVec(x, y), w, h)
}

func TestBasicCamera_Translate_WrapsOnToroidalWorld(t *testing.T) {
	surface := geom.NewVec(20, 20)
	c := newBasicCamera(surface, testViewport(2, 2, 4, 4), aabbworld.Torus)

	c.Translate(-10, 0)

	if c.effective.TopLeft.X != 12 {
		t.Errorf("Viewport.TopLeft.X = %v, want 12 (wrapped: (2-10) mod 20)", c.effective.TopLeft.X)
	}
	if c.effective.TopLeft.Y != 2 {
		t.Errorf("Viewport.TopLeft.Y = %v, want 2 (unchanged)", c.effective.TopLeft.Y)
	}
}

func TestBasicCamera_Pan_IsInScreenPixels(t *testing.T) {
	c := NewFromSpace(1000, 1000, 0, testViewport(100, 100, 200, 200))
	c.Pan(10, 0)
	if x := c.Bounds().TopLeft.X; x != 110 {
		t.Errorf("Pan(10, 0) at zoom 1 moved to %v, want 110", x)
	}
	c.ZoomIn(4, 200, 200)
	before := c.Bounds().TopLeft.X
	c.Pan(10, 0)
	if got := c.Bounds().TopLeft.X - before; got != 2.5 {
		t.Errorf("Pan(10, 0) at zoom 4 moved %v world units, want 2.5", got)
	}
	c.Pan(-100000, 0)
	if x := c.Bounds().TopLeft.X; x != 0 {
		t.Errorf("a Pan past the left edge left the window at %v, want it held at 0 like Translate", x)
	}
}

func TestBasicCamera_Translate_ClampsOnEuclideanWorld(t *testing.T) {
	surface := geom.NewVec(20, 20)
	c := newBasicCamera(surface, testViewport(2, 2, 4, 4), 0)

	c.Translate(-10, 0)

	if c.effective.TopLeft.X != 0 {
		t.Errorf("Viewport.TopLeft.X = %v, want 0 (clamped, not wrapped)", c.effective.TopLeft.X)
	}
}

func TestBasicCamera_MoveTo_WrapsOnToroidalWorld(t *testing.T) {
	surface := geom.NewVec(20, 20)
	c := newBasicCamera(surface, testViewport(2, 2, 4, 4), aabbworld.Torus)

	c.MoveTo(18, 2)
	c.Translate(-10, 0)
	if c.effective.TopLeft.X != 8 {
		t.Fatalf("sanity check failed: Viewport.TopLeft.X = %v, want 8", c.effective.TopLeft.X)
	}

	c.Translate(-10, 0)
	if c.effective.TopLeft.X != 18 {
		t.Errorf("Viewport.TopLeft.X = %v, want 18 (wrapped: (8-10) mod 20)", c.effective.TopLeft.X)
	}
}

func TestBasicCamera_Bounds_MatchesConstructedViewportAtZoom1(t *testing.T) {
	surface := geom.NewVec(100, 100)
	viewport := testViewport(10, 10, 30, 20)
	c := newBasicCamera(surface, viewport, 0)

	got := c.Bounds()
	if got.TopLeft != viewport.TopLeft || got.BottomRight != viewport.BottomRight {
		t.Errorf("Bounds() = %+v, want %+v", got, viewport)
	}
}

func TestBasicCamera_ToScreen_NoZoomIsPlainOffset(t *testing.T) {
	surface := geom.NewVec(100, 100)
	c := newBasicCamera(surface, testViewport(10, 10, 30, 20), 0)

	x, y := c.ToScreen(15, 12)
	if x != 5 || y != 2 {
		t.Errorf("ToScreen(15,12) = (%v,%v), want (5,2)", x, y)
	}
}

func TestBasicCamera_ZoomIn_ScalesToScreen(t *testing.T) {
	surface := geom.NewVec(1000, 1000)
	c := newBasicCamera(surface, testViewport(100, 100, 100, 100), 0)

	c.ZoomIn(2, 150, 150)
	if c.Zoom() != 2 {
		t.Fatalf("Zoom() = %v, want 2", c.Zoom())
	}

	x, y := c.ToScreen(125, 125)
	if x != 0 || y != 0 {
		t.Errorf("ToScreen(125,125) at zoom 2 = (%v,%v), want (0,0)", x, y)
	}

	x2, _ := c.ToScreen(126, 125)
	if x2 != 2 {
		t.Errorf("ToScreen(126,125) at zoom 2 = %v, want 2 (scale applied)", x2)
	}
}

func TestBasicCamera_ZoomIn_ThenZoomOut_RoundTrips(t *testing.T) {
	surface := geom.NewVec(1000, 1000)
	c := newBasicCamera(surface, testViewport(100, 100, 100, 100), 0)

	c.ZoomIn(2, 150, 150)
	c.ZoomOut(2, 150, 150)

	if c.Zoom() != 1 {
		t.Errorf("Zoom() after ZoomIn(2);ZoomOut(2) = %v, want 1", c.Zoom())
	}
}

func TestBasicCamera_Translate_ClampsEffectiveWhenZoomedIn(t *testing.T) {
	surface := geom.NewVec(1000, 1000)
	c := newBasicCamera(surface, testViewport(0, 0, 1000, 1000), 0)
	c.ZoomIn(2, 500, 500)

	c.Translate(1000, 0)

	b := c.Bounds()
	if b.BottomRight.X != 1000 {
		t.Errorf("Bounds().BottomRight.X = %v, want 1000 (clamped to world edge)", b.BottomRight.X)
	}
	if w := b.BottomRight.X - b.TopLeft.X; w != 500 {
		t.Errorf("effective width after clamped Translate = %v, want 500 (must not shrink)", w)
	}
}

func TestBasicCamera_ZoomOut_CappedToWorldFit(t *testing.T) {
	surface := geom.NewVec(1000, 1000)
	c := newBasicCamera(surface, testViewport(0, 0, 1000, 1000), 0)

	c.ZoomOut(10, 500, 500)

	if c.Zoom() < 1 {
		t.Errorf("Zoom() = %v, want >= 1 (viewport already equals world size)", c.Zoom())
	}
	if b := c.Bounds(); b.TopLeft.X != 0 || b.TopLeft.Y != 0 || b.BottomRight.X != 1000 || b.BottomRight.Y != 1000 {
		t.Errorf("Bounds() = %+v, want exactly (0,0)-(1000,1000) (capped at world fit)", b)
	}
}

func TestBasicCamera_ZoomIn_AnchorStaysUnderCursor(t *testing.T) {
	surface := geom.NewVec(1000, 1000)
	c := newBasicCamera(surface, testViewport(0, 0, 1000, 1000), 0)

	const anchorX, anchorY = 900, 500
	beforeX, beforeY := c.ToScreen(anchorX, anchorY)

	c.ZoomIn(1.1, anchorX, anchorY)

	afterX, afterY := c.ToScreen(anchorX, anchorY)
	const tol = 1.5
	if diff := afterX - beforeX; diff < -tol || diff > tol {
		t.Errorf("ToScreen(anchor).X after ZoomIn = %v, want ~%v (anchor should stay under the cursor)", afterX, beforeX)
	}
	if diff := afterY - beforeY; diff < -tol || diff > tol {
		t.Errorf("ToScreen(anchor).Y after ZoomIn = %v, want ~%v (anchor should stay under the cursor)", afterY, beforeY)
	}
}

func TestBasicCamera_ZoomIn_CappedByMaxZoom(t *testing.T) {
	surface := geom.NewVec(1000, 1000)
	c := newBasicCamera(surface, testViewport(0, 0, 100, 100), 0)
	c.SetMaxZoom(2)

	c.ZoomIn(10, 50, 50)

	if c.Zoom() != 2 {
		t.Errorf("Zoom() = %v, want 2 (capped by SetMaxZoom)", c.Zoom())
	}
}

func TestBasicCamera_ZoomOut_CappedByConfiguredMinZoom(t *testing.T) {
	surface := geom.NewVec(10000, 10000)
	c := newBasicCamera(surface, testViewport(0, 0, 100, 100), 0)
	c.SetMinZoom(0.5)

	c.ZoomOut(100, 50, 50)

	if c.Zoom() != 0.5 {
		t.Errorf("Zoom() = %v, want 0.5 (capped by SetMinZoom, stricter than the world-fit floor)", c.Zoom())
	}
}

func TestNewFromSpaceWithConfig_AppliesViewportAndZoomLimits(t *testing.T) {
	c := NewFromSpaceWithConfig(1000, 1000, 0, contract.Config{
		ViewportWidth: 100, ViewportHeight: 100,
		MinZoom: 0.5,
		MaxZoom: 2,
	})

	if w := c.Bounds().BottomRight.X - c.Bounds().TopLeft.X; w != 100 {
		t.Errorf("Bounds() width = %v, want 100 (ViewportWidth applied)", w)
	}

	c.ZoomIn(10, 50, 50)
	if c.Zoom() != 2 {
		t.Errorf("Zoom() after ZoomIn(10,...) = %v, want 2 (MaxZoom applied)", c.Zoom())
	}

	c.ZoomOut(100, 50, 50)
	if c.Zoom() != 0.5 {
		t.Errorf("Zoom() after ZoomOut(100,...) = %v, want 0.5 (MinZoom applied)", c.Zoom())
	}
}

func TestBasicCamera_ImplementsCameraInterface(t *testing.T) {
	var _ contract.Camera = (*basicCamera)(nil)
}

func TestBasicCamera_FromScreen_InvertsToScreen(t *testing.T) {
	surface := geom.NewVec(1000, 1000)
	c := newBasicCamera(surface, testViewport(100, 100, 200, 200), 0)

	cases := []struct {
		name string
		zoom float32
		x, y float32
	}{
		{"no zoom, origin", 1, 100, 100},
		{"no zoom, interior point", 1, 150, 180},
		{"zoomed in", 2.5, 200, 220},
		{"zoomed out", 0.5, 120, 260},
	}

	for _, c2 := range cases {
		t.Run(c2.name, func(t *testing.T) {
			c.ZoomIn(c2.zoom/c.Zoom(), 200, 200)

			sx, sy := c.ToScreen(c2.x, c2.y)
			gotX, gotY := c.FromScreen(sx, sy)

			const tol = 0.01
			if diff := gotX - c2.x; diff < -tol || diff > tol {
				t.Errorf("FromScreen(ToScreen(%v,%v)).X = %v, want ~%v", c2.x, c2.y, gotX, c2.x)
			}
			if diff := gotY - c2.y; diff < -tol || diff > tol {
				t.Errorf("FromScreen(ToScreen(%v,%v)).Y = %v, want ~%v", c2.x, c2.y, gotY, c2.y)
			}
		})
	}
}

func TestBasicCamera_ToScreen_ToroidalWrapsWhenViewportFillsWorld(t *testing.T) {
	surface := geom.NewVec(1024, 1024)
	c := newBasicCamera(surface, testViewport(0, 0, 1024, 1024), aabbworld.Torus)
	c.Translate(8, 8)

	x, y := c.ToScreen(4, 4)

	const wantX, wantY = 1020, 1020
	if x != wantX || y != wantY {
		t.Errorf("ToScreen(4,4) = (%v,%v), want (%v,%v)", x, y, wantX, wantY)
	}
}

func TestBasicCamera_ToScreen_ToroidalWrapsNearCurrentView(t *testing.T) {
	surface := geom.NewVec(1000, 1000)
	c := newBasicCamera(surface, testViewport(0, 0, 200, 200), aabbworld.Torus)
	c.MoveTo(900, 900)

	nearX, nearY := c.ToScreen(950, 950)
	wrappedX, wrappedY := c.ToScreen(50, 50)

	if wrappedX <= nearX {
		t.Errorf("ToScreen(50,_).X = %v, want > ToScreen(950,_).X = %v (continues past the seam)", wrappedX, nearX)
	}
	if wrappedY <= nearY {
		t.Errorf("ToScreen(_,50).Y = %v, want > ToScreen(_,950).Y = %v (continues past the seam)", wrappedY, nearY)
	}
}

func TestBasicCamera_Visible_ToroidalWrap(t *testing.T) {
	surface := geom.NewVec(1000, 1000)
	c := newBasicCamera(surface, testViewport(0, 0, 200, 200), aabbworld.Torus)
	c.MoveTo(900, 900)

	if !c.Visible(testViewport(30, 30, 30, 30)) {
		t.Error("Visible((30,30)-(60,60)) = false, want true (within the wrapped-into-view portion)")
	}
}

func TestBasicCamera_Visible_PartialViewportDoesNotWrapAtWorldSize(t *testing.T) {
	surface := geom.NewVec(2000, 2000)
	c := newBasicCamera(surface, testViewport(0, 0, 768, 512), aabbworld.Torus)
	c.Translate(400, 0)

	if !c.Visible(testViewport(395, 10, 10, 10)) {
		t.Error("Visible((395,10)-(405,20)) = false, want true (merely straddles the view's near edge)")
	}
}

func TestToScreenQuads_PartialViewportClipsInsteadOfWrapping(t *testing.T) {
	surface := geom.NewVec(2000, 2000)
	c := newBasicCamera(surface, testViewport(0, 0, 768, 512), aabbworld.Torus)
	c.Translate(400, 0)

	quads := c.ToScreenQuads(395, 10, 405, 20, nil)

	if len(quads) != 1 {
		t.Fatalf("len(quads) = %d, want 1 (no wrap split — this isn't near the world's own edge)", len(quads))
	}
	if q := quads[0]; q.X0 != -5 || q.X1 != 5 {
		t.Errorf("quads[0] = %+v, want X0=-5, X1=5 (clipped off the left edge, not teleported to the far side)", q)
	}
}

func TestBasicCamera_FromScreen_ToroidalCanonicalRange(t *testing.T) {
	surface := geom.NewVec(1000, 1000)
	c := newBasicCamera(surface, testViewport(0, 0, 200, 200), aabbworld.Torus)
	c.MoveTo(900, 900)

	x, y := c.FromScreen(150, 150)
	if x < 0 || x >= 1000 || y < 0 || y >= 1000 {
		t.Errorf("FromScreen(150,150) = (%v,%v), want both in [0,1000)", x, y)
	}
}

func TestBasicCamera_Bounds_ToroidalReturnsLogicalExtent(t *testing.T) {
	surface := geom.NewVec(1024, 1024)
	c := newBasicCamera(surface, testViewport(0, 0, 1024, 1024), aabbworld.Torus)

	c.Translate(100, 0)

	if got := c.Bounds().BottomRight.X; got != 1124 {
		t.Errorf("Bounds().BottomRight.X = %v, want 1124 (100+1024, not clamped to 1024)", got)
	}
}

func TestBasicCamera_ZoomOut_ToroidalCappedToWorldFit(t *testing.T) {
	surface := geom.NewVec(1000, 1000)
	c := newBasicCamera(surface, testViewport(0, 0, 1000, 1000), aabbworld.Torus)

	c.ZoomOut(10, 500, 500)

	if c.Zoom() < 1 {
		t.Errorf("Zoom() = %v, want >= 1 (viewport already equals world size, one lap max)", c.Zoom())
	}
}

func TestBasicCamera_GobRoundTrip_RefusesToEncode(t *testing.T) {
	surface := geom.NewVec(1000, 1000)
	c := newBasicCamera(surface, testViewport(10, 10, 100, 100), 0)
	c.ZoomIn(2, 60, 60)

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(c); err == nil {
		t.Fatal("Encode of *basicCamera succeeded, want an error (no exported fields)")
	}
}

func TestCamera_PersistedRoundTrip_PreservesViewportAndZoom(t *testing.T) {
	c := NewFromSpace(1000, 1000, 0)
	c.MoveTo(50, 60)
	b := c.Bounds()
	cx, cy := float32(b.TopLeft.X+b.BottomRight.X)/2, float32(b.TopLeft.Y+b.BottomRight.Y)/2
	c.ZoomIn(2, cx, cy)
	wantBounds := c.Bounds()
	wantZoom := c.Zoom()

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	for _, target := range c.Persisted() {
		if err := enc.Encode(target); err != nil {
			t.Fatalf("Encode: %v", err)
		}
	}

	fresh := NewFromSpace(1000, 1000, 0)
	dec := gob.NewDecoder(&buf)
	for _, target := range fresh.Persisted() {
		if err := dec.Decode(target); err != nil {
			t.Fatalf("Decode: %v", err)
		}
	}
	fresh.Restore()

	if fresh.Bounds() != wantBounds {
		t.Errorf("Bounds() after Restore = %+v, want %+v", fresh.Bounds(), wantBounds)
	}
	if fresh.Zoom() != wantZoom {
		t.Errorf("Zoom() after Restore = %v, want %v", fresh.Zoom(), wantZoom)
	}
}

func TestNewFromSpace_DefaultViewportMatchesSize(t *testing.T) {
	c := NewFromSpace(800, 600, 0)

	bounds := c.Bounds()
	if bounds.TopLeft.X != 0 || bounds.TopLeft.Y != 0 {
		t.Errorf("Bounds().TopLeft = %+v, want (0,0)", bounds.TopLeft)
	}
	if w := bounds.BottomRight.X - bounds.TopLeft.X; w != 800 {
		t.Errorf("Bounds() width = %v, want 800", w)
	}
	if h := bounds.BottomRight.Y - bounds.TopLeft.Y; h != 600 {
		t.Errorf("Bounds() height = %v, want 600", h)
	}
}

func TestNewFromSpace_WithViewportOverride(t *testing.T) {
	override := geom.NewAABBAt(geom.NewVec(100, 100), 50, 30)
	c := NewFromSpace(800, 600, 0, override)

	if got := c.Bounds(); got != override {
		t.Errorf("Bounds() = %+v, want overridden viewport %+v (not the size-derived default)", got, override)
	}
}

func TestNewFromSpace_ToroidalWrapsOnTranslate(t *testing.T) {
	c := NewFromSpace(20, 20, aabbworld.Torus)

	c.MoveTo(2, 2)
	c.Translate(-10, 0)

	got := c.Bounds()
	if got.TopLeft.X != 12 {
		t.Errorf("after wrap-around Translate, Bounds().TopLeft.X = %v, want 12 (wrapped, not clamped)", got.TopLeft.X)
	}
}

func TestNewFromSpace_EuclideanClampsOnTranslate(t *testing.T) {
	c := NewFromSpace(20, 20, 0)

	c.MoveTo(2, 2)
	c.Translate(-10, 0)

	got := c.Bounds()
	if got.TopLeft.X != 0 {
		t.Errorf("Bounds().TopLeft.X = %v, want 0 (clamped euclidean, not wrapped)", got.TopLeft.X)
	}
}

func TestToScreenQuads_NoSplitWhenNotStraddlingReference(t *testing.T) {
	c := NewFromSpace(1024, 1024, aabbworld.Torus)
	c.Translate(1000, 0)

	quads := c.ToScreenQuads(1, 0, 11, 10, nil)

	if len(quads) != 1 {
		t.Fatalf("len(quads) = %d, want 1", len(quads))
	}
	q := quads[0]
	if got := q.X1 - q.X0; got != 10 {
		t.Errorf("quad width = %v, want 10", got)
	}
	if got := q.Y1 - q.Y0; got != 10 {
		t.Errorf("quad height = %v, want 10", got)
	}
}

func TestToScreenQuads_SplitsOnSingleAxis(t *testing.T) {
	c := NewFromSpace(1024, 1024, aabbworld.Torus)
	c.Translate(1000, 0)

	quads := c.ToScreenQuads(998, 0, 1010, 10, nil)

	if len(quads) != 2 {
		t.Fatalf("len(quads) = %d, want 2", len(quads))
	}
	totalWidth := float32(0)
	for _, q := range quads {
		totalWidth += q.X1 - q.X0
		if got := q.Y1 - q.Y0; got != 10 {
			t.Errorf("quad height = %v, want 10", got)
		}
	}
	if totalWidth != 12 {
		t.Errorf("total quad width straddling the wrap seam = %v, want 12", totalWidth)
	}
	first, second := quads[0], quads[1]
	if first.X1-first.X0 != 2 || first.T0X != 0 || first.T1X != float32(2)/12 {
		t.Errorf("first quad = %+v, want width 2 spanning T0X=0..%v", first, float32(2)/12)
	}
	if second.X1-second.X0 != 10 || second.X0 != 0 {
		t.Errorf("second quad = %+v, want width 10 starting at screen x=0", second)
	}
}

func TestToScreenQuads_SplitsOnBothAxes(t *testing.T) {
	c := NewFromSpace(1024, 1024, aabbworld.Torus)
	c.Translate(1000, 1000)

	quads := c.ToScreenQuads(998, 998, 1010, 1010, nil)

	if len(quads) != 4 {
		t.Fatalf("len(quads) = %d, want 4", len(quads))
	}
	var totalArea float32
	for _, q := range quads {
		totalArea += (q.X1 - q.X0) * (q.Y1 - q.Y0)
	}
	if totalArea != 12*12 {
		t.Errorf("total quad area straddling both wrap seams = %v, want %v", totalArea, 12*12)
	}
}

func TestFromScreenRect_ConsistentAcrossWrapSeam(t *testing.T) {
	c := NewFromSpace(1024, 1024, aabbworld.Torus)
	c.Translate(1000, 0)

	x0, y0, x1, y1 := contract.FromScreenRect(c, 0, 0, 20, 10)

	if got := x1 - x0; got != 20 {
		t.Errorf("contract.FromScreenRect width straddling the wrap seam = %v, want 20", got)
	}
	if y1-y0 != 10 {
		t.Errorf("contract.FromScreenRect height = %v, want 10", y1-y0)
	}
	if x0 != 1000 {
		t.Errorf("contract.FromScreenRect x0 = %v, want 1000", x0)
	}
}

func TestCamera_WrapsOnlyAlongAWrappingAxis(t *testing.T) {
	c := NewFromSpace(1000, 1000, aabbworld.WrapX, testViewport(0, 0, 200, 200))

	c.Translate(-50, -50)
	if got := c.Bounds().TopLeft; got.X != 950 || got.Y != 0 {
		t.Errorf("window at %v after moving up and left, want it wrapped to x=950 and held at y=0", got)
	}

	c.Translate(0, 5000)
	if got := c.Bounds().TopLeft; got.Y != 800 {
		t.Errorf("window at y=%v after moving far down, want it held whole at 800", got.Y)
	}

	if x, _ := c.ToScreen(20, 900); x != 70 {
		t.Errorf("x=20 lands at screen %v, want 70 — seen through the wrapping seam", x)
	}
	if !c.Visible(geom.NewAABBAt(geom.NewVec(10, 850), 20, 20)) {
		t.Error("a box just across the X seam is not visible")
	}
	if c.Visible(geom.NewAABBAt(geom.NewVec(960, 10), 20, 20)) {
		t.Error("a box at the top is visible from the bottom of a world that does not wrap in Y")
	}
	if got := len(c.ToScreenQuads(940, 850, 1010, 870, nil)); got != 1 {
		t.Errorf("a box inside the window drew as %d quads, want 1", got)
	}
}
