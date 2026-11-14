package buildings

import (
	"image/color"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
)

// the ground rising a unit every four eastwards
func slope(p geom.Vec) float32 { return p.X / 4 }

func TestLay_StandsABuildingOnItsLowestGroundItsStoreysOverItsHighest(t *testing.T) {
	spec := Spec{
		Storey: 10,
		Bay:    8,
		Walls:  color.RGBA{R: 255, A: 255},
		Tiles:  color.RGBA{B: 255, A: 255},
		Parts: []Part{
			{Roof: Hip, Pitch: 0.5},
			{Roof: Pyramid, Pitch: 1, Size: geom.NewVec(4, 4), Off: geom.NewVec(-0.5, 0), Above: 6,
				Blind: true, Fine: true},
		},
	}
	b := spec.of(Row{Mid: [2]float32{100, 50}, Size: [2]float32{40, 20}, Floors: 3})
	whole := lay(nil, spec, b, slope, 0.5, true)
	if len(whole) != 2*solidFloats {
		t.Fatalf("%d floats of a body and a turret, want %d", len(whole), 2*solidFloats)
	}
	body := whole[:solidFloats]
	// its corners lie at x 80 and 120: the ground 20 and 30 there
	want := []float32{
		100, 50, 20, 60,
		20, 10, 1, 0,
		5, 10, 10, 8,
		1, 0, 0, 0.5,
		0, 0, 1, 30,
	}
	for i, v := range want {
		if f32.Abs(body[i]-v) > 1e-4 {
			t.Errorf("the body's float %d is %v, want %v", i, body[i], v)
		}
	}
	turret := whole[solidFloats:]
	if turret[0] != 90 || turret[3] != 66 || turret[4] != 2 || turret[8] != 2 ||
		turret[9] != 0 || turret[11] != 0 {
		t.Errorf("the turret stands %v, want it a quarter of the length west, 6 over the "+
			"walls, a pyramid 2 high and blind", turret)
	}
	coarse := lay(nil, spec, b, slope, 0.5, false)
	if len(coarse) != solidFloats || coarse[11] != 0 {
		t.Errorf("far off %d floats, a bay of %v; want the body alone, without windows",
			len(coarse), coarse[11])
	}
}

func TestSpec_ABuildingsBoxHoldsItTurned(t *testing.T) {
	spec := Spec{Storey: 3, Floors: 2, Parts: []Part{{Roof: Gable, Pitch: 1}}}
	b := spec.of(Row{Size: [2]float32{40, 20}, Turn: 90})
	if hx, hy := b.reach(); f32.Abs(hx-10) > 1e-3 || f32.Abs(hy-20) > 1e-3 {
		t.Errorf("turned a quarter its box reaches %v by %v, want 10 by 20", hx, hy)
	}
	if b.walls != 6 {
		t.Errorf("its walls stand %v, want the kind's two storeys of 3", b.walls)
	}
	if h := b.shade(spec); h != 11 {
		t.Errorf("its shadow is cast from %v, want its walls and half its roof of 10", h)
	}
}
