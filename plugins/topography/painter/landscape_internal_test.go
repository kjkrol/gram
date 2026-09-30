package painter

import (
	"math"
	"strings"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/topography/relief"
	"github.com/kjkrol/gram/plugins/topography/water"
	"github.com/kjkrol/gram/render"
)

type flatAtlas struct{}

func (flatAtlas) Atlas() *render.Image                            { return nil }
func (flatAtlas) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 1, 1 }
func (flatAtlas) White() (u, v float32)                           { return 0, 0 }

// lookFn is a Look handed each tile as the landscape dresses it.
type lookFn func(f *render.Frame, cam camera.Camera, t *tile)

// dressedLook hands a lookFn the board's tiles as d dresses them.
type dressedLook struct {
	d  *Painter
	fn lookFn
}

func (l dressedLook) Cell(f *render.Frame, cam camera.Camera, t *board.Tile) {
	l.fn(f, cam, l.d.tileOf(t))
}

// dressed is a renderer of brd's cells dressed by d, handing look each tile.
func dressed(brd *board.Board, d *Painter, look lookFn) *board.Renderer {
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

// testMap is a board.Map of a Painter and a Look, level where the Painter has no heights.
type testMap struct {
	look board.Look
	d    *Painter
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

// reliefs are the reliefs of the tests' boards, one each, so a board's heights and its Painter
// meet on the same relief.
var reliefs = map[*board.Board]*relief.Relief{}

// reliefFor is brd's relief, made on first use.
func reliefFor(brd *board.Board) *relief.Relief {
	r, ok := reliefs[brd]
	if !ok {
		r = relief.New(brd)
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
	flows, runs := map[board.CellID]water.Flow{}, map[board.CellID]bool{}
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *tile) { flows[t.ID], runs[t.ID] = t.Flow() })
	d := New(brd, reliefFor(brd), skyOf(sky.DefaultSun), false, st)
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
	d := New(brd, reliefFor(brd), skyOf(sky.DefaultSun), false, st)
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
		d := New(brd, reliefFor(brd), skyOf(sky.DefaultSun), true, st)
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
	d := New(brd, reliefFor(brd), skyOf(sky.DefaultSun), false, st)
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
	d := New(brd, reliefFor(brd), skyOf(sky.DefaultSun), false, st)
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
	var shore water.Shores
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *tile) {
		if t.ID == sea {
			var s float32
			s, _, glints = t.Shine()
			shine, shore = s, t.Shore()
		}
	})
	d := New(brd, reliefFor(brd), skyOf(sky.DefaultSun), true, st)
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
		shine, glints, shore = -1, false, water.Shores{}
		compose(r, cam)
		if math.Abs(float64(shine-c.shine)) > 1e-4 || glints != c.glints || (shore != water.Shores{}) != c.shore {
			t.Errorf("at %v px a cell: shine %v, glints %v, a shore %v; want %v, %v, %v",
				32*c.zoom, shine, glints, shore != water.Shores{}, c.shine, c.glints, c.shore)
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
		d := New(brd, reliefFor(brd), skyOf(sky.DefaultSun), true, st)
		r := dressed(brd, d, look)
		f.Reset(cam)
		r.Compose(&f, cam)
		f.Each(func(_ render.Tier, _ float32, v []render.Vertex) {
			if v[0].ColorA > 1.5 { // an overlay
				marks[running] = v[0].ColorA
			}
		})
	}
	sea, run := float32(2+2*water.SeaGlint()), float32(2+2*water.RunningWater())
	if m := marks[false]; m < sea || m > sea+1 {
		t.Errorf("a still sea's overlay is marked %v, want the sea's glint, %v and the sun above it", m, sea)
	}
	if m := marks[true]; m < run || m > run+1 {
		t.Errorf("running water's overlay is marked %v, want the running water's, %v and the sun above it", m, run)
	}
}

// A shiny kind's cell shines as much as its kind's Shine says, all the sun on it — the GPU casts the
// shadows — and nothing else shines; in a flat world nothing does.
func TestTile_AShinyCellShines(t *testing.T) {
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
		d := New(brd, reliefFor(brd), skyOf(sun), heights, st)
		r := dressed(brd, d, look)
		compose(r, icamera.NewFromSpace(256, 256, 0))
		return out
	}
	day := sky.Sun{Dir: [3]float32{0, 0, 1}, Strength: 0.6, Ambient: 0.3}
	got := shines(day, true)
	if got[sea] != (shining{0.8, [4]float32{1, 1, 1, 1}}) {
		t.Errorf("the sea shines %v, want its kind's 0.8, all the sun at every corner", got[sea])
	}
	if _, ok := got[grass]; ok {
		t.Errorf("grass shines %v, want nothing", got[grass])
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
	shores := map[board.CellID]water.Shores{}
	look := lookFn(func(_ *render.Frame, _ camera.Camera, t *tile) { shores[t.ID] = t.Shore() })
	sun := sky.Sun{Dir: [3]float32{0, 0, 1}, Strength: 0.6}
	d := New(brd, reliefFor(brd), skyOf(sun), true, st)
	r := dressed(brd, d, look)
	compose(r, icamera.NewFromSpace(320, 128, 0))
	at := func(x uint32) water.Shores { c, _ := grid.CellIndex(x, 1); return shores[c] }

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

// sheetAtlas is an atlas on a sheet of its own, 8 pixels a sprite.
type sheetAtlas struct{ img *render.Image }

func (a sheetAtlas) Atlas() *render.Image { return a.img }
func (sheetAtlas) UV(id render.SpriteID) (sx0, sy0, sx1, sy1 float32) {
	return float32(id) * 8, 0, float32(id)*8 + 8, 8
}
func (sheetAtlas) White() (u, v float32) { return 1, 1 }

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
	d := New(brd, reliefFor(brd), skyOf(sky.DefaultSun), false, st)
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
	d := New(brd, reliefFor(brd), skyOf(sky.DefaultSun), false, st)
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

// The board's albedo is every cell's base painted over its whole cell — the sea under the coast's
// land — with the grounds running in and the ways over it, painted anew for the cells that change
// and left alone while nothing does.
func TestPainter_TheAlbedoPaintsEveryCellsBaseUnderItsGroundsAndWays(t *testing.T) {
	st := map[board.Name]Style{}
	grid := board.DefaultGrids{}.Square(4, 4, 32)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	land := styled(st, board.CellKind{Name: board.Named("k44"), Allows: board.Land, SpriteID: 1}, Style{Spread: 0.3})
	sea := styled(st, board.CellKind{Name: board.Named("k45"), Allows: board.Water, SpriteID: 2}, Style{Under: true})
	road := styled(st, board.CellKind{Name: board.Named("k46"), Allows: board.Land, SpriteID: 0}, Style{})
	brd.SetAll(land)
	at := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	brd.Set(at(0, 0), sea)
	brd.SetWay(at(2, 2), board.Way{Kind: road, Width: 8, Links: board.Links(1<<0 | 1<<1)})
	d := New(brd, reliefFor(brd), skyOf(sky.DefaultSun), false, st)
	img, _, px, _ := d.Surface(sheetAtlas{render.NewImage(24, 8)})
	if img == nil || px != 16 || img.Bounds().Dx() != 64 || img.Bounds().Dy() != 64 {
		t.Fatalf("the albedo is %v pixels a cell, want 16 on a 64 by 64 sheet", px)
	}
	bases, blends, ways := 0, 0, 0
	seaUnderCoast := false
	d.canvas.Each(func(_ render.Tier, _ float32, v []render.Vertex) {
		switch {
		case v[0].Custom3 > 50000:
			blends++
		case v[0].ColorA == 1 && v[3].DstX-v[0].DstX == 16 && v[3].DstY-v[0].DstY == 16 && float32(int(v[0].DstX))/16 == v[0].DstX/16:
			bases++
			if v[0].DstX == 16 && v[0].DstY == 0 && v[0].SrcX >= 16 { // the coast east of the sea, in the sea's sprite
				seaUnderCoast = true
			}
		default:
			ways++
		}
	})
	if bases != 16 || !seaUnderCoast || blends == 0 || ways == 0 {
		t.Errorf("painted %d bases (the sea under the coast: %v), %d grounds running in and %d pieces of the way; want 16 bases, the sea under, some of each", bases, seaUnderCoast, blends, ways)
	}
	seen := d.albedo.seen
	d.Surface(sheetAtlas{render.NewImage(24, 8)})
	if d.albedo.seen != seen {
		t.Error("the albedo was painted again with the board as it was")
	}
	brd.Set(at(3, 3), sea)
	d.Surface(sheetAtlas{render.NewImage(24, 8)})
	if d.albedo.seen == seen || d.canvas.Len() == 0 || d.canvas.Len() > 9*16 {
		t.Errorf("after a cell changed the albedo painted %d pieces at version %d (was %d); want the changed cells alone, anew", d.canvas.Len(), d.albedo.seen, seen)
	}
}

// Beside the albedo the water is painted in its layers: the sea's shine over the sea and under the
// coast, the coast's grounds covering it, a river's flow down its slope and its shine where it runs.
func TestPainter_PaintsTheWaterBesideTheAlbedo(t *testing.T) {
	st := map[board.Name]Style{}
	grid := board.DefaultGrids{}.Square(4, 4, 32)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	land := styled(st, board.CellKind{Name: board.Named("k47"), Allows: board.Land, SpriteID: 1}, Style{Spread: 0.3})
	sea := styled(st, board.CellKind{Name: board.Named("k48"), Allows: board.Water, SpriteID: 2}, Style{Under: true, Shine: 0.9})
	river := styled(st, board.CellKind{Name: board.Named("k49"), Allows: board.Land | board.Water, SpriteID: 0}, Style{Shine: 0.9, Flow: 30})
	brd.SetAll(land)
	at := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	brd.Set(at(0, 0), sea)
	for y := uint32(1); y < 4; y++ {
		brd.SetWay(at(2, y), board.Way{Kind: river, Width: 8, Links: board.Links(1<<0 | 1<<1)})
	}
	reliefFor(brd).SetHeights(func(p geom.Vec) float64 { return 40 - p.Y/4 }) // falling to the south
	d := New(brd, reliefFor(brd), skyOf(sky.DefaultSun), true, st)
	_, sheet, _, wpx := d.Surface(sheetAtlas{render.NewImage(24, 8)})
	if sheet == nil || wpx != 16 || sheet.Bounds().Dx() != 128 || sheet.Bounds().Dy() != 128 {
		t.Fatalf("the water is %v pixels a cell, want 16 in quadrants of the board's 64 by 64", wpx)
	}
	sea00, seaCoast, covered, riverShine := false, false, 0, 0
	d.albedo.wcanvas[water.ShineLayer].Each(func(_ render.Tier, _ float32, v []render.Vertex) {
		switch {
		case v[0].Custom3 > 50000 && v[0].ColorR == 0 && v[0].ColorG == 0 && v[0].ColorB == 0:
			covered++
		case v[0].ColorB == 1 && near(v[0].ColorG, 0.9) && v[0].DstX == 64 && v[0].DstY == 0:
			sea00 = true
		case v[0].ColorB == 1 && near(v[0].ColorG, 0.9) && v[0].DstX == 80 && v[0].DstY == 0:
			seaCoast = true
		case near(v[0].ColorR, 0.9) && v[0].ColorG == 0:
			riverShine++
		}
	})
	flows := 0
	d.albedo.wcanvas[water.FlowLayer].Each(func(_ render.Tier, _ float32, v []render.Vertex) {
		if v[0].ColorB == 1 && v[0].ColorG > 0.5 && near(v[0].ColorR, 0.5) {
			flows++ // running south, down the slope
		}
	})
	if !sea00 || !seaCoast || covered == 0 || riverShine == 0 || flows == 0 {
		t.Errorf("the sea's shine over the sea %v and under the coast %v, %d grounds covering it, %d pieces of the river shining and %d running south; want all", sea00, seaCoast, covered, riverShine, flows)
	}
}

// Water may lie on the sea, on the cells a river runs across and on every cell beside either; the
// rest is dry, and a cell turned to water wets it and the cells round it.
func TestPainter_TheWetCellsAreTheWaterAndTheCellsBesideIt(t *testing.T) {
	st := map[board.Name]Style{}
	grid := board.DefaultGrids{}.Square(8, 6, 32)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	land := styled(st, board.CellKind{Name: board.Named("k60"), Allows: board.Land, SpriteID: 1}, Style{})
	sea := styled(st, board.CellKind{Name: board.Named("k61"), Allows: board.Water, SpriteID: 2}, Style{Under: true, Shine: 0.9})
	river := styled(st, board.CellKind{Name: board.Named("k62"), Allows: board.Land | board.Water, SpriteID: 0}, Style{Shine: 0.9, Flow: 30})
	brd.SetAll(land)
	at := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	brd.Set(at(0, 0), sea)
	for y := uint32(2); y < 5; y++ {
		brd.SetWay(at(6, y), board.Way{Kind: river, Width: 8, Links: board.Links(1<<0 | 1<<1)})
	}
	d := New(brd, reliefFor(brd), skyOf(sky.DefaultSun), true, st)
	d.Surface(sheetAtlas{render.NewImage(24, 8)})
	wet, v := d.Wet()
	if len(wet) != 8*6 || v == 0 {
		t.Fatalf("%d cells' flags at %d, want one a cell of 8 by 6", len(wet), v)
	}
	for _, c := range []struct {
		x, y int
		wet  bool
	}{{0, 0, true}, {1, 1, true}, {2, 2, false}, {6, 3, true}, {5, 3, true}, {7, 5, true}, {3, 4, false}} {
		if got := wet[c.y*8+c.x]; got != c.wet {
			t.Errorf("cell (%d, %d) wet %v, want %v", c.x, c.y, got, c.wet)
		}
	}
	brd.Set(at(3, 4), sea)
	d.Surface(sheetAtlas{render.NewImage(24, 8)})
	wet, w := d.Wet()
	if w == v || !wet[4*8+3] || !wet[3*8+2] {
		t.Errorf("after the sea came to (3, 4): flags at %d, it %v, its neighbour (2, 3) %v; want both wet anew", w, wet[4*8+3], wet[3*8+2])
	}
}

// The coast is the way to the shore from every corner, as shoreAt works it out; the ground raised
// leaves it be, a cell turned to water moves it.
func TestPainter_TheCoastFollowsTheShineNotTheRelief(t *testing.T) {
	st := map[board.Name]Style{}
	grid := board.DefaultGrids{}.Square(10, 8, 32)
	brd := board.NewBoard(grid, board.NewTerrainMap())
	land := styled(st, board.CellKind{Name: board.Named("k50"), Allows: board.Land, SpriteID: 1}, Style{})
	sea := styled(st, board.CellKind{Name: board.Named("k51"), Allows: board.Water, SpriteID: 2}, Style{Under: true, Shine: 0.9})
	brd.SetAll(sea)
	at := func(x, y uint32) board.CellID { c, _ := grid.CellIndex(x, y); return c }
	brd.Set(at(4, 4), land)
	d := New(brd, reliefFor(brd), skyOf(sky.DefaultSun), true, st)
	check := func(when string) uint64 {
		shores, reach, v := d.Coast()
		if len(shores) != 11*9 || reach != 96 {
			t.Fatalf("%s: %d shores reaching %v, want one a corner of 11 by 9, 3 cells", when, len(shores), reach)
		}
		for gy := range 9 {
			for gx := range 11 {
				if want := water.Shore(d.workShore(int64(gx), int64(gy), 32)); shores[gy*11+gx] != want {
					t.Fatalf("%s: corner (%d, %d) has %+v, want %+v", when, gx, gy, shores[gy*11+gx], want)
				}
			}
		}
		return v
	}
	v := check("at first")
	reliefFor(brd).SetHeights(func(p geom.Vec) float64 { return p.X / 8 })
	if check("the ground raised") != v {
		t.Error("the ground raised moved the coast")
	}
	brd.Set(at(8, 1), land)
	if check("a cell turned to land") == v {
		t.Error("a cell turned to land left the coast as it was")
	}
}

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }
