package response_test

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/collision/internal/response"
)

func TestSweep_WhereThePathFirstTouchesTheBox(t *testing.T) {
	box := geom.NewAABBAt(geom.NewVec(100, 100), 10, 10)
	half := geom.NewVec(2, 2)
	cases := map[string]struct {
		from, to geom.Vec
		along    float32
		normal   geom.Vec
		ok       bool
	}{
		"straight through":    {geom.NewVec(50, 105), geom.NewVec(150, 105), 0.48, geom.NewVec(-1, 0), true},
		"from the right":      {geom.NewVec(150, 105), geom.NewVec(50, 105), 0.38, geom.NewVec(1, 0), true},
		"from above":          {geom.NewVec(105, 50), geom.NewVec(105, 150), 0.48, geom.NewVec(0, -1), true},
		"slantwise":           {geom.NewVec(90, 95), geom.NewVec(120, 125), 0.2667, geom.NewVec(-1, 0), true},
		"beside the box":      {geom.NewVec(50, 120), geom.NewVec(150, 120), 0, geom.Vec{}, false},
		"grazing an edge":     {geom.NewVec(50, 112), geom.NewVec(150, 112), 0, geom.Vec{}, false},
		"too short":           {geom.NewVec(50, 105), geom.NewVec(90, 105), 0, geom.Vec{}, false},
		"ending at the face":  {geom.NewVec(50, 105), geom.NewVec(98, 105), 0, geom.Vec{}, false},
		"behind the path":     {geom.NewVec(120, 105), geom.NewVec(150, 105), 0, geom.Vec{}, false},
		"starting inside":     {geom.NewVec(101, 105), geom.NewVec(150, 105), 0, geom.NewVec(-1, 0), true},
		"no step, inside":     {geom.NewVec(105, 105), geom.NewVec(105, 105), 0, geom.NewVec(-1, 0), true},
		"no step, outside":    {geom.NewVec(50, 105), geom.NewVec(50, 105), 0, geom.Vec{}, false},
		"along the near face": {geom.NewVec(97, 50), geom.NewVec(97, 150), 0, geom.Vec{}, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			along, normal, ok := response.Sweep(tc.from, tc.to, half, box)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if !ok {
				return
			}
			if f32.Abs(along-tc.along) > 1e-3 {
				t.Errorf("along = %v, want %v", along, tc.along)
			}
			if normal != tc.normal {
				t.Errorf("normal = %v, want %v", normal, tc.normal)
			}
		})
	}
}

func TestSweepWall_WhereThePathFirstTouchesTheWall(t *testing.T) {
	box := geom.NewAABBAt(geom.NewVec(100, 100), 10, 10)
	half := geom.NewVec(2, 2)
	a, b := geom.NewVec(100, 105), geom.NewVec(110, 105)
	for _, path := range [][2]geom.Vec{
		{geom.NewVec(50, 105), geom.NewVec(150, 105)},
		{geom.NewVec(150, 105), geom.NewVec(50, 105)},
		{geom.NewVec(105, 50), geom.NewVec(105, 150)},
		{geom.NewVec(90, 95), geom.NewVec(120, 125)},
		{geom.NewVec(50, 120), geom.NewVec(150, 120)},
		{geom.NewVec(50, 105), geom.NewVec(90, 105)},
		{geom.NewVec(97, 50), geom.NewVec(97, 150)},
	} {
		wantAlong, wantNormal, wantOK := response.Sweep(path[0], path[1], half, box)
		along, normal, ok := response.SweepWall(path[0], path[1], half, a, b, 5)
		if ok != wantOK || f32.Abs(along-wantAlong) > f32.Eps(1) || normal != wantNormal {
			t.Errorf("%v: %v %v %v, want %v %v %v as against the box", path, along, normal, ok,
				wantAlong, wantNormal, wantOK)
		}
	}
	along, normal, ok := response.SweepWall(geom.NewVec(0, 100), geom.NewVec(200, 100), half,
		geom.NewVec(50, 50), geom.NewVec(150, 150), 1)
	r := float32(math.Sqrt2 / 2)
	if !ok || f32.Abs(normal.X+r) > f32.Eps(1) || f32.Abs(normal.Y-r) > f32.Eps(1) {
		t.Errorf("east into a slanted wall: %v %v %v, want the way out back square to it", along,
			normal, ok)
	}
	// the box's corner meets the wall's face 2·2·r + 1 off its line, the line at x 100
	if want := (100 - (4*r+1)/r) / 200; f32.Abs(along-want) > f32.Eps(1) {
		t.Errorf("east into a slanted wall it touches at %v, want %v", along, want)
	}
}
