package painter

import (
	"image/color"
	"math"
	"strings"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/gram/internal/gpu/gputest"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/topography/internal/relief"
	"github.com/kjkrol/gram/plugins/topography/internal/water"
	"github.com/kjkrol/gram/render"
)

type testMap struct {
	look look.Look
	d    *Painter
}

func (m testMap) Look() look.Look         { return m.look }
func (m testMap) Dressing() look.Dressing { return m.d }
func (m testMap) Heights() ground.Heights { return m.d.relief }
func (m testMap) Top(c cell.ID) (corners [4]float32, level float32) {
	t := m.d.topOf(c)
	return t.z, t.alt
}
func (testMap) Climb(cell.ID, cell.ID, cell.Domain) float64   { return 1 }
func (testMap) Least(cell.Domain) float64                     { return 1 }
func (testMap) Slope(geom.Vec, geom.Vec, cell.Domain) float64 { return 1 }

// one a board, so a board's heights and its Painter meet on the same relief
var (
	reliefs = map[*board.BoardRead]*relief.Relief{}
	writes  = map[*board.BoardRead]*board.BoardWrite{}
)

// a board of g, its relief counting its changes on it
func newBoard(g grid.Grid) (*board.BoardRead, *board.BoardWrite) {
	brd, w := board.NewBoard(g)
	writes[brd] = w
	return brd, w
}

func reliefFor(brd *board.BoardRead) *relief.Relief {
	r, ok := reliefs[brd]
	if !ok {
		r = relief.New(brd, writes[brd].Touch)
		reliefs[brd] = r
	}
	return r
}

// solids is an atlas of a sprite of each colour
func solids(colours map[render.SpriteID]color.RGBA) render.AtlasSource {
	atlas := render.NewAtlas()
	for id, c := range colours {
		atlas.Add(id, 16, render.Solid(c))
	}
	atlas.Close()
	return atlas
}

// baked is the board's Surface as the GPU bakes it, read at points of the world
type baked struct {
	s            *groundSheet
	cell         float32
	albedo, wpix []byte
}

// bakedOf bakes d's board out of atlas, cells cell wide
func bakedOf(t *testing.T, d *Painter, atlas render.AtlasSource, cell float32) *baked {
	t.Helper()
	img, wtr, _, _ := d.Surface(atlas)
	if img == nil {
		t.Fatal("no albedo baked")
	}
	b := &baked{s: d.albedo, cell: cell}
	b.albedo = make([]byte, 4*img.Bounds().Dx()*img.Bounds().Dy())
	img.ReadPixels(b.albedo)
	if wtr != nil {
		b.wpix = make([]byte, 4*wtr.Bounds().Dx()*wtr.Bounds().Dy())
		wtr.ReadPixels(b.wpix)
	}
	return b
}

// at is the pixel of pix, w wide, at the world point x, y, px pixels a cell from origin
func (b *baked) at(pix []byte, w, px int, ox, oy int, x, y float32) color.RGBA {
	k := float32(px) / b.cell
	i := 4 * ((oy+int(y*k))*w + ox + int(x*k))
	return color.RGBA{pix[i], pix[i+1], pix[i+2], pix[i+3]}
}

// albedoAt is the albedo at the world point x, y
func (b *baked) albedoAt(x, y float32) color.RGBA {
	return b.at(b.albedo, b.s.img.Bounds().Dx(), b.s.px, 0, 0, x, y)
}

// waterAt is the water's layer q at the world point x, y
func (b *baked) waterAt(q int, x, y float32) color.RGBA {
	o := b.s.waterLayer(q).Min
	return b.at(b.wpix, b.s.water.Bounds().Dx(), b.s.wpx, o.X, o.Y, x, y)
}

// close reports whether a and b differ by at most d in every colour
func close(a, b color.RGBA, d int) bool {
	for _, p := range [][2]uint8{{a.R, b.R}, {a.G, b.G}, {a.B, b.B}} {
		if k := int(p[0]) - int(p[1]); k > d || k < -d {
			return false
		}
	}
	return true
}

// between reports whether c lies between a and b, near neither
func between(c, a, b color.RGBA) bool { return !close(c, a, 8) && !close(c, b, 8) }

var (
	green = color.RGBA{90, 160, 70, 255}
	blue  = color.RGBA{40, 80, 170, 255}
	brown = color.RGBA{150, 110, 70, 255}
	sandy = color.RGBA{210, 190, 120, 255}
)

func styled(st map[cell.Name]Style, k cell.Kind, s Style) cell.Kind {
	st[k.Name] = s
	return k
}

func TestBake_RunningWaterRunsDownItsSlopeAndNotIntoItsBanks(t *testing.T) {
	gputest.Need(t)
	st := map[cell.Name]Style{}
	g := grid.DefaultGrids{}.Square(3, 3, 32)
	brd, brdW := newBoard(g)
	brdW.SetAll(cell.Kind{Cost: 1, Allows: cell.Land, SpriteID: 1})
	for y := range uint32(3) {
		brdW.Set(g.At(1, y), styled(st, cell.Kind{Name: cell.Named("k1"), Allows: cell.Water,
			SpriteID: 2}, Style{Shine: 1, Flow: 10}))
	}
	brdW.Set(g.At(0, 1), styled(st, cell.Kind{Name: cell.Named("k2"), Allows: cell.Water,
		SpriteID: 2}, Style{Shine: 1}))
	// falling 0.4 southward, the banks rising half as fast away from the stream
	reliefFor(brd).SetHeights(func(p geom.Vec) float64 { return 40 - 0.4*p.Y + 0.5*math.Abs(p.X-48) })
	d := New(brd, reliefFor(brd), true, st)
	b := bakedOf(t, d, solids(map[render.SpriteID]color.RGBA{1: green, 2: blue}), 32)
	south := 0.5 + 10*math.Sqrt(0.4)/(2*water.FlowSpan)
	if f := b.waterAt(water.FlowLayer, 49, 49); !close(f, color.RGBA{128,
		uint8(south*255 + 0.5), 255, 255}, 2) {
		t.Errorf("the stream flows %v, want south, down the valley, not into a bank", f)
	}
	if f, s := b.waterAt(water.FlowLayer, 17, 49), b.waterAt(water.ShineLayer, 17, 49); f.A != 0 ||
		!close(s, color.RGBA{0, 255, 255, 255}, 1) {
		t.Errorf("still water flows %v and shines %v, want no flow and still water's shine", f, s)
	}
	if s := b.waterAt(water.ShineLayer, 81, 49); s.A != 0 {
		t.Errorf("a bank shines %v, want no water", s)
	}
}

func TestBake_AWayStraightAcrossIsOneBandFromSideToSide(t *testing.T) {
	gputest.Need(t)
	st := map[cell.Name]Style{}
	g := grid.DefaultGrids{}.Square(3, 3, 32)
	brd, brdW := newBoard(g)
	brdW.SetAll(cell.Kind{Cost: 1, Allows: cell.Land, SpriteID: 1})
	reliefFor(brd).SetHeights(func(p geom.Vec) float64 { return 40 - 0.5*p.X }) // falling east
	stream := styled(st, cell.Kind{Name: cell.Named("k3"), Allows: cell.Land | cell.Water,
		SpriteID: 2}, Style{Shine: 1, Flow: 10})
	west, east := cell.Links(1<<2), cell.Links(1<<3)
	brdW.SetWay(g.At(1, 1), cell.Way{Kind: stream, Width: 8, Links: west | east})
	brdW.SetWay(g.At(0, 1), cell.Way{Kind: stream, Width: 16, Links: east})
	d := New(brd, reliefFor(brd), true, st)
	b := bakedOf(t, d, solids(map[render.SpriteID]color.RGBA{1: green, 2: blue}), 32)
	// 12 wide on the west side, the mean of 8 and 16; 8 on the east
	for _, c := range []struct {
		x, y float32
		want color.RGBA
	}{{41, 51, blue}, {49, 55, green}, {63, 51, blue}, {63, 53, green}, {81, 81, green}} {
		if got := b.albedoAt(c.x, c.y); !close(got, c.want, 2) {
			t.Errorf("at %v, %v the albedo is %v, want %v", c.x, c.y, got, c.want)
		}
	}
	run := 0.5 + 10*math.Sqrt(0.5)/(2*water.FlowSpan)
	if f := b.waterAt(water.FlowLayer, 49, 49); !close(f, color.RGBA{uint8(run*255 + 0.5), 128,
		255, 255}, 2) {
		t.Errorf("the way's water flows %v, want east, down the slope", f)
	}
}

func TestBake_AWayTurningCurvesRoundTheMiddle(t *testing.T) {
	gputest.Need(t)
	g := grid.DefaultGrids{}.Square(3, 3, 32)
	brd, brdW := newBoard(g)
	brdW.SetAll(cell.Kind{Cost: 1, Allows: cell.Land, SpriteID: 1})
	brdW.SetWay(g.At(1, 1), cell.Way{Kind: cell.Kind{Allows: cell.Land, SpriteID: 2}, Width: 8,
		Links: 1<<2 | 1<<1}) // west and south
	d := New(brd, reliefFor(brd), false, map[cell.Name]Style{})
	b := bakedOf(t, d, solids(map[render.SpriteID]color.RGBA{1: green, 2: brown}), 32)
	for _, c := range []struct {
		x, y float32
		want color.RGBA
		what string
	}{{33, 49, brown, "the west side's middle"}, {49, 63, brown, "the south side's middle"},
		{43, 53, brown, "halfway, round the middle"}, {35, 61, green, "the corner inside the turn"},
		{61, 35, green, "the corner across from it"}} {
		if got := b.albedoAt(c.x, c.y); !close(got, c.want, 2) {
			t.Errorf("%s is %v, want %v", c.what, got, c.want)
		}
	}
}

func TestBake_AWayFadingOutShowsLessAndRunsOnTheWayItFades(t *testing.T) {
	gputest.Need(t)
	st := map[cell.Name]Style{}
	g := grid.DefaultGrids{}.Square(4, 1, 32)
	brd, brdW := newBoard(g)
	brdW.SetAll(cell.Kind{Cost: 1, Allows: cell.Water, SpriteID: 1})
	river := styled(st, cell.Kind{Name: cell.Named("k4"), Allows: cell.Water, SpriteID: 2},
		Style{Shine: 1, Flow: 40})
	west, east := cell.Links(1<<2), cell.Links(1<<3)
	brdW.SetWay(g.At(0, 0), cell.Way{Kind: river, Width: 8, Links: east})
	brdW.SetWay(g.At(1, 0), cell.Way{Kind: river, Width: 8, Links: west | east, Fade: 0.5})
	brdW.SetWay(g.At(2, 0), cell.Way{Kind: river, Width: 8, Links: west, Fade: 0.75})
	d := New(brd, reliefFor(brd), true, st)
	b := bakedOf(t, d, solids(map[render.SpriteID]color.RGBA{1: green, 2: blue}), 32)
	if got := b.albedoAt(17, 17); !close(got, blue, 2) {
		t.Errorf("the way not fading is %v, want its own %v", got, blue)
	}
	if got := b.albedoAt(49, 17); !between(got, blue, green) {
		t.Errorf("the way faded by half is %v, want it between %v and %v", got, blue, green)
	}
	if got := b.albedoAt(85, 17); !close(got, green, 2) {
		t.Errorf("past the way's faded end it is %v, want the ground's %v", got, green)
	}
	// level water runs on the way the way fades: east
	f := b.waterAt(water.FlowLayer, 49, 17)
	if run := float64(f.R) / float64(f.B); f.B == 0 ||
		math.Abs(run-(0.5+40*0.1/(2*water.FlowSpan))) > 0.02 {
		t.Errorf("the fading way's water flows %v, want east at %v", f, 40*0.1)
	}
}

func TestBake_AWayRunsSlantwiseToTheCornerAndEndsSquareAcrossItself(t *testing.T) {
	gputest.Need(t)
	g := grid.DefaultGrids{}.Square(3, 3, 32)
	brd, brdW := newBoard(g)
	brdW.SetAll(cell.Kind{Cost: 1, Allows: cell.Land, SpriteID: 1})
	brdW.SetWay(g.At(1, 1), cell.Way{Kind: cell.Kind{Allows: cell.Land, SpriteID: 2}, Width: 4,
		Links: 1 << 7}) // south-east
	d := New(brd, reliefFor(brd), false, map[cell.Name]Style{})
	b := bakedOf(t, d, solids(map[render.SpriteID]color.RGBA{1: green, 2: brown}), 32)
	for _, c := range []struct {
		x, y float32
		want color.RGBA
		what string
	}{{57, 57, brown, "the band to the corner"}, {47, 47, brown, "its end behind the middle"},
		{43, 43, green, "past its end"}, {61, 51, green, "beside the band"}} {
		if got := b.albedoAt(c.x, c.y); !close(got, c.want, 2) {
			t.Errorf("%s is %v, want %v", c.what, got, c.want)
		}
	}
}

func TestBake_BlendsWeighTheNeighboursGroundsAtTheTilesPoints(t *testing.T) {
	gputest.Need(t)
	st := map[cell.Name]Style{}
	g := grid.DefaultGrids{}.Square(3, 3, 32)
	brd, brdW := newBoard(g)
	earth := styled(st, cell.Kind{Name: cell.Named("k5"), Allows: cell.Land, SpriteID: 1},
		Style{Spread: 0.2})
	sand := styled(st, cell.Kind{Name: cell.Named("k6"), Allows: cell.Land, SpriteID: 2},
		Style{Spread: 0.4})
	brdW.SetAll(earth)
	brdW.Set(g.At(2, 1), sand)
	atlas := solids(map[render.SpriteID]color.RGBA{1: green, 2: sandy, 3: blue})
	d := New(brd, reliefFor(brd), true, st)
	b := bakedOf(t, d, atlas, 32)
	if got := b.albedoAt(63, 47); !between(got, green, sandy) {
		t.Errorf("by the sand the earth is %v, want the sand running in", got)
	}
	if got := b.albedoAt(41, 41); !close(got, green, 2) {
		t.Errorf("away from the sand the earth is %v, want %v", got, green)
	}
	brdW.Set(g.At(2, 1), cell.Kind{Allows: cell.Water, SpriteID: 3}) // the sea keeps its cells square
	b = bakedOf(t, d, atlas, 32)
	if got := b.albedoAt(63, 47); !close(got, green, 2) {
		t.Errorf("by the sea the earth is %v, want its own %v", got, green)
	}
}

func TestBake_ByTheSeaTheLandIsLaidOverTheSeaUnderIt(t *testing.T) {
	gputest.Need(t)
	st := map[cell.Name]Style{}
	g := grid.DefaultGrids{}.Square(3, 3, 32)
	brd, brdW := newBoard(g)
	earth := styled(st, cell.Kind{Name: cell.Named("k7"), Allows: cell.Land, SpriteID: 1},
		Style{Spread: 0.25})
	sea := styled(st, cell.Kind{Name: cell.Named("k8"), Allows: cell.Water, SpriteID: 4},
		Style{Under: true})
	brdW.SetAll(earth)
	brdW.Set(g.At(0, 1), sea)
	d := New(brd, reliefFor(brd), false, st)
	b := bakedOf(t, d, solids(map[render.SpriteID]color.RGBA{1: green, 4: blue}), 32)
	if d.Base(g.At(1, 1), 1) != sea.SpriteID || d.Base(g.At(2, 1), 1) != earth.SpriteID {
		t.Errorf("bases %v by the sea, %v away from it; want the sea and the earth",
			d.Base(g.At(1, 1), 1), d.Base(g.At(2, 1), 1))
	}
	for _, c := range []struct {
		x, y float32
		want color.RGBA
		what string
	}{{49, 49, green, "the land's middle"}, {17, 49, blue, "the sea's middle"}} {
		if got := b.albedoAt(c.x, c.y); !close(got, c.want, 2) {
			t.Errorf("%s is %v, want %v", c.what, got, c.want)
		}
	}
	if got := b.albedoAt(33, 49); !between(got, green, blue) {
		t.Errorf("the land's side by the sea is %v, want it running over the sea", got)
	}
}

func TestWater_TheShaderCompilesWithTheBoardsMaterials(t *testing.T) {
	gputest.Need(t)
	if err := render.Compile(); err != nil {
		t.Fatal(err)
	}
	if src := string(render.ShaderSource()); !strings.Contains(src, "return SeaGlint(") || !strings.Contains(src, "return CloudShadow(") {
		t.Error("the shader hands no overlay to the sea's or the clouds' material")
	}
}

// off a square grid a still water's top glints, as the hexes' tops lay it; a square grid's the
// terrain paints
func TestPainter_OverlaysTheStillWaterGlintingOnAHexBoard(t *testing.T) {
	st := map[cell.Name]Style{}
	sea := styled(st, cell.Kind{Name: cell.Named("k12"), Allows: cell.Water, SpriteID: 1},
		Style{Shine: 0.8})
	land := cell.Kind{Name: cell.Named("k14"), Allows: cell.Land, SpriteID: 2}
	for _, g := range []grid.Grid{grid.DefaultGrids{}.Hex(3, 3, 16),
		grid.DefaultGrids{}.Square(3, 3, 32)} {
		brd, brdW := newBoard(g)
		brdW.SetAll(sea)
		brdW.Set(g.At(0, 0), land)
		d := New(brd, reliefFor(brd), true, st)
		d.Above()
		m, red, ok := d.Overlay(g.At(1, 1))
		shape, _ := brd.Shape()
		if shape.Hex && (!ok || m != water.SeaGlint() || red < 0.79 || red > 0.81) {
			t.Errorf("a hex of still sea is overlaid %v, %v, %v; want the sea's glint at 0.8", m,
				red, ok)
		}
		if !shape.Hex && ok {
			t.Errorf("a square of sea is overlaid %v; want none, the terrain painting it", m)
		}
		if _, _, ok := d.Overlay(g.At(0, 0)); ok {
			t.Error("a cell of land is overlaid, want none")
		}
	}
}

func TestTile_AShinyCellShines(t *testing.T) {
	st := map[cell.Name]Style{}
	g := grid.DefaultGrids{}.Square(4, 4, 32)
	brd, brdW := newBoard(g)
	brdW.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	sea := g.At(1, 1)
	brdW.Set(sea, styled(st, cell.Kind{Name: cell.Named("k14"), Cost: 1, Allows: cell.Water},
		Style{Shine: 0.8}))
	for _, heights := range []bool{true, false} {
		d := New(brd, reliefFor(brd), heights, st)
		d.Above()
		s, ok := d.cellTile(sea).Shine()
		if ok != heights || heights && math.Abs(float64(s-0.8)) > 1e-6 {
			t.Errorf("with heights %v the sea shines %v, %v; want its kind's 0.8 in relief alone",
				heights, s, ok)
		}
		if _, ok := d.cellTile(g.At(3, 3)).Shine(); ok {
			t.Error("land shines, want nothing")
		}
	}
}

func TestShores_LieTheWayOfTheNearestCellThatDoesNotShine(t *testing.T) {
	gputest.Need(t)
	st := map[cell.Name]Style{}
	g := grid.DefaultGrids{}.Square(10, 4, 32)
	brd, brdW := newBoard(g)
	brdW.SetAll(styled(st, cell.Kind{Name: cell.Named("k15"), Cost: 1, Allows: cell.Water},
		Style{Shine: 1}))
	for y := range uint32(4) {
		brdW.Set(g.At(0, y), cell.Kind{Cost: 1, Allows: cell.Land}) // a coast along x = 32
	}
	d := New(brd, reliefFor(brd), true, st)
	img, reach, _ := d.Shores(nil)
	if reach != 96 || img.Bounds().Dx() != 11 || img.Bounds().Dy() != 5 {
		t.Fatalf("the shores are %v reaching %v, want 11 by 5 corners reaching 3 cells", img.Bounds(),
			reach)
	}
	pix := make([]byte, 4*11*5)
	img.ReadPixels(pix)
	at := func(x, y int) color.RGBA {
		i := 4 * (y*11 + x)
		return color.RGBA{pix[i], pix[i+1], pix[i+2], pix[i+3]}
	}
	for _, c := range []struct {
		x, y int
		want color.RGBA
		what string
	}{{1, 1, color.RGBA{0, 128, 0, 255}, "on the coast, the land right there to the west"},
		{3, 1, color.RGBA{0, 128, 170, 255}, "two cells out, the land 64 to the west"},
		{6, 1, color.RGBA{128, 128, 255, 255}, "out at sea, open water"}} {
		if got := at(c.x, c.y); !close(got, c.want, 1) {
			t.Errorf("%s: the corner holds %v, want %v", c.what, got, c.want)
		}
	}
}

// 8 pixels a sprite
type sheetAtlas struct{ img *render.Image }

func (a sheetAtlas) Atlas() *render.Image { return a.img }
func (sheetAtlas) UV(id render.SpriteID) (sx0, sy0, sx1, sy1 float32) {
	return float32(id) * 8, 0, float32(id)*8 + 8, 8
}
func (sheetAtlas) White() (u, v float32) { return 1, 1 }

type kindsOf []cell.Kind

func (k kindsOf) Define(string, cell.Kind, ...*ceremony.Part) {}
func (k kindsOf) Named(name string) cell.Of {
	c, _ := k.Get(name)
	return cell.OfKind(c)
}
func (k kindsOf) Draw(string, render.SpriteDrawer) {}
func (k kindsOf) All() []cell.Kind                 { return k }
func (k kindsOf) Get(name string) (cell.Kind, bool) {
	for _, c := range k {
		if c.Name == cell.Named(name) {
			return c, true
		}
	}
	return cell.Kind{}, false
}

func TestBake_AWayTurnsIntoTheKindItMixesWith(t *testing.T) {
	gputest.Need(t)
	st := map[cell.Name]Style{}
	g := grid.DefaultGrids{}.Square(3, 3, 32)
	brd, brdW := newBoard(g)
	brdW.SetAll(cell.Kind{Cost: 1, Allows: cell.Land, SpriteID: 1})
	sea := cell.Kind{Name: cell.Named("k44"), Allows: cell.Water, SpriteID: 7}
	stream := styled(st, cell.Kind{Name: cell.Named("k45"), Allows: cell.Land | cell.Water,
		SpriteID: 3}, Style{MixWith: "k44"})
	west, east := cell.Links(1<<2), cell.Links(1<<3)
	brdW.SetWay(g.At(0, 1), cell.Way{Kind: stream, Width: 8, Links: east, Mix: 0})
	brdW.SetWay(g.At(1, 1), cell.Way{Kind: stream, Width: 8, Links: west | east, Mix: 0.5})
	brdW.SetWay(g.At(2, 1), cell.Way{Kind: stream, Width: 8, Links: west, Mix: 1})
	d := New(brd, reliefFor(brd), false, st)
	d.kinds = kindsOf{sea, stream}
	b := bakedOf(t, d, solids(map[render.SpriteID]color.RGBA{1: green, 3: brown, 7: blue}), 32)
	half := color.RGBA{(brown.R + blue.R) / 2, (brown.G + blue.G) / 2, (brown.B + blue.B) / 2, 255}
	for _, c := range []struct {
		x, y float32
		want color.RGBA
		what string
	}{{17, 49, brown, "where it has not turned"}, {49, 49, half, "halfway turned"},
		{81, 49, blue, "turned all the way"}} {
		if got := b.albedoAt(c.x, c.y); !close(got, c.want, 3) {
			t.Errorf("the way %s is %v, want %v", c.what, got, c.want)
		}
	}
}

func TestBake_AWayRunsOnUnderTheWaterItRunsInto(t *testing.T) {
	gputest.Need(t)
	st := map[cell.Name]Style{}
	g := grid.DefaultGrids{}.Square(3, 3, 32)
	brd, brdW := newBoard(g)
	brdW.SetAll(styled(st, cell.Kind{Name: cell.Named("k49"), Allows: cell.Land, SpriteID: 1},
		Style{Spread: 0.3}))
	sea := styled(st, cell.Kind{Name: cell.Named("k50"), Allows: cell.Water, SpriteID: 4},
		Style{Under: true})
	stream := styled(st, cell.Kind{Name: cell.Named("k51"), Allows: cell.Land | cell.Water,
		SpriteID: 5}, Style{})
	for x := range uint32(3) {
		brdW.Set(g.At(x, 2), sea)
	}
	north, south := cell.Links(1<<0), cell.Links(1<<1)
	brdW.SetWay(g.At(1, 0), cell.Way{Kind: stream, Width: 8, Links: south})
	brdW.SetWay(g.At(1, 1), cell.Way{Kind: stream, Width: 8, Links: north | south})
	d := New(brd, reliefFor(brd), false, st)
	b := bakedOf(t, d, solids(map[render.SpriteID]color.RGBA{1: green, 4: blue, 5: brown}), 32)
	if got := b.albedoAt(49, 49); !close(got, brown, 2) {
		t.Errorf("the way well ashore is %v, want %v", got, brown)
	}
	if got := b.albedoAt(49, 69); !between(got, brown, blue) {
		t.Errorf("the way running into the water is %v, want it fading under it", got)
	}
	if got := b.albedoAt(49, 81); !close(got, blue, 2) {
		t.Errorf("at the water's middle it is %v, want the water alone", got)
	}
}

func TestPainter_TheAlbedoIsPaintedAnewOnlyAsTheBoardChanges(t *testing.T) {
	gputest.Need(t)
	st := map[cell.Name]Style{}
	g := grid.DefaultGrids{}.Square(4, 4, 32)
	brd, brdW := newBoard(g)
	land := styled(st, cell.Kind{Name: cell.Named("k44"), Allows: cell.Land, SpriteID: 1},
		Style{Spread: 0.3})
	sea := styled(st, cell.Kind{Name: cell.Named("k45"), Allows: cell.Water, SpriteID: 2},
		Style{Under: true})
	road := styled(st, cell.Kind{Name: cell.Named("k46"), Allows: cell.Land, SpriteID: 3},
		Style{})
	brdW.SetAll(land)
	brdW.Set(g.At(0, 0), sea)
	brdW.SetWay(g.At(2, 2), cell.Way{Kind: road, Width: 8, Links: cell.Links(1<<0 | 1<<1)})
	atlas := solids(map[render.SpriteID]color.RGBA{1: green, 2: blue, 3: brown})
	d := New(brd, reliefFor(brd), false, st)
	img, _, px, _ := d.Surface(atlas)
	if img == nil || px != 16 || img.Bounds().Dx() != 64 || img.Bounds().Dy() != 64 {
		t.Fatalf("the albedo is %v pixels a cell, want 16 on a 64 by 64 sheet", px)
	}
	b := bakedOf(t, d, atlas, 32)
	for _, c := range []struct {
		x, y float32
		want color.RGBA
	}{{17, 17, blue}, {49, 17, green}, {81, 81, brown}, {113, 113, green}} {
		if got := b.albedoAt(c.x, c.y); !close(got, c.want, 2) {
			t.Errorf("at %v, %v the albedo is %v, want %v", c.x, c.y, got, c.want)
		}
	}
	seen := d.albedo.seen
	d.Surface(atlas)
	if d.albedo.seen != seen {
		t.Error("the albedo was painted again with the board as it was")
	}
	brdW.Set(g.At(3, 3), sea)
	b = bakedOf(t, d, atlas, 32)
	if d.albedo.seen == seen || !close(b.albedoAt(113, 113), blue, 2) {
		t.Errorf("after a cell turned to sea the albedo was painted at %d (was %d), the cell %v; "+
			"want it anew, the sea", d.albedo.seen, seen, b.albedoAt(113, 113))
	}
}

func TestPainter_PaintsTheWaterBesideTheAlbedo(t *testing.T) {
	gputest.Need(t)
	st := map[cell.Name]Style{}
	g := grid.DefaultGrids{}.Square(4, 4, 32)
	brd, brdW := newBoard(g)
	land := styled(st, cell.Kind{Name: cell.Named("k47"), Allows: cell.Land, SpriteID: 1},
		Style{Spread: 0.3})
	sea := styled(st, cell.Kind{Name: cell.Named("k48"), Allows: cell.Water, SpriteID: 2},
		Style{Under: true, Shine: 0.9})
	river := styled(st, cell.Kind{Name: cell.Named("k49"), Allows: cell.Land | cell.Water,
		SpriteID: 3}, Style{Shine: 0.9, Flow: 30})
	brdW.SetAll(land)
	brdW.Set(g.At(0, 0), sea)
	for y := uint32(1); y < 4; y++ {
		brdW.SetWay(g.At(2, y), cell.Way{Kind: river, Width: 8, Links: cell.Links(1<<0 | 1<<1)})
	}
	reliefFor(brd).SetHeights(func(p geom.Vec) float64 { return 40 - p.Y/4 }) // falling south
	d := New(brd, reliefFor(brd), true, st)
	_, sheet, _, wpx := d.Surface(solids(map[render.SpriteID]color.RGBA{1: green, 2: blue,
		3: brown}))
	if sheet == nil || wpx != 16 || sheet.Bounds().Dx() != 128 || sheet.Bounds().Dy() != 128 {
		t.Fatalf("the water is %v pixels a cell, want 16 in quadrants of the board's 64 by 64",
			wpx)
	}
	b := bakedOf(t, d, d.albedo.atlas, 32)
	if s := b.waterAt(water.ShineLayer, 17, 17); !close(s, color.RGBA{0, 230, 255, 255}, 1) {
		t.Errorf("the sea shines %v, want still water at 0.9", s)
	}
	if s := b.waterAt(water.ShineLayer, 81, 81); !close(s, color.RGBA{230, 0, 0, 255}, 1) {
		t.Errorf("the river shines %v, want running water at 0.9", s)
	}
	if f := b.waterAt(water.FlowLayer, 81, 81); !close(f, color.RGBA{128, f.G, 255, 255}, 1) ||
		f.G <= 130 {
		t.Errorf("the river flows %v, want south, down the slope", f)
	}
	if s := b.waterAt(water.ShineLayer, 49, 49); s.A != 0 && (s.R != 0 || s.G != 0) {
		t.Errorf("the land shines %v, want no water", s)
	}
}

func TestPainter_TheWetCellsAreTheWaterAndTheCellsBesideIt(t *testing.T) {
	gputest.Need(t)
	st := map[cell.Name]Style{}
	grid := grid.DefaultGrids{}.Square(8, 6, 32)
	brd, brdW := newBoard(grid)
	land := styled(st, cell.Kind{Name: cell.Named("k60"), Allows: cell.Land, SpriteID: 1}, Style{})
	sea := styled(st, cell.Kind{Name: cell.Named("k61"), Allows: cell.Water, SpriteID: 2}, Style{Under: true, Shine: 0.9})
	river := styled(st, cell.Kind{Name: cell.Named("k62"), Allows: cell.Land | cell.Water, SpriteID: 0}, Style{Shine: 0.9, Flow: 30})
	brdW.SetAll(land)
	at := func(x, y uint32) cell.ID { c := grid.At(x, y); return c }
	brdW.Set(at(0, 0), sea)
	for y := uint32(2); y < 5; y++ {
		brdW.SetWay(at(6, y), cell.Way{Kind: river, Width: 8, Links: cell.Links(1<<0 | 1<<1)})
	}
	d := New(brd, reliefFor(brd), true, st)
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
	brdW.Set(at(3, 4), sea)
	d.Surface(sheetAtlas{render.NewImage(24, 8)})
	wet, w := d.Wet()
	if w == v || !wet[4*8+3] || !wet[3*8+2] {
		t.Errorf("after the sea came to (3, 4): flags at %d, it %v, its neighbour (2, 3) %v; want both wet anew", w, wet[4*8+3], wet[3*8+2])
	}
}

func TestShores_FollowTheShineNotTheRelief(t *testing.T) {
	gputest.Need(t)
	st := map[cell.Name]Style{}
	g := grid.DefaultGrids{}.Square(10, 8, 32)
	brd, brdW := newBoard(g)
	land := styled(st, cell.Kind{Name: cell.Named("k50"), Allows: cell.Land, SpriteID: 1}, Style{})
	sea := styled(st, cell.Kind{Name: cell.Named("k51"), Allows: cell.Water, SpriteID: 2},
		Style{Under: true, Shine: 0.9})
	brdW.SetAll(sea)
	brdW.Set(g.At(4, 4), land)
	d := New(brd, reliefFor(brd), true, st)
	corner := func(x, y int) color.RGBA {
		img, _, _ := d.Shores(nil)
		pix := make([]byte, 4*11*9)
		img.ReadPixels(pix)
		i := 4 * (y*11 + x)
		return color.RGBA{pix[i], pix[i+1], pix[i+2], pix[i+3]}
	}
	if c := corner(4, 4); c.B != 0 {
		t.Errorf("the land's corner is %v off the shore, want on it", c)
	}
	if c := corner(0, 0); c.B != 255 {
		t.Errorf("a corner four cells from the land holds %v, want open water", c)
	}
	_, _, v := d.Shores(nil)
	reliefFor(brd).SetHeights(func(p geom.Vec) float64 { return p.X / 8 })
	if _, _, w := d.Shores(nil); w != v {
		t.Error("the ground raised moved the coast")
	}
	brdW.Set(g.At(1, 1), land)
	if _, _, w := d.Shores(nil); w == v || corner(0, 0).B == 255 {
		t.Errorf("a cell turned to land left the coast as it was: the corner by it %v",
			corner(0, 0))
	}
}

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }
