package world

import (
	"slices"
	"testing"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
)

// a camera looking from (x, y)
type lookingFrom struct {
	camera.Camera
	x, y float32
}

func (l lookingFrom) Focus() (x, y, z float32) { return l.x, l.y, 0 }

// wholes over a row of n sectors 512 a side, wrapping in x where wraps, seen from the points
// given, its unfolding a stretch a sector from the west, the sector its whole too
func rowOfStretches(n int, wraps bool, from ...geom.Vec) (*wholes, []piece) {
	width := uint32(n * 512)
	var edges aabbworld.Edges
	if wraps {
		edges = aabbworld.WrapX
	}
	w := &module{
		config:  Config{Space: SpaceCfg{Width: width, Height: 512, Edges: edges}},
		sectors: &sectors{},
	}
	for _, at := range from {
		w.sectors.cams = append(w.sectors.cams,
			lookingFrom{Camera: cameratest.NewFromSpace(width, 512, edges), x: at.X, y: at.Y})
	}
	var all []piece
	for i := range n {
		x := float32(i * 512)
		all = append(all, piece{whole: 1, sector: i,
			box: geom.NewAABB(geom.NewVec(x, 0), geom.NewVec(x+512, 512))})
	}
	return &wholes{w: w}, all
}

func sectorsUnfolding(s *wholes) []int {
	var got []int
	for _, p := range s.unfolding {
		got = append(got, p.sector)
	}
	return got
}

func TestWholes_Nearest_KeepsTheBudgetNearestAnEyeInTheOrderFound(t *testing.T) {
	for name, c := range map[string]struct {
		wraps  bool
		from   []geom.Vec
		budget int
		want   []int
	}{
		"the east end": {
			from:   []geom.Vec{{X: 20480, Y: 256}},
			budget: 4,
			want:   []int{36, 37, 38, 39},
		},
		"either end": {
			from:   []geom.Vec{{X: 0, Y: 0}, {X: 20480, Y: 512}},
			budget: 4,
			want:   []int{0, 1, 38, 39},
		},
		"across the seam": {
			wraps:  true,
			from:   []geom.Vec{{X: 100, Y: 256}},
			budget: 4,
			want:   []int{0, 1, 38, 39},
		},
		// 0 and 3 as far
		"ties to the first found": {
			from:   []geom.Vec{{X: 1024, Y: 256}},
			budget: 3,
			want:   []int{0, 1, 2},
		},
		"none, the first found": {budget: 4, want: []int{0, 1, 2, 3}},
	} {
		t.Run(name, func(t *testing.T) {
			s, all := rowOfStretches(40, c.wraps, c.from...)
			s.budget = c.budget
			s.unfolding = append(s.unfolding, all...)
			s.nearest()
			if got := sectorsUnfolding(s); !slices.Equal(got, c.want) {
				t.Errorf("kept sectors %v, want %v", got, c.want)
			}
		})
	}
}

func TestWholes_Nearest_AllocatesNothingOnceWarm(t *testing.T) {
	s, all := rowOfStretches(240, false, geom.NewVec(20480, 256), geom.NewVec(0, 0))
	s.budget = 16
	keep := func() {
		s.unfolding = append(s.unfolding[:0], all...)
		s.nearest()
	}
	keep()
	if n := testing.AllocsPerRun(100, keep); n != 0 {
		t.Errorf("%v allocations a step, want none", n)
	}
}
