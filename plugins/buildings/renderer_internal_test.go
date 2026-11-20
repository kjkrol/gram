package buildings

import (
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
	"image/color"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
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
	whole := lay(nil, spec, b, slope, 0.5, true, nil)
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
	coarse := lay(nil, spec, b, slope, 0.5, false, nil)
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

type ground bool

func (g ground) Hazes() bool { return bool(g) }

func TestSolids_AreHazedOnlyWhereTheGroundIs(t *testing.T) {
	r := &solids{p: &Plugin{}}
	for _, hazed := range []bool{false, true, false} {
		r.p.WithAir(nil, ground(hazed))
		r.Publish()
		r.Swap()
		if r.hazed != hazed {
			t.Errorf("the ground hazed %v, its buildings %v", hazed, r.hazed)
		}
	}
	r.p.WithAir(nil, nil)
	r.Publish()
	r.Swap()
	if r.hazed {
		t.Error("buildings on no ground are hazed")
	}
}

func TestShaders_Compile(t *testing.T) {
	gputest.Need(t)
	for name, s := range map[string]*render.Shader{"solids": solidShader, "rooms": innerShader,
		"rooms bared": baredShader} {
		if err := s.Compile(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestSolidsOf_StandsAPartWhollyOverItsBodyOnItsRoof(t *testing.T) {
	// a hall 64 by 32 on ground at 5, its walls 16 high: a room on its roof, half its length off
	// its middle, and a tower at its end reaching past it, each 10 over its eaves
	spec := Spec{Storey: 16, Floors: 1, Parts: []Part{
		{Roof: Flat},
		{Roof: Flat, Size: geom.NewVec(16, 12), Off: geom.NewVec(0.5, 0), Above: 10},
		{Roof: Flat, Size: geom.NewVec(16, 12), Off: geom.NewVec(1, 0), Above: 10},
	}}
	b := spec.of(Row{Mid: [2]float32{100, 100}, Size: [2]float32{64, 32}})
	got := solidsOf(spec, b, func(geom.Vec) float32 { return 5 }, nil)
	if got[1].foot != 21 || got[1].eaves != 31 {
		t.Errorf("the room on the roof stands from %v to %v, want from the hall's eaves at 21 "+
			"to 31", got[1].foot, got[1].eaves)
	}
	if got[2].foot != 5 || got[2].eaves != 31 {
		t.Errorf("the tower reaching past the hall stands from %v to %v, want from the ground "+
			"at 5 to 31", got[2].foot, got[2].eaves)
	}
}

func TestOpenings_LeaveOutTheWindowsTooNearTheDoor(t *testing.T) {
	// the door of the house of houseFrame stands where two bays of its north wall meet, a pane
	// either side of it 0.24 off
	spec := Spec{Storey: 12, Floors: 1, Bay: 8, Door: cell.Gap{Width: 4, Height: 8},
		Parts: []Part{{Roof: Hip, Pitch: 0.5}}}
	at := spec.of(Row{Mid: [2]float32{128, 112}, Size: [2]float32{64, 32}})
	h := house{spec: spec, at: at, floor: 10, doorAt: geom.NewVec(112, 128)}
	room := openings(spec, at, h, func(geom.Vec) float32 { return 10 })
	opens := func(x float32) bool {
		for _, o := range room.Openings[:room.Open] {
			if o.Bottom > 0 && o.A.Y == 128 && min(o.A.X, o.B.X) < x && x < max(o.A.X, o.B.X) {
				return true
			}
		}
		return false
	}
	for x, want := range map[float32]bool{100: true, 108: false, 116: false, 124: true} {
		if got := opens(x); got != want {
			t.Errorf("a window of the door's wall at %v opens: %v, want %v", x, got, want)
		}
	}
}

func TestSpec_WashesItsBuildingsInItsColoursByTheirShares(t *testing.T) {
	white := color.RGBA{R: 250, G: 250, B: 250, A: 255}
	pink := color.RGBA{R: 240, G: 180, B: 170, A: 255}
	spec := Spec{Walls: color.RGBA{R: 9, A: 255}, Washes: []Wash{{white, 3}, {pink, 1}}}
	seen := map[color.RGBA]int{}
	for x := range 40 {
		for y := range 40 {
			b := standing{mid: geom.NewVec(float32(x)*32+16, float32(y)*32+16)}
			seen[spec.washOf(b)]++
		}
	}
	if len(seen) != 2 || seen[white] < 1100 || seen[white] > 1300 {
		t.Errorf("1600 buildings washed white 3 to pink 1 are %v", seen)
	}
	if got := (Spec{Walls: white}).washOf(standing{}); got != white &&
		got != tinted(white, 1, 0.95, 0.84) && got != tinted(white, 1, 0.9, 0.88) {
		t.Errorf("a kind without washes washes a building %v, not its walls' colour", got)
	}
}

func TestNear_AKindOfStoneHasItsPlinthDressedInIt(t *testing.T) {
	gputest.Need(t)
	// a block on ground rising 4 to its south, its storeys counted from its highest corner: its
	// north wall stands on 4 of plinth
	stone := color.RGBA{R: 60, G: 58, B: 55, A: 255}
	spec := Spec{Storey: 12, Floors: 1, Walls: color.RGBA{R: 230, G: 230, B: 230, A: 255},
		Tiles: blueTiles, Parts: []Part{{Roof: Flat}}}
	at := spec.of(Row{Mid: [2]float32{128, 112}, Size: [2]float32{64, 32}})
	level := func(p geom.Vec) float32 { return 10 + (p.Y-96)/8 }
	look := func(spec Spec, z float32) uint8 {
		cam := slant(cameratest.NewFromSpace(64, 64, 0), [3]float32{128, 96, z}, 0, 1, -0.3)
		_, pix := drawn(t, cam, spec, at, level, nil, false, Settings{})
		return pix[4*(32*64+32)]
	}
	plain, dressed := spec, spec
	dressed.Stone = stone
	if got := look(dressed, 12); got > 120 {
		t.Errorf("dressed in stone, its plinth is %v bright, want the stone's", got)
	}
	if got := look(plain, 12); got < 150 {
		t.Errorf("without stone, its plinth is %v bright, want its walls'", got)
	}
	if got := look(dressed, 18); got < 150 {
		t.Errorf("dressed in stone, its wall over the plinth is %v bright, want plaster", got)
	}
}

func TestNear_AKindsStoneFramesItsWindowsWhereTheyStand(t *testing.T) {
	gputest.Need(t)
	// a storey of 18 and bays of 12 along a south wall 48 long: panes round x 122 and 134, 2.64
	// either side; between them the pier, at the pane's edge its frame
	spec := Spec{Storey: 18, Floors: 1, Walls: color.RGBA{R: 230, G: 230, B: 230, A: 255},
		Tiles: blueTiles, Bay: 12, Stone: color.RGBA{R: 60, G: 58, B: 55, A: 255},
		Parts: []Part{{Roof: Flat}}}
	at := spec.of(Row{Mid: [2]float32{128, 112}, Size: [2]float32{48, 24}})
	look := func(x float32) uint8 {
		cam := slant(cameratest.NewFromSpace(64, 64, 0), [3]float32{x, 124, 20}, 0, -1, -0.3)
		_, pix := drawn(t, cam, spec, at, func(geom.Vec) float32 { return 10 }, nil, false,
			Settings{})
		return pix[4*(32*64+32)]
	}
	if got := look(128); got < 150 {
		t.Errorf("the pier between two windows is %v bright, want plaster", got)
	}
	if got := look(137.2); got > 120 {
		t.Errorf("by a window's edge the wall is %v bright, want its stone frame", got)
	}
}
