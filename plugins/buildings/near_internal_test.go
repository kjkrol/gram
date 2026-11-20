package buildings

import (
	"image/color"
	"math"
	"slices"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gputest"
)

// straight down from 200 up, a screen pixel 4 world units
type above struct{ camera.Camera }

func (above) Rays() (camera.RayField, bool) {
	return camera.RayField{Origin: [3]float32{0, 0, 200}, DX: [3]float32{4, 0, 0},
		DY: [3]float32{0, 4, 0}, Dir: [3]float32{0, 0, -1}}, true
}

// riding a unit, behind it
type behind struct{ above }

func (behind) Fasten(camera.Fastening) {}

func (behind) Fastening() camera.Fastening {
	return camera.Fastening{How: camera.Behind, Entity: 1 << 40}
}

// riding a unit, inside it
type inUnit struct{ above }

func (inUnit) Fasten(camera.Fastening) {}

func (inUnit) Fastening() camera.Fastening {
	return camera.Fastening{How: camera.Inside, Entity: 1 << 40}
}

// a ground of nothing but its depth, for the buildings to stand in
type depthOnly struct{}

func (depthOnly) Compose(f *render.Frame, _ camera.Camera)           { f.DrawOpaque() }
func (depthOnly) Draw(render.Target, camera.Camera, render.Uniforms) {}

var blueTiles = color.RGBA{R: 40, G: 60, B: 200, A: 255}

// a house of two cells, (3, 3) and (4, 3), its floor 10 up on level ground, its roof blue; the
// room bared or not, under settings, seen through cam: the pixel over its middle
func housePixel(t *testing.T, cam camera.Camera, bared bool, settings Settings) color.RGBA {
	pix := houseFrame(t, cam, bared, settings)
	i := 4 * (28*64 + 32) // (128, 112)
	return color.RGBA{R: pix[i], G: pix[i+1], B: pix[i+2], A: pix[i+3]}
}

// houseFrame is the house of housePixel seen through cam, a screen 64 by 64
func houseFrame(t *testing.T, cam camera.Camera, bared bool, settings Settings) []byte {
	t.Helper()
	_, pix := houseDrawn(t, cam, bared, settings)
	return pix
}

// houseDrawn is houseFrame with the plugin that drew it
func houseDrawn(t *testing.T, cam camera.Camera, bared bool, settings Settings) (*Plugin, []byte) {
	t.Helper()
	spec := Spec{Storey: 12, Floors: 1, Walls: color.RGBA{R: 200, G: 200, B: 200, A: 255},
		Tiles: blueTiles, Bay: 8, Door: cell.Gap{Width: 4, Height: 8},
		Parts: []Part{{Roof: Hip, Pitch: 0.5}}}
	at := spec.of(Row{Mid: [2]float32{128, 112}, Size: [2]float32{64, 32}})
	h := house{spec: spec, at: at, floor: 10, doorAt: geom.NewVec(112, 128)}
	return drawn(t, cam, spec, at, func(geom.Vec) float32 { return 10 }, &h, bared, settings)
}

// drawn is the building of spec standing at on the ground level gives, a house with the room h
// or without one, seen through cam on a screen 64 by 64, the room bared or not, under settings
func drawn(t *testing.T, cam camera.Camera, spec Spec, at standing, level func(geom.Vec) float32,
	h *house, bared bool, settings Settings) (*Plugin, []byte) {
	t.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 256, Height: 256},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 1, MaxSize: 64},
		Heights:  true,
	})
	brd := board.NewPlugin(grid.DefaultGrids{}.Square(8, 8, 32), &cell.MultipleOccupancy{}, w)
	p := NewPlugin(w, brd)
	parts := map[int][]float32{0: lay(nil, spec, at, level, 0.5, true, h)}
	kept, ranges := render.PackInstances(solidFloats, parts, map[int][]float32{})
	st := &stock{kept: kept, fine: ranges[0], coarse: ranges[1],
		spans: map[int][2]float32{0: {0, 60}}, bays: 8}
	if h != nil {
		inner := inside(spec, at, *h, 0.5)
		inner[15] = 1
		inside, in := render.PackInstances(innerFloats, map[int][]float32{0: inner[:]})
		st.inside, st.inner = inside, in[0]
		st.rooms = []laidRoom{{sector: 0, inner: inner, lamp: 1,
			parts: render.InstanceRange{Count: 1}}}
	}
	p.solids.shown = st
	if bared {
		p.solids.frames[p.solids.frame].bared = []int{0}
	}
	p.solids.frames[p.solids.frame].settings = settings
	c := render.NewComposer(depthOnly{}, p.solids, p.near)
	c.Light(func() *render.Lights {
		return &render.Lights{Sun: render.Distant{Dir: [3]float32{0.3, 0.2, 1}, Strength: 1,
			Ambient: 0.4}}
	})
	screen := render.NewImage(64, 64)
	for range 2 {
		c.DrawWorld(screen, cam)
	}
	pix := make([]byte, 4*64*64)
	screen.ReadPixels(pix)
	return p, pix
}

func blue(c color.RGBA) bool { return int(c.B) > int(c.R)+60 && int(c.B) > int(c.G)+60 }

func TestNear_ARoomAnUncoveredEntityStandsInOpensBeforeTheCamerasTheSettingsSay(t *testing.T) {
	gputest.Need(t)
	tactical := above{cameratest.NewFromSpace(64, 64, 0)}
	riding := behind{above{cameratest.NewFromSpace(64, 64, 0)}}
	inside := inUnit{above{cameratest.NewFromSpace(64, 64, 0)}}
	for name, tc := range map[string]struct {
		cam      camera.Camera
		bared    bool
		settings Settings
		open     bool
	}{
		"bared by none":                 {tactical, false, Settings{}, false},
		"bared, the tactical view":      {tactical, true, Settings{}, true},
		"bared, the tactical view shut": {tactical, true, Settings{Shut: true}, false},
		"bared, behind a unit":          {riding, true, Settings{}, true},
		"bared, behind a unit shut":     {riding, true, Settings{ShutBehind: true}, false},
		"bared, inside a unit":          {inside, true, Settings{}, false},
	} {
		got := housePixel(t, tc.cam, tc.bared, tc.settings)
		switch {
		case tc.open && (blue(got) || got.A == 0):
			t.Errorf("%s: the house shows %v, want its floor through the roof", name, got)
		case !tc.open && !blue(got):
			t.Errorf("%s: the house shows %v, want its blue roof whole", name, got)
		}
	}
}

// looking is a camera in perspective with its eye at eye looking along dir, right the way across
// its screen and up the way up it, a pixel of a screen 64 wide a sixty-fourth of a unit a unit off
type looking struct {
	camera.Camera
	eye, dir, right, up [3]float32
}

func (c looking) Rays() (camera.RayField, bool) {
	const k = 1.0 / 64
	var f camera.RayField
	f.Origin = c.eye
	for i := range 3 {
		f.Dir[i] = c.dir[i] - 32*k*c.right[i] + 32*k*c.up[i]
		f.DDX[i], f.DDY[i] = k*c.right[i], -k*c.up[i]
	}
	return f, true
}

func TestNear_FromInsideItsRoomAHouseShowsItsWallsAndCeilingAllRound(t *testing.T) {
	gputest.Need(t)
	eye := [3]float32{128, 112, 16} // in the room, from its floor at 10 to its ceiling at 22
	for name, way := range map[string][3][3]float32{
		"up":    {{0, 0, 1}, {1, 0, 0}, {0, 1, 0}},
		"west":  {{-1, 0, 0}, {0, -1, 0}, {0, 0, 1}},
		"east":  {{1, 0, 0}, {0, 1, 0}, {0, 0, 1}},
		"north": {{0, -1, 0}, {1, 0, 0}, {0, 0, 1}},
		"south": {{0, 1, 0}, {-1, 0, 0}, {0, 0, 1}},
	} {
		cam := looking{cameratest.NewFromSpace(64, 64, 0), eye, way[0], way[1], way[2]}
		pix := houseFrame(t, cam, false, Settings{})
		i := 4 * (32*64 + 32)
		if got := (color.RGBA{R: pix[i], G: pix[i+1], B: pix[i+2], A: pix[i+3]}); got.A == 0 ||
			blue(got) {
			t.Errorf("looking %s from inside the room the middle shows %v, want its plaster",
				name, got)
		}
	}
}

// slanting looks down the way (x, y, z) from far off, a pixel a quarter of a unit, the middle of
// a screen 64 wide on mid
type slanting struct {
	camera.Camera
	mid, dir [3]float32
}

func slant(cam camera.Camera, mid [3]float32, x, y, z float64) slanting {
	l := math.Sqrt(x*x + y*y + z*z)
	return slanting{cam, mid, [3]float32{float32(x / l), float32(y / l), float32(z / l)}}
}

func (c slanting) Rays() (camera.RayField, bool) {
	const s = 0.25
	d := c.dir
	l := float32(math.Hypot(float64(d[0]), float64(d[1])))
	right := [3]float32{-d[1] / l, d[0] / l, 0}
	down := [3]float32{
		right[1]*d[2] - right[2]*d[1],
		right[2]*d[0] - right[0]*d[2],
		right[0]*d[1] - right[1]*d[0],
	}
	var o, dx, dy [3]float32
	for i := range 3 {
		dx[i], dy[i] = s*right[i], s*down[i]
		o[i] = c.mid[i] - 200*d[i] - 32*dx[i] - 32*dy[i]
	}
	return camera.RayField{Origin: o, DX: dx, DY: dy, Dir: d}, true
}

func brown(c color.RGBA) bool { return int(c.R) > int(c.B)+30 && int(c.G) > int(c.B)+10 }

func TestNear_AHouseOpenRoundItsRoomCutsTheWallsItShowsLow(t *testing.T) {
	gputest.Need(t)
	// through the south wall 6 over the floor, 2 over the low wall it is cut to open, and down
	// on the floor behind it; and the top of that low wall
	cam := slant(cameratest.NewFromSpace(64, 64, 0), [3]float32{128, 128, 16}, 0, -1, -1)
	pixel := func(pix []byte, x, y int) color.RGBA {
		i := 4 * (y*64 + x)
		return color.RGBA{R: pix[i], G: pix[i+1], B: pix[i+2], A: pix[i+3]}
	}
	shut := houseFrame(t, cam, true, Settings{Shut: true})
	if got := pixel(shut, 32, 32); got.A == 0 || brown(got) {
		t.Errorf("shut, the south wall shows %v, want the wall", got)
	}
	open := houseFrame(t, cam, true, Settings{})
	if got := pixel(open, 32, 32); !brown(got) {
		t.Errorf("open, over the low wall shows %v, want the room's floor behind it", got)
	}
	if got := pixel(open, 32, 42); got.A == 0 || brown(got) {
		t.Errorf("open, the top of the low wall shows %v, want it drawn", got)
	}
	// across the room, the north wall 5 over the floor: standing as high as the room
	far := houseFrame(t, slant(cameratest.NewFromSpace(64, 64, 0), [3]float32{128, 100, 18}, 0,
		-1, -1), true, Settings{})
	if got := pixel(far, 32, 32); got.A == 0 || brown(got) {
		t.Errorf("open, the north wall across the room shows %v, want it standing", got)
	}
}

func TestNear_FromInsideARoomItsWallsStandAWallsThicknessIn(t *testing.T) {
	gputest.Need(t)
	// from the room's middle 6 under its ceiling, toward the south wall 16 off: a line of sight
	// rising 0.383 meets that wall's inner face 15 off under the ceiling's edge; the ceiling higher
	cam := looking{cameratest.NewFromSpace(64, 64, 0), [3]float32{128, 112, 16},
		[3]float32{0, 1, 0}, [3]float32{1, 0, 0}, [3]float32{0, 0, 1}}
	pix := houseFrame(t, cam, false, Settings{})
	wall, ceiling := pix[4*(7*64+32)], pix[4*(3*64+32)]
	if wall <= ceiling {
		t.Errorf("from inside, 0.383 up the south wall is %v, no brighter than the ceiling %v: "+
			"the wall stands at the building's face, not a thickness in", wall, ceiling)
	}
}

func TestNear_AHouseShutRoundAnUncoveredEntityStandsWholeGivingWayToNone(t *testing.T) {
	gputest.Need(t)
	cam := slant(cameratest.NewFromSpace(64, 64, 0), [3]float32{128, 128, 16}, 0, -1, -1)
	p, _ := houseDrawn(t, cam, true, Settings{Shut: true})
	if !slices.Equal(p.near.whole, []render.InstanceRange{{Count: 1}}) ||
		len(p.near.cutParts) > 0 {
		t.Errorf("shut, the house's parts are drawn whole %v and giving way %v, want all whole",
			p.near.whole, p.near.cutParts)
	}
}

func TestNear_AHouseOpenHasItsDoorClearThroughItsLowWallAndAThresholdInIt(t *testing.T) {
	gputest.Need(t)
	// the door at 112 in the south wall cut low: 2 over its floor where the low wall's top would
	// cross it, the room's floor behind; at the floor in the wall, its threshold
	pixel := func(mid [3]float32) color.RGBA {
		_, pix := houseDrawn(t, slant(cameratest.NewFromSpace(64, 64, 0), mid, 0, -1, -1), true,
			Settings{})
		i := 4 * (32*64 + 32)
		return color.RGBA{R: pix[i], G: pix[i+1], B: pix[i+2], A: pix[i+3]}
	}
	if got := pixel([3]float32{112, 127.5, 12}); !brown(got) {
		t.Errorf("open, over the door where the low wall's top runs shows %v, want the floor "+
			"behind: nothing across the door", got)
	}
	if got := pixel([3]float32{112, 127.5, 10}); !brown(got) {
		t.Errorf("open, in the door at the floor shows %v, want its threshold", got)
	}
}

func TestNear_AWallStandingByOneCutLowShowsItsEndOverIt(t *testing.T) {
	gputest.Need(t)
	// leaning east, so the east wall stands by the south one cut low: its end, 4 over the floor
	// and 2 over the low wall, faces the eye; past it only the ground
	cam := slant(cameratest.NewFromSpace(64, 64, 0), [3]float32{159.25, 128, 14}, 0.1, -1, -1)
	pix := houseFrame(t, cam, true, Settings{})
	if i := 4 * (32*64 + 32); pix[i+3] == 0 {
		t.Errorf("open, the east wall standing by the low south one shows no end over it")
	}
}

func TestNear_FromInsideARoomTheWindowsTooNearTheDoorAreWall(t *testing.T) {
	gputest.Need(t)
	// the panes either side of the door at 112, 0.24 off it, and the next one, at their middles
	for x, want := range map[float32]bool{108: true, 116: true, 124: false} {
		cam := looking{cameratest.NewFromSpace(64, 64, 0), [3]float32{x, 112, 16},
			[3]float32{0, 1, 0}, [3]float32{1, 0, 0}, [3]float32{0, 0, 1}}
		pix := houseFrame(t, cam, false, Settings{})
		if got := pix[4*(32*64+32)+3] != 0; got != want {
			t.Errorf("from inside, the south wall at %v is wall: %v, want %v", x, got, want)
		}
	}
}

func TestNear_FromInsideARoomAWindowsRevealStandsThroughTheWall(t *testing.T) {
	gputest.Need(t)
	// toward the fourth window of the north wall, round x 124, 1.76 either side: in at its inner
	// face by its east side and out of it through the wall's thickness
	d := [3]float32{13.5, -15, 0}
	l := float32(math.Hypot(13.5, 15))
	d = [3]float32{d[0] / l, d[1] / l, 0}
	cam := looking{cameratest.NewFromSpace(64, 64, 0), [3]float32{112, 112, 16}, d,
		[3]float32{-d[1], d[0], 0}, [3]float32{0, 0, 1}}
	pix := houseFrame(t, cam, false, Settings{})
	if i := 4 * (32*64 + 32); pix[i+3] == 0 {
		t.Errorf("looking slantwise into a window, its reveal is not drawn: the world beyond " +
			"shows through the wall")
	}
}

func TestCut_LeavesTheHolesOutOfTheSpans(t *testing.T) {
	r := func(first, count int) render.InstanceRange {
		return render.InstanceRange{First: first, Count: count}
	}
	spans := []render.InstanceRange{r(0, 10), r(20, 5)}
	holes := []render.InstanceRange{r(22, 1), r(3, 2), r(9, 1), r(0, 1)}
	got := cut(nil, spans, holes)
	want := []render.InstanceRange{r(1, 2), r(5, 4), r(20, 2), r(23, 2)}
	if !slices.Equal(got, want) {
		t.Errorf("the spans %v without %v are %v, want %v", spans, holes, got, want)
	}
	if !inSpans(spans, r(21, 3)) || inSpans(spans, r(8, 3)) {
		t.Errorf("a range in a span is not found in it, or one across its end is")
	}
}
