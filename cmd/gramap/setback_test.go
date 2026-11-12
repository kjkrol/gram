package main

import (
	"math"
	"testing"
)

func TestDrawBack_ABuildingsWallDrawsBackOffALane(t *testing.T) {
	// a road along x, 18 either side of its axis, and a house 10 by 10 whose wall stands 15
	// off it
	road := lane{a: [2]float64{-50, 0}, b: [2]float64{50, 0}, half: 18}
	house := built{mid: [2]float64{0, 20}, size: [2]float64{10, 10}}
	back, off, ok := house.drawBack(road, 4)
	if !ok || back != 3 || off != [2]float64{0, 1} {
		t.Fatalf("drawn back %v the way %v, %v; want 3 off the road", back, off, ok)
	}
	if house.size != [2]float64{10, 7} || house.mid != [2]float64{0, 21.5} {
		t.Errorf("the house %v at %v, want 7 deep with its far wall where it stood", house.size,
			house.mid)
	}
	// turned an eighth, its corner gives way along the side lying more across the road
	turned := built{mid: [2]float64{0, 22}, size: [2]float64{10, 10}, turn: 45}
	if _, _, ok := turned.drawBack(road, 4); !ok {
		t.Fatal("a house turned would not draw back")
	}
	sin, cos := math.Sincos(math.Pi / 4)
	near := math.Inf(1)
	for _, sx := range [2]float64{-1, 1} {
		for _, sy := range [2]float64{-1, 1} {
			near = min(near, turned.mid[1]+sx*sin*turned.size[0]/2+sy*cos*turned.size[1]/2)
		}
	}
	if math.Abs(near-18) > 1e-9 {
		t.Errorf("its nearest corner stands %v off the axis, want at the lane's edge, 18", near)
	}
	// too narrow to be cut, and a road through its middle: not drawn back, as it stood
	small := built{mid: [2]float64{0, 19.25}, size: [2]float64{10, 4.5}}
	if back, _, ok := small.drawBack(road, 4); ok || back != 1 || small.size[1] != 4.5 {
		t.Errorf("a house 4.5 deep a unit in the road: back %v, %v, size %v; want it whole and a "+
			"step of 1 asked", back, ok, small.size)
	}
	across := built{mid: [2]float64{0, 1}, size: [2]float64{10, 10}}
	if back, _, ok := across.drawBack(road, 4); ok || back < 18 {
		t.Errorf("a house on the axis: back %v, %v; want no drawing back", back, ok)
	}
	// beside the road's end, and clear of it: untouched
	for name, b := range map[string]built{
		"past the end": {mid: [2]float64{70, 5}, size: [2]float64{10, 10}},
		"clear":        {mid: [2]float64{0, 30}, size: [2]float64{10, 10}},
	} {
		was := b
		if _, _, ok := b.drawBack(road, 4); !ok || b != was {
			t.Errorf("a house %s is %v, was %v", name, b, was)
		}
	}
}
