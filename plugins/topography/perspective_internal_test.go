package topography

import (
	"math"
	"testing"
)

// testPerspective looks from 200 off at a pitch of 30°, the 2:1 view's heading, through a 400 px
// focal length over 32-unit cells, at the point (320, 320, 0).
func testPerspective(heading float32) perspective {
	target := [3]float32{320, 320, 0}
	s, c := math.Sincos(float64(heading) + math.Pi/4)
	cp, sp := math.Cos(math.Pi/6), math.Sin(math.Pi/6)
	eye := [3]float32{320 + 200*float32(cp*s), 320 + 200*float32(cp*c), 200 * float32(sp)}
	return newPerspective(eye, target, heading, 400, 8, 4000, 32)
}

func TestPerspective_TheTargetLandsInTheMiddleAndXRunsDownRight(t *testing.T) {
	p := testPerspective(0)
	if sx, sy := p.Project(320, 320, 0); !near(sx, 0) || !near(sy, 0) {
		t.Errorf("the target projects to (%v, %v), want the middle", sx, sy)
	}
	if sx, sy := p.Project(352, 320, 0); sx <= 0 || sy <= 0 {
		t.Errorf("a cell along x projects to (%v, %v), want down-right as the isometric view has it", sx, sy)
	}
	if sx, sy := p.Project(320, 352, 0); sx >= 0 || sy <= 0 {
		t.Errorf("a cell along y projects to (%v, %v), want down-left", sx, sy)
	}
	if _, sy := p.Project(320, 320, 10); sy >= 0 {
		t.Errorf("a point 10 up projects to y %v, want above the middle", sy)
	}
	if at, off := width(p, 320, 320), width(p, 192, 192); at <= off {
		t.Errorf("a cell at the target is %v wide, one four cells further off %v: want the further one smaller", at, off)
	}
}

// width is how wide a cell at (x, y) is drawn across the screen.
func width(p perspective, x, y float32) float32 {
	ax, _ := p.Project(x-16, y+16, 0)
	bx, _ := p.Project(x+16, y-16, 0)
	return bx - ax
}

func TestPerspective_UnprojectInvertsProjectOnEveryPlane(t *testing.T) {
	for _, heading := range []float32{0, 0.7, math.Pi / 2, 4.5} {
		p := testPerspective(heading)
		for _, q := range [][3]float32{{320, 320, 0}, {400, 300, 0}, {250, 350, 7}, {330, 310, 30}} {
			sx, sy := p.Project(q[0], q[1], q[2])
			x, y, hit := p.cast(sx, sy, q[2])
			if !hit || !near(x, q[0]) || !near(y, q[1]) {
				t.Errorf("heading %v: point %v went to (%v, %v) and came back as (%v, %v), hit %v", heading, q, sx, sy, x, y, hit)
			}
		}
	}
}

func TestPerspective_DepthGrowsTowardsTheEyeAndTiesInACell(t *testing.T) {
	p := testPerspective(0)
	if back, front := p.Depth(320, 320, 0), p.Depth(352, 352, 0); back >= front {
		t.Errorf("the target's depth %v is not behind the cell towards the eye %v", back, front)
	}
	if p.Depth(330, 330, 0) != p.Depth(345, 335, 20) {
		t.Error("two points of one cell differ in depth")
	}
	if p.Depth(352, 320, 0) != p.Depth(320, 352, 0) {
		t.Error("two cells the same way off along x and y differ in depth")
	}
}

func TestPerspective_WhatLiesBehindTheEyeOrOverTheHorizonStaysFinite(t *testing.T) {
	p := testPerspective(0)
	// well behind the eye along the way it looks from
	behind := add(p.eye, scale(p.forward, -100))
	sx, sy := p.Project(behind[0], behind[1], behind[2])
	if math.IsNaN(float64(sx)) || math.IsInf(float64(sx), 0) || math.IsNaN(float64(sy)) || math.IsInf(float64(sy), 0) {
		t.Errorf("a point behind the eye projects to (%v, %v)", sx, sy)
	}
	x, y, hit := p.cast(0, -400, 0) // 45° up from the middle: over the horizon at a pitch of 30°
	if hit {
		t.Errorf("a screen point over the horizon hit the ground at (%v, %v)", x, y)
	}
	if d := math.Hypot(float64(x-320), float64(y-320)); d < 1000 || math.IsInf(d, 0) || math.IsNaN(d) {
		t.Errorf("over the horizon the ground point is %v off, want far and finite", d)
	}
}

func TestPerspective_TowardLeadsFromTheTargetToTheEye(t *testing.T) {
	for _, heading := range []float32{0, 2} {
		p := testPerspective(heading)
		d := p.Toward()
		if !near(d[0]*d[0]+d[1]*d[1]+d[2]*d[2], 1) || d[2] <= 0 {
			t.Fatalf("heading %v: Toward %v, want a unit vector rising towards the eye", heading, d)
		}
		if sx, sy := p.Project(320+40*d[0], 320+40*d[1], 40*d[2]); !near(sx, 0) || !near(sy, 0) {
			t.Errorf("heading %v: 40 along Toward from the target lands at (%v, %v), not in the middle", heading, sx, sy)
		}
		if p.Depth(320+64*d[0], 320+64*d[1], 0) <= p.Depth(320, 320, 0) {
			t.Errorf("heading %v: the way towards the eye is not nearer the eye", heading)
		}
	}
}

func TestPerspective_VanishIsWhereEverythingFarAlongADirectionIsDrawn(t *testing.T) {
	p := testPerspective(0.4)
	if sx, sy, ok := p.Vanish(p.forward[0], p.forward[1], p.forward[2]); !ok || !near(sx, 0) || !near(sy, 0) {
		t.Errorf("the way the eye looks vanishes at (%v, %v) %v, want the middle", sx, sy, ok)
	}
	if _, _, ok := p.Vanish(-p.forward[0], -p.forward[1], -p.forward[2]); ok {
		t.Error("the way behind the eye vanishes on the screen")
	}
	dir := norm32([3]float32{p.forward[0] + 0.2*p.right[0] + 0.1*p.up[0], p.forward[1] + 0.2*p.right[1] + 0.1*p.up[1], p.forward[2] + 0.2*p.right[2] + 0.1*p.up[2]})
	vx, vy, ok := p.Vanish(dir[0], dir[1], dir[2])
	if !ok || vx <= 0 || vy >= 0 {
		t.Fatalf("a way right of and above the middle vanishes at (%v, %v) %v, want right of and above it", vx, vy, ok)
	}
	far := add(p.eye, scale(dir, 1e5))
	if sx, sy := p.Project(far[0], far[1], far[2]); math.Abs(float64(sx-vx)) > 0.5 || math.Abs(float64(sy-vy)) > 0.5 {
		t.Errorf("a point far along the way is drawn at (%v, %v), want where it vanishes (%v, %v)", sx, sy, vx, vy)
	}
}

func norm32(v [3]float32) [3]float32 { return scale(v, 1/norm(v)) }
