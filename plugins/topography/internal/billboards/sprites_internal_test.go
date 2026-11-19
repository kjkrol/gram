package billboards

import "testing"

func TestSprites_SetPutsItsOwnOverTheFramesWithoutAllocating(t *testing.T) {
	s := sprites{own: map[string]any{}}
	s.opts.Uniforms = map[string]any{}
	s.set("Look", 1, 2, 3)
	s.opts.Uniforms["Look"] = []float32{9, 9, 9}
	n := testing.AllocsPerRun(10, func() { s.set("Look", 4, 5, 6) })
	if got := s.opts.Uniforms["Look"].([]float32); n != 0 || got[0] != 4 || got[2] != 6 {
		t.Errorf("Look set over another: %v allocations, %v; want none and [4 5 6]", n, got)
	}
}

func TestSprites_BodiesAreLaidFromTheNearestAlongTheWayTheCameraLooks(t *testing.T) {
	var s sprites
	const n = 60
	for i := range n {
		x, y, z := float32(i*37%100), float32(i*61%400), float32(i*13%50)
		s.bodies = append(s.bodies, x, y, z, 10, 6, float32(i), 0, 2, 1, 1, 1, 0, 1, 0, 0, 0)
	}
	way := [3]float32{0.5, 0.7, -0.5}
	s.nearestFirst(way)
	if len(s.bodies) != n*bodyFloats {
		t.Fatalf("%d floats of bodies after they were laid, want the %d", len(s.bodies),
			n*bodyFloats)
	}
	deep := func(b []float32) float32 { return b[0]*way[0] + b[1]*way[1] + (b[2]+5)*way[2] }
	low, high := deep(s.bodies), deep(s.bodies)
	met := map[float32]int{}
	for i := 0; i < len(s.bodies); i += bodyFloats {
		low, high = min(low, deep(s.bodies[i:])), max(high, deep(s.bodies[i:]))
		met[s.bodies[i+5]]++
	}
	for i := range n {
		if met[float32(i)] != 1 {
			t.Fatalf("the body %d is laid %d times, want once", i, met[float32(i)])
		}
	}
	slice := (high - low) / (bodySlices - 1)
	for i := bodyFloats; i < len(s.bodies); i += bodyFloats {
		before, after := deep(s.bodies[i-bodyFloats:]), deep(s.bodies[i:])
		if after < before-slice {
			t.Fatalf("a body %v deep is laid after one %v deep, over a slice of %v nearer",
				after, before, slice)
		}
	}
	first := s.bodies[5]
	s.nearestFirst([3]float32{-0.5, -0.7, 0.5})
	if s.bodies[5] != first {
		t.Error("the bodies were laid anew in a frame they were laid in already")
	}
}
