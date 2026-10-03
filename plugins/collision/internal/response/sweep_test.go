package response_test

import (
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/collision/internal/response"
)

// A path of a 4-wide box across a 10-wide box: it touches where its leading edge meets the box's
// near face, entering by that face; one beside the box, one too short and one only grazing an
// edge touch nothing; one starting inside touches at once, the shortest way out.
func TestSweep_WhereThePathFirstTouchesTheBox(t *testing.T) {
	box := geom.NewAABBAt(geom.NewVec(100, 100), 10, 10)
	half := geom.NewVec(2, 2)
	cases := map[string]struct {
		from, to geom.Vec
		along    float64
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
			if math.Abs(along-tc.along) > 1e-3 {
				t.Errorf("along = %v, want %v", along, tc.along)
			}
			if normal != tc.normal {
				t.Errorf("normal = %v, want %v", normal, tc.normal)
			}
		})
	}
}
