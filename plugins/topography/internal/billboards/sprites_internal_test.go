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
