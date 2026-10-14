package render

import (
	"slices"
	"testing"
)

func TestSprites_DrawsTheLowerFirstAndAtOneHeightTheLyingFirst(t *testing.T) {
	var s Sprites
	add := func(mark, depth float32, under bool) {
		s.Layer(depth, under)
		s.piece(nil)
		s.instances = append(s.instances, mark, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
	}
	add(1, 40, false) // a gate's top
	add(2, 32, false) // a walker 32 tall
	add(3, 0, true)   // a trapdoor under him
	add(4, 32, true)  // a deck as high as his head
	drawn, runs := s.byLayer()
	var got []float32
	for k := 0; k < len(drawn); k += 12 {
		got = append(got, drawn[k])
	}
	if want := []float32{3, 4, 2, 1}; !slices.Equal(got, want) {
		t.Errorf("drawn %v, want %v", got, want)
	}
	if len(runs) != 1 || runs[0].count != 4 {
		t.Errorf("runs %+v, want one of the four", runs)
	}
	if n := testing.AllocsPerRun(100, func() { s.byLayer() }); n != 0 {
		t.Errorf("%v allocations a frame, want none", n)
	}
}
