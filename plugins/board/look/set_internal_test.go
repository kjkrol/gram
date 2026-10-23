package look

import "testing"

func TestSet_PutsItsOwnOverTheFramesWithoutAllocating(t *testing.T) {
	var g gridLines
	g.own, g.opts.Uniforms = map[string]any{}, map[string]any{}
	g.set("ViewAt", 1, 2)
	g.opts.Uniforms["ViewAt"] = []float32{9, 9}
	n := testing.AllocsPerRun(10, func() { g.set("ViewAt", 3, 4) })
	if got := g.opts.Uniforms["ViewAt"].([]float32); n != 0 || got[0] != 3 || got[1] != 4 {
		t.Errorf("the grid's ViewAt set over another: %v allocations, %v; want none and [3 4]",
			n, got)
	}
	var s groundShade
	s.own, s.opts.Uniforms = map[string]any{}, map[string]any{}
	s.set("ViewAt", 1, 2)
	s.opts.Uniforms["ViewAt"] = []float32{9, 9}
	n = testing.AllocsPerRun(10, func() { s.set("ViewAt", 3, 4) })
	if got := s.opts.Uniforms["ViewAt"].([]float32); n != 0 || got[0] != 3 || got[1] != 4 {
		t.Errorf("the shade's ViewAt set over another: %v allocations, %v; want none and [3 4]",
			n, got)
	}
}
