package topography

import (
	"math"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/render"
)

type flatAtlas struct{}

func (flatAtlas) Atlas() *ebiten.Image                            { return nil }
func (flatAtlas) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 1, 1 }
func (flatAtlas) White() (u, v float32)                           { return 0, 0 }

// lookFn is a Look handed each tile as the landscape dresses it.
type lookFn func(f *render.Frame, cam camera.Camera, t *tile)

// dressedLook hands a lookFn the board's tiles as d dresses them.
type dressedLook struct {
	d  *dresser
	fn lookFn
}

func (l dressedLook) Cell(f *render.Frame, cam camera.Camera, t *board.Tile) {
	l.fn(f, cam, l.d.tileOf(t))
}

// dressed is a renderer of brd's cells dressed by d, handing look each tile.
func dressed(brd *board.Board, d *dresser, look lookFn) *board.Renderer {
	return board.NewRenderer(brd, flatAtlas{}, testMap{look: dressedLook{d, look}, d: d})
}

// testSky is an Atmosphere of a fixed sun and weather.
type testSky struct {
	sun sky.Sun
	air air.Weather
}

func (s testSky) Sun() sky.Sun     { return s.sun }
func (s testSky) Air() air.Weather { return s.air }

// skyOf is a still, clear sky under sun s.
func skyOf(s sky.Sun) testSky { return testSky{sun: s} }

// movingSky is a still, clear sky under whatever sun the pointer holds now.
type movingSky struct{ sun *sky.Sun }

func (s *movingSky) Sun() sky.Sun   { return *s.sun }
func (*movingSky) Air() air.Weather { return air.Weather{} }

// testMap is a board.Map of a dresser and a Look, level where the dresser has no heights.
type testMap struct {
	look board.Look
	d    *dresser
}

func (m testMap) Look() board.Look         { return m.look }
func (m testMap) Dressing() board.Dressing { return m.d }
func (m testMap) Heights() board.Heights   { return m.d.relief }
func (m testMap) Top(c board.CellID) (corners [4]float32, level float32) {
	t := m.d.topOf(c)
	return t.z, t.alt
}
func (testMap) Climb(board.CellID, board.CellID, board.Domain) float64 { return 1 }
func (testMap) Least(board.Domain) float64                             { return 1 }
func (testMap) Slope(geom.Vec, geom.Vec, board.Domain) float64         { return 1 }

// reliefs are the reliefs of the tests' boards, one each, so a board's heights and its dresser
// meet on the same relief.
var reliefs = map[*board.Board]*Relief{}

// reliefFor is brd's relief, made on first use.
func reliefFor(brd *board.Board) *Relief {
	r, ok := reliefs[brd]
	if !ok {
		r = NewRelief(brd)
		reliefs[brd] = r
	}
	return r
}

// compose composes r through cam.
func compose(r *board.Renderer, cam camera.Camera) {
	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
}

// styled is k, styled in st as s.
func styled(st map[board.Name]Style, k board.CellKind, s Style) board.CellKind {
	st[k.Name] = s
	return k
}

// lightsOf composes brd from above in a world with heights under sun and gives each cell's light.
func lightsOf(brd *board.Board, st map[board.Name]Style, sun sky.Sun) map[board.CellID]render.Shade {
	out := map[board.CellID]render.Shade{}
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *tile) { out[t.ID] = t.Light() })
	d := newDresser(brd, reliefFor(brd), skyOf(sun), true, st)
	r := dressed(brd, d, look)
	compose(r, icamera.NewFromSpace(256, 256, 0))
	return out
}

func TestTile_LightFollowsTheSlopeOfTheGround(t *testing.T) {
	st := map[board.Name]Style{}
	grid := board.DefaultGrids{}.Square(4, 4, 32)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	brd.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	hill, _ := grid.CellIndex(1, 1)
	reliefFor(brd).SetHeights(MeanOfCells(grid, func(c board.CellID) float64 {
		if c == hill {
			return 16
		}
		return 0
	}))
	sun := sky.Sun{Dir: [3]float32{1, 0, 1}, Strength: 0.6, Ambient: 0.3} // from the east, 45° up
	lights := lightsOf(brd, st, sun)
	at := func(x, y uint32) render.Shade { c, _ := grid.CellIndex(x, y); return lights[c] }

	level := sun.Light(0, 0, 1)
	if far := at(3, 3); far != render.Lit(level) {
		t.Errorf("level ground far from the hill is lit %v, want %v everywhere", far, level)
	}
	// the hill's east side falls away towards the sun, its west side rises away from it
	if east, west := at(2, 1)[0], at(0, 1)[1]; east[0] <= level[0] || west[0] >= level[0] {
		t.Errorf("the hill's sunny side is lit %v and its shady side %v, want above and below level %v", east, west, level)
	}
	// neighbouring tiles agree on the corner they share: the slope runs on without a seam
	if a, b := at(1, 1)[1], at(2, 1)[0]; a != b {
		t.Errorf("the corner the hill shares with its east neighbour is lit %v from one side and %v from the other", a, b)
	}
}

// A flat world with heights shows its slopes: ground facing the sun lighter than as drawn, ground
// turned away darker.
func TestTile_AFlatWorldShadesItsSlopes(t *testing.T) {
	st := map[board.Name]Style{}
	grid := board.DefaultGrids{}.Square(4, 1, 32)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	brd.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	reliefFor(brd).SetHeights(func(p geom.Vec) float64 { return 16 - math.Abs(p.X-64)/2 }) // a ridge along x = 64
	got := map[board.CellID]render.Shade{}
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *tile) { got[t.ID] = t.Light() })
	d := newDresser(brd, reliefFor(brd), skyOf(sky.DefaultSun), false, st)
	r := dressed(brd, d, look)
	compose(r, icamera.NewFromSpace(128, 32, 0))
	east, _ := grid.CellIndex(2, 0) // falls to the east, where the default sun stands
	west, _ := grid.CellIndex(1, 0)
	if e, w := got[east][1][0], got[west][0][0]; e <= 1 || w >= 1 {
		t.Errorf("the ridge's east side is lit %v and its west side %v, want above and below 1", e, w)
	}
}

// A stream down the middle of a valley runs down it at every corner, as fast as its Flow by the
// square root of the slope; the banks falling into it turn it neither way, and still water does
// not run.
func TestTile_RunningWaterRunsDownItsSlopeAndNotIntoItsBanks(t *testing.T) {
	st := map[board.Name]Style{}
	grid := board.DefaultGrids{}.Square(3, 3, 10)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	brd.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	at := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	for y := range uint32(3) {
		brd.Set(at(1, y), styled(st, board.CellKind{Name: board.Named("k1"), Allows: board.Water}, Style{Shine: 1, Flow: 10}))
	}
	still, _ := grid.CellIndex(0, 1)
	brd.Set(still, styled(st, board.CellKind{Name: board.Named("k2"), Allows: board.Water}, Style{Shine: 1}))
	// falling 0.4 southward, the banks rising half as fast away from the stream
	reliefFor(brd).SetHeights(func(p geom.Vec) float64 { return 40 - 0.4*p.Y + 0.5*math.Abs(p.X-15) })
	flows, runs := map[board.CellID]Flow{}, map[board.CellID]bool{}
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *tile) { flows[t.ID], runs[t.ID] = t.Flow() })
	d := newDresser(brd, reliefFor(brd), skyOf(sky.DefaultSun), false, st)
	r := dressed(brd, d, look)
	compose(r, icamera.NewFromSpace(30, 30, 0))

	want := float32(10 * math.Sqrt(0.4))
	for k, v := range flows[at(1, 1)] {
		if math.Abs(float64(v[0])) > 1e-5 || math.Abs(float64(v[1]-want)) > 1e-4 {
			t.Errorf("corner %d runs at %v, want (0, %v): down the valley, not into a bank", k, v, want)
		}
	}
	if runs[still] || runs[at(2, 1)] {
		t.Errorf("still water runs: %v, a bank runs: %v; want neither", runs[still], runs[at(2, 1)])
	}
}

// wayPieces draws brd from above, near enough for every detail, and returns each tile's way pieces.
func wayPieces(t *testing.T, brd *board.Board, st map[board.Name]Style, w, h uint32) map[board.CellID][]WayPiece {
	t.Helper()
	return wayPiecesAt(t, brd, st, w, h, 2)
}

// wayPiecesAt is wayPieces through a camera at zoom.
func wayPiecesAt(t *testing.T, brd *board.Board, st map[board.Name]Style, w, h uint32, zoom float32) map[board.CellID][]WayPiece {
	t.Helper()
	got := map[board.CellID][]WayPiece{}
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *tile) { got[t.ID] = append([]WayPiece(nil), t.Way()...) })
	d := newDresser(brd, reliefFor(brd), skyOf(sky.DefaultSun), false, st)
	r := dressed(brd, d, look)
	cam := icamera.NewFromSpace(w, h, 0)
	cam.ZoomIn(zoom, float32(w)/2, float32(h)/2)
	if z := cam.Zoom(); math.Abs(float64(z-zoom)) > 1e-4 {
		t.Fatalf("the camera zoomed to %v, want %v", z, zoom)
	}
	cam.CenterOn(float64(w)/2, float64(h)/2, 0)
	compose(r, cam)
	return got
}

// mid is the middle of a piece's k-th end: 0 where it starts, 1 where it ends.
func mid(p WayPiece, k int) [2]float32 {
	return [2]float32{(p.World[k][0] + p.World[k+2][0]) / 2, (p.World[k][1] + p.World[k+2][1]) / 2}
}

func near2(a, b [2]float32) bool {
	return math.Abs(float64(a[0]-b[0])) < 1e-4 && math.Abs(float64(a[1]-b[1])) < 1e-4
}

// A stream straight across a cell is one band from side to side, as wide at each side as the mean
// of it and its neighbour there, the water running down it.
func TestTile_AWayStraightAcrossIsOneBandFromSideToSide(t *testing.T) {
	st := map[board.Name]Style{}
	grid := board.DefaultGrids{}.Square(3, 3, 10)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	brd.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	reliefFor(brd).SetHeights(func(p geom.Vec) float64 { return 20 - 0.5*p.X }) // falling east
	at := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	stream := styled(st, board.CellKind{Name: board.Named("k3"), Allows: board.Land | board.Water}, Style{Shine: 1, Flow: 10})
	west, east := board.Links(1<<2), board.Links(1<<3)
	brd.SetWay(at(1, 1), board.Way{Kind: stream, Width: 4, Links: west | east})
	brd.SetWay(at(0, 1), board.Way{Kind: stream, Width: 8, Links: east})

	pieces := wayPieces(t, brd, st, 30, 30)[at(1, 1)]
	if len(pieces) != 6 {
		t.Fatalf("%d pieces, want the one band in 6", len(pieces))
	}
	first, last := pieces[0], pieces[len(pieces)-1]
	if first.World[0] != [2]float32{10, 18} || first.World[2] != [2]float32{10, 12} {
		t.Errorf("the band starts %v–%v, want 6 wide on the west side: the mean of 4 and 8", first.World[0], first.World[2])
	}
	if !near2(last.World[1], [2]float32{20, 17}) || !near2(last.World[3], [2]float32{20, 13}) {
		t.Errorf("the band ends %v–%v, want 4 wide on the east side", last.World[1], last.World[3])
	}
	// from far, where a cell spans 10 pixels, the band is two pieces between the same ends
	far := wayPiecesAt(t, brd, st, 30, 30, 1)[at(1, 1)]
	if len(far) != 2 || far[0].World[0] != first.World[0] || !near2(far[1].World[3], last.World[3]) {
		t.Errorf("from far %d pieces; want the band in 2 from %v to %v", len(far), first.World[0], last.World[3])
	}
	speed := float32(10 * math.Sqrt(0.5))
	for _, p := range pieces {
		for k, f := range p.Flow {
			if math.Abs(float64(f[0]-speed)) > 1e-4 || math.Abs(float64(f[1])) > 1e-4 {
				t.Errorf("piece from %v corner %d runs at %v, want %v eastward, down the slope", mid(p, 0), k, f, speed)
			}
		}
		if z := p.Z[0]; math.Abs(float64(z-(20-0.5*p.World[0][0]))) > 1e-4 {
			t.Errorf("band corner at %v stands at %v, want on the ground", p.World[0], z)
		}
	}
	if len(wayPieces(t, brd, st, 30, 30)[at(2, 2)]) != 0 {
		t.Error("a cell with no way drew pieces")
	}
}

// A way turning in a cell curves round its middle from side to side, running along the line
// between the middles of the cells at each side.
func TestTile_AWayTurningCurvesRoundTheMiddle(t *testing.T) {
	st := map[board.Name]Style{}
	grid := board.DefaultGrids{}.Square(3, 3, 10)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	brd.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	c, _ := grid.CellIndex(1, 1)
	brd.SetWay(c, board.Way{Kind: board.CellKind{Allows: board.Land}, Width: 4, Links: 1<<2 | 1<<1}) // west and south
	pieces := wayPieces(t, brd, st, 30, 30)[c]
	if len(pieces) != 6 {
		t.Fatalf("%d pieces, want the curve in 6", len(pieces))
	}
	if s, e := mid(pieces[0], 0), mid(pieces[5], 1); !near2(s, [2]float32{10, 15}) && !near2(s, [2]float32{15, 20}) ||
		!near2(e, [2]float32{10, 15}) && !near2(e, [2]float32{15, 20}) || near2(s, e) {
		t.Errorf("the curve runs from %v to %v, want from the west side's middle to the south's", s, e)
	}
	if m := mid(pieces[2], 1); !near2(m, [2]float32{13.75, 16.25}) {
		t.Errorf("halfway the curve is at %v, want (13.75, 16.25): round the middle", m)
	}
	// at each side the band stands square across the line between the cells' middles
	for _, across := range [][2][2]float32{{pieces[0].World[0], pieces[0].World[2]}, {pieces[5].World[1], pieces[5].World[3]}} {
		if dx, dy := across[0][0]-across[1][0], across[0][1]-across[1][1]; math.Abs(float64(dx)) > 1e-4 && math.Abs(float64(dy)) > 1e-4 {
			t.Errorf("at the side the band lies across from %v to %v, at a slant", across[0], across[1])
		}
	}
}

// A way fading out shows the less the further it has faded, the ends of a band as much as the
// mean of the two ways there, down to nothing where it ends; on level ground its water runs on the
// way it fades.
func TestTile_AWayFadingOutShowsLessAndRunsOnTheWayItFades(t *testing.T) {
	st := map[board.Name]Style{}
	grid := board.DefaultGrids{}.Square(4, 1, 10)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	brd.SetAll(board.CellKind{Cost: 1, Allows: board.Water})
	at := func(x uint32) board.CellID { c, _ := grid.CellIndex(x, 0); return c }
	river := styled(st, board.CellKind{Name: board.Named("k4"), Allows: board.Water}, Style{Shine: 1, Flow: 40})
	west, east := board.Links(1<<2), board.Links(1<<3)
	brd.SetWay(at(0), board.Way{Kind: river, Width: 4, Links: east})
	brd.SetWay(at(1), board.Way{Kind: river, Width: 4, Links: west | east, Fade: 0.5})
	brd.SetWay(at(2), board.Way{Kind: river, Width: 4, Links: west, Fade: 0.75})
	pieces := wayPieces(t, brd, st, 40, 10)

	mid := pieces[at(1)]
	if w0, w1 := mid[0].Weight[0], mid[len(mid)-1].Weight[1]; !mid[0].Faded || math.Abs(float64(w0-0.75)) > 1e-5 || math.Abs(float64(w1-0.375)) > 1e-5 {
		t.Errorf("the band shows %v at its west end and %v at its east, faded %v; want 0.75 and 0.375", w0, w1, mid[0].Faded)
	}
	speed := float32(40 * 0.1)
	for _, p := range mid {
		if f := p.Flow[0]; math.Abs(float64(f[0]-speed)) > 1e-4 {
			t.Errorf("level water runs at %v, want %v eastward, the way it fades", f, speed)
		}
	}
	end := pieces[at(2)]
	if w := end[0].Weight[0]; w != 0 {
		t.Errorf("the end of the way shows %v at its tip, want nothing", w)
	}
	if pieces[at(0)][0].Faded {
		t.Error("a way not fading is drawn faded")
	}
}

// A way running on slantwise to its one neighbour runs straight to the corner the two share, the
// stretch reaching into the cells either side of it across the corner, and ends square across
// itself half as far behind the middle as it is wide.
func TestTile_AWayRunsSlantwiseToTheCornerAndEndsSquareAcrossItself(t *testing.T) {
	st := map[board.Name]Style{}
	grid := board.DefaultGrids{}.Square(3, 3, 10)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	brd.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	c, _ := grid.CellIndex(1, 1)
	brd.SetWay(c, board.Way{Kind: board.CellKind{Allows: board.Land}, Width: 2, Links: 1 << 7}) // south-east
	pieces := wayPieces(t, brd, st, 30, 30)[c]
	if len(pieces) != 5 {
		t.Fatalf("%d pieces, want its end and the band in 4", len(pieces))
	}
	d := float32(1 / math.Sqrt2)
	if b, m := mid(pieces[0], 0), mid(pieces[0], 1); !near2(b, [2]float32{15 - d, 15 - d}) || !near2(m, [2]float32{15, 15}) {
		t.Errorf("the end runs from %v to %v, want from 1 behind the middle, slantwise, to the middle", b, m)
	}
	last := pieces[4]
	if !near2(mid(last, 1), [2]float32{20, 20}) || !last.Across || last.Corner != [2]float32{20, 20} {
		t.Errorf("the band ends at %v, across %v at %v; want across the corner (20, 20)", mid(last, 1), last.Across, last.Corner)
	}
	for _, p := range pieces[:3] {
		for _, at := range p.World {
			if at[0] < 10 || at[0] > 20 || at[1] < 10 || at[1] > 20 || p.Across {
				t.Errorf("a piece within the cell reaches %v, out of it", at)
			}
		}
	}
}

// A neighbour of another kind that spreads runs into the tile over the quarters it touches, weighed
// by the share of the cells at each point that are of it; a cell alone is weighed so that only a
// diamond of it shows; nothing blends with a kind that keeps its cells square, nor with one standing
// over its ground.
func TestTile_BlendsWeighTheNeighboursGroundsAtTheTilesPoints(t *testing.T) {
	st := map[board.Name]Style{}
	grid := board.DefaultGrids{}.Square(3, 3, 10)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	earth := styled(st, board.CellKind{Name: board.Named("k5"), Allows: board.Land, SpriteID: 1}, Style{Spread: 0.2})
	sand := styled(st, board.CellKind{Name: board.Named("k6"), Allows: board.Land, SpriteID: 2}, Style{Spread: 0.4})
	brd.SetAll(earth)
	at := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	brd.Set(at(2, 1), sand)
	blends := func() map[board.CellID][]BlendPiece {
		got := map[board.CellID][]BlendPiece{}
		look := lookFn(func(_ *render.Frame, _ camera.Camera, t *tile) { got[t.ID] = append([]BlendPiece(nil), t.Blends()...) })
		d := newDresser(brd, reliefFor(brd), skyOf(sky.DefaultSun), true, st)
		r := dressed(brd, d, look)
		compose(r, icamera.NewFromSpace(30, 30, 0))
		return got
	}
	got := blends()
	pieces := got[at(1, 1)]
	if len(pieces) != 2 {
		t.Fatalf("%d quarters, want the two by the sand: %+v", len(pieces), pieces)
	}
	want := map[render.World][4]float32{
		{{15, 10}, {20, 10}, {15, 15}, {20, 15}}: {0, 0.25, 0, 0.5},
		{{15, 15}, {20, 15}, {15, 20}, {20, 20}}: {0, 0.5, 0, 0.25},
	}
	for _, p := range pieces {
		if w, ok := want[p.World]; !ok || p.Weight != w || p.Sprite != sand.SpriteID || math.Abs(float64(p.Soft-0.3)) > 1e-6 {
			t.Errorf("quarter %v weighs %v, sprite %v, soft %v; want %v of the sand, 0.3 soft", p.World, p.Weight, p.Sprite, p.Soft, w)
		}
	}
	// the sand cell alone: the earth round it weighs three quarters at its corners, a half at its
	// sides, none at its middle — a diamond of sand shows
	var corner [4]float32
	for _, p := range got[at(2, 1)] {
		if p.World[0] == [2]float32{20, 10} {
			corner = p.Weight
		}
	}
	if corner != [4]float32{0.75, 0.5, 0.5, 0} {
		t.Errorf("the earth over the sand's top-left quarter weighs %v, want 0.75 at the corner, 0.5 at the sides, 0 at the middle", corner)
	}

	brd.Set(at(2, 1), board.CellKind{Allows: board.Water, SpriteID: 3}) // the sea keeps its cells square
	if n := len(blends()[at(1, 1)]); n != 0 {
		t.Errorf("%d quarters blend with the sea, want none", n)
	}
}

// By a coast the land tile is drawn as the sea under it and its own kind laid over it by the share
// of land round it; the sea tile has that land laid over it the same way, so the coast runs round.
func TestTile_ByTheSeaTheLandIsLaidOverTheSeaUnderIt(t *testing.T) {
	st := map[board.Name]Style{}
	grid := board.DefaultGrids{}.Square(3, 3, 10)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	earth := styled(st, board.CellKind{Name: board.Named("k7"), Allows: board.Land, SpriteID: 1}, Style{Spread: 0.25})
	sea := styled(st, board.CellKind{Name: board.Named("k8"), Allows: board.Water, SpriteID: 4}, Style{Under: true})
	brd.SetAll(earth)
	at := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	brd.Set(at(0, 1), sea)
	bases, blends := map[board.CellID]render.SpriteID{}, map[board.CellID][]BlendPiece{}
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *tile) {
		bases[t.ID], blends[t.ID] = t.Base(), append([]BlendPiece(nil), t.Blends()...)
	})
	d := newDresser(brd, reliefFor(brd), skyOf(sky.DefaultSun), false, st)
	r := dressed(brd, d, look)
	compose(r, icamera.NewFromSpace(30, 30, 0))

	if bases[at(1, 1)] != sea.SpriteID || bases[at(2, 1)] != earth.SpriteID {
		t.Errorf("bases %v by the sea, %v away from it; want the sea and the earth", bases[at(1, 1)], bases[at(2, 1)])
	}
	var nw [4]float32
	for _, p := range blends[at(1, 1)] {
		if p.Sprite == earth.SpriteID && p.World[0] == [2]float32{10, 10} {
			nw = p.Weight
		}
	}
	if len(blends[at(1, 1)]) != 4 || nw != [4]float32{0.75, 1, 0.5, 1} {
		t.Errorf("the land tile lays %d quarters of its earth, the north-west weighing %v; want 4, 0.75 at the corner by the sea, 0.5 at its side, 1 inland",
			len(blends[at(1, 1)]), nw)
	}
	var ne [4]float32
	for _, p := range blends[at(0, 1)] {
		if p.Sprite == earth.SpriteID && p.World[0] == [2]float32{5, 10} {
			ne = p.Weight
		}
	}
	if ne != [4]float32{0.5, 0.75, 0, 0.5} {
		t.Errorf("the sea's north-east quarter weighs the earth %v, want 0.5 at its sides, 0.75 at the corner, none at its middle", ne)
	}
}

// A tile's bake is worked out anew when a cell round it changes, and kept when one further off does.
func TestRenderer_KeepsATilesBakeUntilACellRoundItChanges(t *testing.T) {
	st := map[board.Name]Style{}
	grid := board.DefaultGrids{}.Square(6, 3, 10)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	earth := styled(st, board.CellKind{Name: board.Named("k9"), Allows: board.Land, SpriteID: 1}, Style{Spread: 0.3})
	sand := styled(st, board.CellKind{Name: board.Named("k10"), Allows: board.Land, SpriteID: 2}, Style{Spread: 0.3})
	brd.SetAll(earth)
	at := func(x uint32) board.CellID { c, _ := grid.CellIndex(x, 1); return c }
	var got []BlendPiece
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *tile) {
		if t.ID == at(1) {
			got = append(got[:0], t.Blends()...)
		}
	})
	d := newDresser(brd, reliefFor(brd), skyOf(sky.DefaultSun), false, st)
	r := dressed(brd, d, look)
	cam := icamera.NewFromSpace(60, 30, 0)
	compose(r, cam)
	baked := d.bakes[1*6+1].ver
	if len(got) != 0 {
		t.Fatalf("%d blends among earth alone", len(got))
	}

	brd.Set(at(5), sand) // four cells off
	compose(r, cam)
	if d.bakes[1*6+1].ver != baked {
		t.Error("a change four cells off worked the bake out anew")
	}
	brd.Set(at(2), sand) // next door
	compose(r, cam)
	if d.bakes[1*6+1].ver == baked || len(got) == 0 {
		t.Errorf("after the cell next door turned to sand the bake is %d (was %d) with %d blends; want it anew, blending",
			d.bakes[1*6+1].ver, baked, len(got))
	}
}

// Near, water glints with all its shine and rolls in on a shore; far off, it does not glint at all;
// between, it glints the less the further off it is.
func TestTile_FarOffTheWaterGlintsLessAndThenNot(t *testing.T) {
	st := map[board.Name]Style{}
	grid := board.DefaultGrids{}.Square(4, 4, 32)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	brd.SetAll(styled(st, board.CellKind{Name: board.Named("k11"), Allows: board.Water, SpriteID: 1}, Style{Shine: 0.8}))
	land, _ := grid.CellIndex(0, 0)
	brd.Set(land, board.CellKind{Allows: board.Land, SpriteID: 2})
	sea, _ := grid.CellIndex(1, 1)
	var shine float32
	var glints bool
	var shore Shore
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *tile) {
		if t.ID == sea {
			var s float32
			s, _, glints = t.Shine()
			shine, shore = s, t.Shore()
		}
	})
	d := newDresser(brd, reliefFor(brd), skyOf(sky.DefaultSun), true, st)
	r := dressed(brd, d, look)
	for _, c := range []struct {
		zoom   float32
		shine  float32
		glints bool
		shore  bool
	}{{1, 0.8, true, true}, {9.0 / 32, 0.4, true, false}, {4.0 / 32, 0, false, false}} {
		cam := icamera.NewFromSpace(128, 128, 0, geom.NewAABBAt(geom.NewVec(0, 0), 16, 16)) // room to zoom out to 1/8
		cam.ZoomIn(c.zoom, 64, 64)
		if z := cam.Zoom(); math.Abs(float64(z-c.zoom)) > 1e-4 {
			t.Fatalf("the camera zoomed to %v, want %v", z, c.zoom)
		}
		cam.CenterOn(48, 48, 0) // over the sea cell
		shine, glints, shore = -1, false, Shore{}
		compose(r, cam)
		if math.Abs(float64(shine-c.shine)) > 1e-4 || glints != c.glints || (shore != Shore{}) != c.shore {
			t.Errorf("at %v px a cell: shine %v, glints %v, a shore %v; want %v, %v, %v",
				32*c.zoom, shine, glints, shore != Shore{}, c.shine, c.glints, c.shore)
		}
	}
}

// The composer's shader compiles with the materials the board and the world bring.
func TestWater_TheShaderCompilesWithTheBoardsMaterials(t *testing.T) {
	if err := render.Compile(); err != nil {
		t.Fatal(err)
	}
	if src := string(render.ShaderSource()); !strings.Contains(src, "return SeaGlint(") || !strings.Contains(src, "return CloudShadow(") {
		t.Error("the shader hands no overlay to the sea's or the clouds' material")
	}
}

// Over a still sea the surface is the sea's glint, over running water the running water's, each
// overlay marked with its material and the sun reaching it.
func TestTile_DrawSurfaceLaysTheWatersMaterial(t *testing.T) {
	st := map[board.Name]Style{}
	grid := board.DefaultGrids{}.Square(3, 3, 32)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	brd.SetAll(styled(st, board.CellKind{Name: board.Named("k12"), Allows: board.Water, SpriteID: 1}, Style{Shine: 1}))
	reliefFor(brd).SetHeights(func(p geom.Vec) float64 { return 30 - p.X/10 })
	c, _ := grid.CellIndex(1, 1)
	marks := map[bool]float32{}
	for _, running := range []bool{false, true} {
		if running {
			brd.SetAll(styled(st, board.CellKind{Name: board.Named("k13"), Allows: board.Water, SpriteID: 1}, Style{Shine: 1, Flow: 20}))
		}
		var f render.Frame
		cam := icamera.NewFromSpace(96, 96, 0)
		look := lookFn(func(f *render.Frame, _ camera.Camera, t *tile) {
			if t.ID == c {
				f.Sprite(render.Ground, 0, flatAtlas{}, 0, render.Corners{{0, 0}, {1, 0}, {0, 1}, {1, 1}}, render.Even(1))
				t.DrawSurface(f, t.X0, t.Y0, t.X1, t.Y1)
			}
		})
		d := newDresser(brd, reliefFor(brd), skyOf(sky.DefaultSun), true, st)
		r := dressed(brd, d, look)
		f.Reset(cam)
		r.Compose(&f, cam)
		f.Each(func(_ render.Tier, _ float32, v []ebiten.Vertex) {
			if v[0].ColorA > 1.5 { // an overlay
				marks[running] = v[0].ColorA
			}
		})
	}
	sea, run := float32(2+2*seaGlint), float32(2+2*runningWater)
	if m := marks[false]; m < sea || m > sea+1 {
		t.Errorf("a still sea's overlay is marked %v, want the sea's glint, %v and the sun above it", m, sea)
	}
	if m := marks[true]; m < run || m > run+1 {
		t.Errorf("running water's overlay is marked %v, want the running water's, %v and the sun above it", m, run)
	}
}

// wallInSun is a 6x3 board of level grass with a wall 10 tall at (3, 1), under a sun low in the
// east: its shadow falls 50 to the west.
func wallInSun(t *testing.T) (*board.Board, board.Grid, sky.Sun) {
	t.Helper()
	grid := board.DefaultGrids{}.Square(6, 3, 32)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	brd.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	wall, _ := grid.CellIndex(3, 1)
	brd.Set(wall, board.CellKind{Cost: 1, Allows: board.Land, Solid: true, Height: 10})
	return brd, grid, sky.Sun{Dir: [3]float32{1, 0, 0.2}, Strength: 0.6, Ambient: 0.3}
}

func TestTile_TheTerrainCastsItsShadowAwayFromTheSun(t *testing.T) {
	st := map[board.Name]Style{}
	brd, grid, sun := wallInSun(t)
	lights := lightsOf(brd, st, sun)
	at := func(x uint32) render.Shade { c, _ := grid.CellIndex(x, 1); return lights[c] }
	lit, shade := sun.Light(0, 0, 1), sun.Shaded(0, 0, 1, 0)

	// the wall's west edge is at x 96: the grass right behind it is in shadow up to 50 away
	if got := at(2); got[1] != shade || got[0] != shade {
		t.Errorf("the grass behind the wall is lit %v, want its corners 32 and 0 from the wall in shadow %v", got, shade)
	}
	if got := at(1); got[1] != shade || got[0] != lit {
		t.Errorf("the next tile is lit %v, want 32 from the wall in shadow and 64 from it in the sun", got)
	}
	if got := at(4); got[0] != lit || got[1] != lit {
		t.Errorf("the grass on the sunny side is lit %v, want %v", got, lit)
	}
}

func TestTile_AShadowGoesWithWhatCastItAndWithTheSun(t *testing.T) {
	st := map[board.Name]Style{}
	brd, grid, sun := wallInSun(t)
	var got map[board.CellID]render.Shade
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *tile) { got[t.ID] = t.Light() })
	current := sun
	d := newDresser(brd, reliefFor(brd), &movingSky{sun: &current}, true, st)
	r := dressed(brd, d, look)
	behind, _ := grid.CellIndex(2, 1)
	frame := func() render.Shade {
		got = map[board.CellID]render.Shade{}
		compose(r, icamera.NewFromSpace(192, 96, 0))
		return got[behind]
	}
	if frame()[1] != sun.Shaded(0, 0, 1, 0) {
		t.Fatal("no shadow behind the wall to begin with")
	}
	noon := sky.Sun{Dir: [3]float32{0, 0, 1}, Strength: 0.6, Ambient: 0.3}
	current = noon
	if l := frame()[1]; l != noon.Light(0, 0, 1) {
		t.Errorf("under a sun overhead the grass behind the wall is lit %v, want %v: no shadow", l, noon.Light(0, 0, 1))
	}
	current = sun
	wall, _ := grid.CellIndex(3, 1)
	brd.Set(wall, board.CellKind{Cost: 1, Allows: board.Land})
	if l := frame()[1]; l != sun.Light(0, 0, 1) {
		t.Errorf("with the wall knocked down the grass is lit %v, want the full sun %v", l, sun.Light(0, 0, 1))
	}
	d.shadows = false
	brd.Set(wall, board.CellKind{Cost: 1, Allows: board.Land, Solid: true, Height: 10})
	if l := frame()[1]; l != sun.Light(0, 0, 1) {
		t.Errorf("with shadows off the grass behind the wall is lit %v, want the full sun", l)
	}
}

func TestTile_AShinyCellShinesAsMuchAsTheSunReachesIt(t *testing.T) {
	st := map[board.Name]Style{}
	grid := board.DefaultGrids{}.Square(4, 4, 32)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	brd.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	sea, _ := grid.CellIndex(1, 1)
	grass, _ := grid.CellIndex(3, 3)
	brd.Set(sea, styled(st, board.CellKind{Name: board.Named("k14"), Cost: 1, Allows: board.Water}, Style{Shine: 0.8}))
	type shining struct {
		shine float32
		lit   [4]float32
	}
	shines := func(sun sky.Sun, heights bool) map[board.CellID]shining {
		out := map[board.CellID]shining{}
		look := lookFn(func(_ *render.Frame, _ camera.Camera, t *tile) {
			if s, lit, ok := t.Shine(); ok {
				out[t.ID] = shining{s, lit}
			}
		})
		d := newDresser(brd, reliefFor(brd), skyOf(sun), heights, st)
		r := dressed(brd, d, look)
		compose(r, icamera.NewFromSpace(256, 256, 0))
		return out
	}
	day := sky.Sun{Dir: [3]float32{0, 0, 1}, Strength: 0.6, Ambient: 0.3}
	got := shines(day, true)
	if got[sea] != (shining{0.8, [4]float32{1, 1, 1, 1}}) {
		t.Errorf("the sea in the full sun shines %v, want its kind's 0.8, all the sun at every corner", got[sea])
	}
	if _, ok := got[grass]; ok {
		t.Errorf("grass shines %v, want nothing", got[grass])
	}
	if got := shines(sky.Sun{Dir: [3]float32{0, 0, -1}, Ambient: 0.1}, true); got[sea] != (shining{0.8, [4]float32{}}) {
		t.Errorf("at night the sea shines %v, want its shine and none of the sun: it reflects the night sky, foams", got[sea])
	}
	if got := shines(day, false); len(got) > 0 {
		t.Errorf("in a flat world %d cells shine, want none: it is drawn as its sprites are", len(got))
	}
}

func TestTile_TheShoreLiesTheWayOfTheNearestCellThatDoesNotShine(t *testing.T) {
	st := map[board.Name]Style{}
	grid := board.DefaultGrids{}.Square(10, 4, 32)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	brd.SetAll(styled(st, board.CellKind{Name: board.Named("k15"), Cost: 1, Allows: board.Water}, Style{Shine: 1}))
	for y := range uint32(4) {
		land, _ := grid.CellIndex(0, y)
		brd.Set(land, board.CellKind{Cost: 1, Allows: board.Land}) // a coast along x = 32, the sea east of it
	}
	shores := map[board.CellID]Shore{}
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *tile) { shores[t.ID] = t.Shore() })
	sun := sky.Sun{Dir: [3]float32{0, 0, 1}, Strength: 0.6}
	d := newDresser(brd, reliefFor(brd), skyOf(sun), true, st)
	r := dressed(brd, d, look)
	compose(r, icamera.NewFromSpace(320, 128, 0))
	at := func(x uint32) Shore { c, _ := grid.CellIndex(x, 1); return shores[c] }

	if c := at(1)[0]; c.Dist != 0 || c.Near != 1 || abs32(c.X+1) > 1e-4 || abs32(c.Y) > 1e-4 {
		t.Errorf("on the coast a corner sees the shore %+v, want it right there to the west", c)
	}
	if c := at(2)[1]; abs32(c.Dist-64) > 1e-4 || abs32(c.Near-1.0/3) > 1e-4 || abs32(c.X+1) > 1e-4 {
		t.Errorf("two cells out a corner sees the shore %+v, want it 64 to the west, a third near", c)
	}
	if c := at(6)[0]; c.Near != 0 || c.X != 0 || c.Y != 0 {
		t.Errorf("out at sea a corner sees the shore %+v, want open water", c)
	}
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// Under clouds each tile's top takes their shadow once, over the grounds running in on it too.
func TestTile_TheCloudsShadowLiesOnceOverATopAndTheGroundsOnIt(t *testing.T) {
	st := map[board.Name]Style{}
	grid := board.DefaultGrids{}.Square(3, 3, 32)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	brd.SetAll(styled(st, board.CellKind{Name: board.Named("k40"), Allows: board.Land, SpriteID: 1}, Style{Spread: 0.3}))
	c, _ := grid.CellIndex(1, 1)
	brd.Set(c, styled(st, board.CellKind{Name: board.Named("k41"), Allows: board.Water, SpriteID: 4}, Style{Under: true}))
	look := lookFn(func(f *render.Frame, cam camera.Camera, t *tile) {
		f.Sprite(render.Ground, 0, flatAtlas{}, t.Base(), render.Corners{{t.X0, t.Y0}, {t.X1, t.Y0}, {t.X0, t.Y1}, {t.X1, t.Y1}}, t.Light())
		t.Tile.Dress(f, cam, t.X0, t.Y0, t.X1, t.Y1, 0)
	})
	d := newDresser(brd, reliefFor(brd), skyOf(sky.DefaultSun), false, st)
	r := dressed(brd, d, look)
	var f render.Frame
	cam := icamera.NewFromSpace(96, 96, 0)
	f.Reset(cam)
	d.sky = testSky{sun: sky.DefaultSun, air: air.Weather{Clouds: 1}} // an overcast sky: every top the clouds reach is shaded
	r.Compose(&f, cam)

	shadow := float32(2 + 2*air.CloudShadow())
	var top []ebiten.Vertex
	shadows, grounds := 0, 0
	f.Each(func(_ render.Tier, _ float32, v []ebiten.Vertex) {
		switch {
		case v[0].ColorA >= shadow && v[0].ColorA <= shadow+1:
			shadows++
			if top == nil || v[0].DstX != top[0].DstX || v[3].DstY != top[3].DstY {
				t.Errorf("a shadow lies at %v,%v; want over the top before it", v[0].DstX, v[0].DstY)
			}
			top = nil
		case v[0].ColorA > 1.5: // another overlay
		case top == nil:
			top = v
		default:
			grounds++
		}
	})
	// under a sky all covered the clouds' noise (fixed, the drift 0) reaches every one of the 9 tops
	if shadows != 9 || grounds == 0 {
		t.Errorf("%d shadows over 9 tops with %d grounds running in on them; want one each, some grounds", shadows, grounds)
	}
}

// sheetAtlas is an atlas on a sheet of its own, 8 pixels a sprite.
type sheetAtlas struct{ img *ebiten.Image }

func (a sheetAtlas) Atlas() *ebiten.Image { return a.img }
func (sheetAtlas) UV(id render.SpriteID) (sx0, sy0, sx1, sy1 float32) {
	return float32(id) * 8, 0, float32(id)*8 + 8, 8
}
func (sheetAtlas) White() (u, v float32) { return 1, 1 }

// From far a tile is dressed from the ground sheet: its top is drawn from the sheet, what lies over
// it is one piece of it painted once, and a tile nothing lies over has none; near, the grounds
// running in are drawn piece by piece from the board's atlas.
func TestDresser_FromFarTilesAreDressedFromTheGroundSheet(t *testing.T) {
	st := map[board.Name]Style{}
	grid := board.DefaultGrids{}.Square(4, 4, 32)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	brd.SetAll(styled(st, board.CellKind{Name: board.Named("k42"), Allows: board.Land, SpriteID: 1}, Style{Spread: 0.3}))
	at := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	brd.Set(at(0, 0), styled(st, board.CellKind{Name: board.Named("k43"), Allows: board.Water, SpriteID: 2}, Style{Under: true}))
	atlas := sheetAtlas{ebiten.NewImage(24, 8)}
	sheets := map[board.CellID]render.AtlasSource{}
	look := lookFn(func(f *render.Frame, cam camera.Camera, t *tile) {
		sheets[t.ID] = t.Atlas
		f.Sprite(render.Ground, 0, t.Atlas, t.Base(), render.Corners{{t.X0, t.Y0}, {t.X1, t.Y0}, {t.X0, t.Y1}, {t.X1, t.Y1}}, t.Light())
		t.Tile.Dress(f, cam, t.X0, t.Y0, t.X1, t.Y1, 0)
	})
	d := newDresser(brd, reliefFor(brd), skyOf(sky.DefaultSun), false, st)
	r := board.NewRenderer(brd, atlas, testMap{look: dressedLook{d, look}, d: d})

	for _, c := range []struct {
		zoom float32
		far  bool
	}{{0.25, true}, {1, false}} {
		cam := icamera.NewFromSpace(128, 128, 0, geom.NewAABBAt(geom.NewVec(0, 0), 16, 16))
		cam.ZoomIn(c.zoom, 64, 64)
		cam.CenterOn(64, 64, 0)
		var f render.Frame
		f.Reset(cam)
		clear(sheets)
		r.Compose(&f, cam)
		fromSheet, blends := 0, 0
		f.Each(func(_ render.Tier, _ float32, v []ebiten.Vertex) {
			switch {
			case v[0].Custom3 > 5.5:
				blends++
			case v[0].ColorA <= 1.5 && v[0].SrcY >= 8: // below the atlas: a piece of the painted cells
				fromSheet++
			}
		})
		sheet, ok := sheets[at(2, 2)].(*groundSheet)
		switch {
		case c.far && (!ok || sheets[at(0, 0)] != sheet):
			t.Errorf("from far the tiles are drawn from %T, want the ground sheet", sheets[at(2, 2)])
		case c.far && (blends != 0 || fromSheet != 4 || sheet.dressed[3*4+3]):
			t.Errorf("from far %d grounds drawn running in, %d pieces of the sheet; want none and one each over the sea and its 3 neighbours",
				blends, fromSheet)
		case !c.far && (ok || blends == 0 || fromSheet != 0):
			t.Errorf("near the tiles are drawn from %T with %d grounds running in, %d pieces of a sheet; want the atlas, some, none",
				sheets[at(2, 2)], blends, fromSheet)
		}
	}

	// the sheet is painted anew where the board changes
	brd.Set(at(3, 3), board.CellKind{Name: board.Named("k43"), Allows: board.Water, SpriteID: 2})
	cam := icamera.NewFromSpace(128, 128, 0, geom.NewAABBAt(geom.NewVec(0, 0), 16, 16))
	cam.ZoomIn(0.25, 64, 64)
	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	if !d.sheet.dressed[3*4+3] || !d.sheet.dressed[2*4+2] {
		t.Errorf("a sea laid at 3, 3 left it and its neighbour undressed on the sheet")
	}
}

// kindsOf is a CellKindDict of the kinds given.
type kindsOf []board.CellKind

func (k kindsOf) Create(...board.CellKind)         {}
func (k kindsOf) Draw(string, render.SpriteDrawer) {}
func (k kindsOf) All() []board.CellKind            { return k }
func (k kindsOf) Get(name string) (board.CellKind, bool) {
	for _, c := range k {
		if c.Name == board.Named(name) {
			return c, true
		}
	}
	return board.CellKind{}, false
}

// A way turning into another kind's look carries that kind's sprite and how far it has turned at
// each corner, the mean of its cell's Mix and its neighbour's where a band meets it.
func TestTile_AWayTurnsIntoTheKindItMixesWith(t *testing.T) {
	st := map[board.Name]Style{}
	grid := board.DefaultGrids{}.Square(3, 3, 10)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	brd.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	sea := board.CellKind{Name: board.Named("k44"), Allows: board.Water, SpriteID: 7}
	stream := styled(st, board.CellKind{Name: board.Named("k45"), Allows: board.Land | board.Water, SpriteID: 3}, Style{MixWith: "k44"})
	at := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	west, east := board.Links(1<<2), board.Links(1<<3)
	brd.SetWay(at(0, 1), board.Way{Kind: stream, Width: 4, Links: east, Mix: 0})
	brd.SetWay(at(1, 1), board.Way{Kind: stream, Width: 4, Links: west | east, Mix: 0.5})
	brd.SetWay(at(2, 1), board.Way{Kind: stream, Width: 4, Links: west, Mix: 1})

	var pieces []WayPiece
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *tile) {
		if t.ID == at(1, 1) {
			pieces = append([]WayPiece(nil), t.Way()...)
		}
	})
	d := newDresser(brd, reliefFor(brd), skyOf(sky.DefaultSun), false, st)
	d.kinds = kindsOf{sea, stream}
	r := dressed(brd, d, look)
	compose(r, icamera.NewFromSpace(30, 30, 0))
	if len(pieces) == 0 {
		t.Fatal("no pieces")
	}
	first, last := pieces[0], pieces[len(pieces)-1]
	if !first.Mixes || first.MixSprite != 7 {
		t.Fatalf("the way mixes %v with sprite %v, want the sea's, 7", first.Mixes, first.MixSprite)
	}
	if math.Abs(float64(first.Mix[0]-0.25)) > 1e-5 || math.Abs(float64(last.Mix[1]-0.75)) > 1e-5 {
		t.Errorf("the band turns from %v to %v, want 0.25 on the west side to 0.75 on the east", first.Mix[0], last.Mix[1])
	}
}

// A way running out into water runs on to the water's middle under it: its pieces show only where
// the land does, as the grounds round the coast are laid over the water, none at the water's middle.
func TestTile_AWayRunsOnUnderTheWaterItRunsInto(t *testing.T) {
	st := map[board.Name]Style{}
	grid := board.DefaultGrids{}.Square(3, 3, 10)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	brd.SetAll(styled(st, board.CellKind{Name: board.Named("k49"), Allows: board.Land, SpriteID: 1}, Style{Spread: 0.3}))
	sea := styled(st, board.CellKind{Name: board.Named("k50"), Allows: board.Water, SpriteID: 4}, Style{Under: true})
	stream := styled(st, board.CellKind{Name: board.Named("k51"), Allows: board.Land | board.Water, SpriteID: 5}, Style{})
	at := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	for x := range uint32(3) {
		brd.Set(at(x, 2), sea)
	}
	north, south := board.Links(1<<0), board.Links(1<<1)
	brd.SetWay(at(1, 0), board.Way{Kind: stream, Width: 4, Links: south})
	brd.SetWay(at(1, 1), board.Way{Kind: stream, Width: 4, Links: north | south})

	pieces := wayPieces(t, brd, st, 30, 30)[at(1, 1)]
	deepest := pieces[0]
	for _, p := range pieces {
		if p.World[3][1] > deepest.World[3][1] {
			deepest = p
		}
	}
	if y := deepest.World[3][1]; y < 24.9 {
		t.Fatalf("the way ends at y %v, want on to the water's middle, 25", y)
	}
	if !deepest.Faded || deepest.Weight[3] != 0 || deepest.Soft != 0.3 {
		t.Errorf("at the water's middle the way shows %v (faded %v, soft %v), want nothing, blended as the coast is", deepest.Weight, deepest.Faded, deepest.Soft)
	}
	for _, p := range pieces {
		if p.World[3][1] <= 12 && p.Faded {
			t.Errorf("a piece well ashore, down to y %v, lies under water", p.World[3][1])
		}
	}
}

// A crossing is cut into pieces as a way is, over the way under it: a bridge runs from its cell's
// middle out to the road either side, as wide there as the mean of the two; the road meets the
// bridge, not the river under it.
func TestTile_ABridgeRunsOverItsRiverOnToTheRoad(t *testing.T) {
	st := map[board.Name]Style{}
	grid := board.DefaultGrids{}.Square(3, 3, 10)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	brd.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	river := board.CellKind{Name: board.Named("k52"), Allows: board.Water, SpriteID: 4}
	road := board.CellKind{Name: board.Named("k53"), Allows: board.Land, SpriteID: 6}
	bridge := board.CellKind{Name: board.Named("k54"), Allows: board.Land, SpriteID: 7}
	at := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	north, south, west, east := board.Links(1<<0), board.Links(1<<1), board.Links(1<<2), board.Links(1<<3)
	for y := range uint32(3) {
		brd.SetWay(at(1, y), board.Way{Kind: river, Width: 8, Links: north | south})
	}
	brd.SetWay(at(0, 1), board.Way{Kind: road, Width: 4, Links: east})
	brd.SetWay(at(2, 1), board.Way{Kind: road, Width: 4, Links: west})
	brd.SetCrossing(at(1, 1), board.Crossing{Way: board.Way{Kind: bridge, Width: 6, Links: west | east}})

	var ways, crossings map[board.CellID][]WayPiece = map[board.CellID][]WayPiece{}, map[board.CellID][]WayPiece{}
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *tile) {
		ways[t.ID] = append([]WayPiece(nil), t.Way()...)
		crossings[t.ID] = append([]WayPiece(nil), t.Crossing()...)
	})
	d := newDresser(brd, reliefFor(brd), skyOf(sky.DefaultSun), false, st)
	r := dressed(brd, d, look)
	cam := icamera.NewFromSpace(30, 30, 0)
	cam.ZoomIn(2, 15, 15)
	cam.CenterOn(15, 15, 0)
	compose(r, cam)

	bridgePieces := crossings[at(1, 1)]
	if len(bridgePieces) == 0 || bridgePieces[0].Sprite != 7 || len(ways[at(1, 1)]) == 0 || ways[at(1, 1)][0].Sprite != 4 {
		t.Fatalf("at the crossing %d bridge pieces over %d of river, want both", len(bridgePieces), len(ways[at(1, 1)]))
	}
	for _, p := range bridgePieces {
		if p.World[0][1] < 10 || p.World[2][1] > 20 {
			t.Errorf("a piece of the bridge runs %v, want across the cell west to east", p.World)
		}
	}
	// the road's end at the bridge is as wide as the mean of road and bridge, 5, not of road and river, 6
	road0 := ways[at(0, 1)]
	last := road0[len(road0)-1]
	if w := math.Abs(float64(last.World[3][1] - last.World[1][1])); math.Abs(w-5) > 0.01 {
		t.Errorf("the road meets the bridge %v wide, want 5", w)
	}
}
