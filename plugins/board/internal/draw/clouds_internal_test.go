package draw

import "testing"

func TestClouds_SetPutsItsOwnOverTheFramesWithoutAllocating(t *testing.T) {
	var c Clouds
	c.own, c.opts.Uniforms = map[string]any{}, map[string]any{}
	c.set("ViewSize", 1, 2)
	c.opts.Uniforms["ViewSize"] = []float32{9, 9}
	n := testing.AllocsPerRun(10, func() { c.set("ViewSize", 3, 4) })
	if got := c.opts.Uniforms["ViewSize"].([]float32); n != 0 || got[0] != 3 || got[1] != 4 {
		t.Errorf("ViewSize set over another: %v allocations, %v; want none and [3 4]", n, got)
	}
}
