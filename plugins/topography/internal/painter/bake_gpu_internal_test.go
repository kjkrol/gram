package painter

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/gpu/gputest"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	drain "github.com/kjkrol/gram/plugins/board/water"
	"github.com/kjkrol/gram/render"
)

// a sprite of colour c grained across, so a texel read from the wrong place shows
func grained(c color.RGBA) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		for y := range size {
			for x := range size {
				v := uint8((x*5 + y*3) % 7 * 6)
				dst.SetRGBA(x, y, color.RGBA{c.R - v, c.G - v, c.B - v, 255})
			}
		}
	}
}

var sprite = grained

// a board of grass and sand round a sea, rocks standing on it, for the bake to paint
func bakedBoard(t *testing.T) (*Painter, *board.BoardWrite, render.AtlasSource, grid.Grid) {
	t.Helper()
	st := map[cell.Name]Style{}
	g := grid.DefaultGrids{}.Square(16, 12, 32)
	brd, brdW := newBoard(g)
	grain := image.NewRGBA(image.Rect(0, 0, 8, 8))
	grass := styled(st, cell.Kind{Name: cell.Named("b1"), Allows: cell.Land, SpriteID: 1},
		Style{Spread: 0.3, Detail: Detail{Image: grain}})
	sand := styled(st, cell.Kind{Name: cell.Named("b2"), Allows: cell.Land, SpriteID: 2},
		Style{Spread: 0.2, Detail: Detail{Image: grain, Across: 2}})
	brook := styled(st, cell.Kind{Name: cell.Named("b5"), Allows: cell.Water, SpriteID: 4},
		Style{Shine: 0.7, Flow: 30})
	rock := cell.Kind{Name: cell.Named("b3"), Allows: cell.Land, SpriteID: 3, Height: 10}
	sea := styled(st, cell.Kind{Name: cell.Named("b4"), Allows: cell.Water, SpriteID: 4},
		Style{Under: true, Shine: 0.8})
	brdW.SetAll(grass)
	for y := range uint32(12) {
		for x := range uint32(16) {
			dx, dy := int(x)-6, int(y)-4
			switch d := dx*dx + dy*dy; {
			case d <= 2:
				brdW.Set(g.At(x, y), sea)
			case d <= 6:
				brdW.Set(g.At(x, y), sand)
			}
		}
	}
	brdW.Set(g.At(1, 1), rock)
	brdW.Set(g.At(2, 6), sand)
	for y := uint32(5); y < 10; y++ {
		brdW.Set(g.At(12, y), brook) // running down the slope
	}
	reliefFor(brd).SetHeights(func(p geom.Vec) float64 { return 20 + p.X/16 - p.Y/8 })
	atlas := render.NewAtlas()
	for id, c := range map[render.SpriteID]color.RGBA{1: {90, 160, 70, 255},
		2: {210, 190, 120, 255}, 3: {120, 120, 130, 255}, 4: {40, 80, 170, 255}} {
		atlas.Add(id, 16, sprite(c))
	}
	atlas.Close()
	d := New(brd, reliefFor(brd), true, st)
	d.kinds = kindsOf{grass, sand, rock, sea, brook}
	return d, brdW, atlas, g
}

// differ counts the pixels of a and b apart by over 8 in a channel, saving both where
// GRAM_TILES_PNG names a folder
func differ(t *testing.T, a, b *render.Image) int {
	t.Helper()
	w, h := a.Bounds().Dx(), a.Bounds().Dy()
	pa, pb := make([]byte, 4*w*h), make([]byte, 4*w*h)
	a.ReadPixels(pa)
	b.ReadPixels(pb)
	if dir := os.Getenv("GRAM_TILES_PNG"); dir != "" {
		for name, pix := range map[string][]byte{"gpu": pa, "cpu": pb} {
			f, _ := os.Create(dir + "/" + t.Name() + "-" + name + ".png")
			_ = png.Encode(f, &image.RGBA{Pix: pix, Stride: 4 * w, Rect: image.Rect(0, 0, w, h)})
			f.Close()
		}
	}
	off := 0
	for i := 0; i < len(pa); i += 4 {
		for k := range 4 {
			if d := int(pa[i+k]) - int(pb[i+k]); d > 8 || d < -8 {
				off++
				break
			}
		}
	}
	return off
}

// bakedApart brings d's Surface up to the board out of atlas, paints the board whole into a
// sheet of its own, and counts the pixels of each layer apart: the albedo, the kinds' details,
// the water
func bakedApart(t *testing.T, d *Painter, atlas render.AtlasSource) [3]int {
	t.Helper()
	d.Surface(atlas)
	got, whole := d.albedo, d.newSheet(atlas)
	d.bake(whole, atlas, d.toPaint(true))
	var off [3]int
	for k, x := range [3][2]*render.Image{{got.img, whole.img}, {got.kinds, whole.kinds},
		{got.water, whole.water}} {
		t.Run([]string{"albedo", "kinds", "water"}[k], func(t *testing.T) {
			off[k] = differ(t, x[0], x[1])
			if !painted(x[1]) {
				t.Error("nothing painted into it, want a layer to compare")
			}
		})
	}
	return off
}

// painted reports whether img holds a pixel of any colour
func painted(img *render.Image) bool {
	pix := make([]byte, 4*img.Bounds().Dx()*img.Bounds().Dy())
	img.ReadPixels(pix)
	for i := 0; i < len(pix); i += 4 {
		if pix[i] != 0 || pix[i+1] != 0 || pix[i+2] != 0 {
			return true
		}
	}
	return false
}

func TestPainter_PaintsTheGroundAnewRoundACellAsWhole(t *testing.T) {
	gputest.Need(t)
	d, brdW, atlas, g := bakedBoard(t)
	if off := bakedApart(t, d, atlas); off != [3]int{} {
		t.Errorf("%v pixels of the albedo, the kinds and the water differ painted twice, want "+
			"none", off)
	}
	brdW.Set(g.At(8, 1), d.kinds.(kindsOf)[3])
	d.refresh(atlas)
	if at := d.toPaint(false); len(at) != 4*20 {
		t.Errorf("one cell by the edge changed, %d cells painted anew, want the 20 within two "+
			"on the board", len(at)/4)
	}
	if off := bakedApart(t, d, atlas); off != [3]int{} {
		t.Errorf("a cell changed, %v pixels of the albedo, the kinds and the water painted round "+
			"it differ from the board painted whole, want none", off)
	}
}

// a board of land round a sea, ways across it: a road straight, turning and forking, a track
// fading out, a river turning into the sea and running into it, a road along the shore
func wayBoard(t *testing.T, g grid.Grid) (*Painter, *board.BoardWrite, render.AtlasSource) {
	t.Helper()
	st := map[cell.Name]Style{}
	brd, brdW := newBoard(g)
	grain := image.NewRGBA(image.Rect(0, 0, 8, 8))
	grass := styled(st, cell.Kind{Name: cell.Named("w1"), Allows: cell.Land, SpriteID: 1},
		Style{Spread: 0.3, Detail: Detail{Image: grain}})
	sea := styled(st, cell.Kind{Name: cell.Named("w2"), Allows: cell.Water, SpriteID: 2},
		Style{Under: true, Shine: 0.8})
	road := styled(st, cell.Kind{Name: cell.Named("w3"), Allows: cell.Land, SpriteID: 3},
		Style{Detail: Detail{Image: grain, Across: 3}})
	river := styled(st, cell.Kind{Name: cell.Named("w4"), Allows: cell.Land | cell.Water,
		SpriteID: 4}, Style{Shine: 0.9, Flow: 22, MixWith: "w2"})
	brdW.SetAll(grass)
	shape, _ := brd.Shape()
	cols, rows := shape.Cols, shape.Rows
	for y := range rows {
		for x := range cols {
			if y >= rows-3 {
				brdW.Set(g.At(x, y), sea)
			}
		}
	}
	// along each grid's own directions: the first two opposite, the rest round
	dirs := g.Directions()
	link := func(ds ...int) cell.Links {
		var l cell.Links
		for _, d := range ds {
			l |= 1 << d
		}
		return l
	}
	back := func(d int) int { return g.Opposite(d) }
	for x := uint32(1); x < cols-2; x++ {
		brdW.SetWay(g.At(x, 1), cell.Way{Kind: road, Width: 7, Links: link(2%dirs, back(2%dirs))})
	}
	brdW.SetWay(g.At(cols-2, 1), cell.Way{Kind: road, Width: 7, Links: link(back(2 % dirs)),
		Fade: 0.6})
	brdW.SetWay(g.At(3, 1), cell.Way{Kind: road, Width: 7,
		Links: link(2%dirs, back(2%dirs), 1)})
	for y := uint32(0); y < rows-3; y++ {
		brdW.SetWay(g.At(5, y), cell.Way{Kind: river, Width: 8, Links: link(0, 1),
			Mix: float32(y) / float32(rows)})
	}
	brdW.SetWay(g.At(5, rows-4), cell.Way{Kind: river, Width: 8, Links: link(0, 1), Mix: 0.8})
	if dirs == 8 {
		// a road turning and slanting: west, north-east, then south-west, east
		brdW.SetWay(g.At(9, 7), cell.Way{Kind: road, Width: 6, Links: link(3)})
		brdW.SetWay(g.At(10, 7), cell.Way{Kind: road, Width: 6, Links: link(2, 5)})
		brdW.SetWay(g.At(11, 6), cell.Way{Kind: road, Width: 6, Links: link(6, 3)})
		brdW.SetWay(g.At(12, 6), cell.Way{Kind: road, Width: 6, Links: link(2, 0)})
		brdW.SetWay(g.At(12, 5), cell.Way{Kind: road, Width: 6, Links: link(1)})
		// a fork of three widths: the stem through the two widest
		brdW.SetWay(g.At(12, 3), cell.Way{Kind: road, Width: 10, Links: link(3)})
		brdW.SetWay(g.At(13, 3), cell.Way{Kind: road, Width: 7, Links: link(2, 3, 1)})
		brdW.SetWay(g.At(14, 3), cell.Way{Kind: road, Width: 4, Links: link(2)})
		brdW.SetWay(g.At(13, 4), cell.Way{Kind: road, Width: 6, Links: link(0), Fade: 0.3})
	}
	reliefFor(brd).SetHeights(func(p geom.Vec) float64 { return 40 - p.Y/8 + p.X/32 })
	atlas := render.NewAtlas()
	for id, c := range map[render.SpriteID]color.RGBA{1: {90, 160, 70, 255},
		2: {40, 80, 170, 255}, 3: {150, 110, 70, 255}, 4: {60, 110, 190, 255}} {
		atlas.Add(id, 16, sprite(c))
	}
	atlas.Close()
	d := New(brd, reliefFor(brd), true, st)
	d.kinds = kindsOf{grass, sea, road, river}
	return d, brdW, atlas
}

func TestPainter_PaintsTheWaysAnewRoundACellAsWhole(t *testing.T) {
	gputest.Need(t)
	g := grid.DefaultGrids{}.Square(16, 12, 32)
	d, brdW, atlas := wayBoard(t, g)
	if off := bakedApart(t, d, atlas); off != [3]int{} {
		t.Errorf("%v pixels of the albedo, the kinds and the water with the ways differ painted "+
			"twice, want none", off)
	}
	brdW.SetWay(g.At(8, 4), cell.Way{Kind: d.kinds.(kindsOf)[2], Width: 7, Links: 0x0f})
	if off := bakedApart(t, d, atlas); off != [3]int{} {
		t.Errorf("a way laid, %v pixels of the albedo, the kinds and the water painted round it "+
			"differ from the board painted whole, want none", off)
	}
}

// a cell holds its kind's detail where nothing else may lie on it, Dressed by a way or a ground
// running in
func TestPainter_TellsTheCellsTheirDetails(t *testing.T) {
	gputest.Need(t)
	g := grid.DefaultGrids{}.Square(16, 12, 32)
	d, _, atlas := wayBoard(t, g)
	d.Surface(atlas)
	cells, _ := d.CellDetails()
	for _, c := range []struct {
		x, y int
		want byte
		what string
	}{{14, 7, d.detailOf(1), "grass far from the ways and the sea"},
		{3, 1, Dressed, "a road's fork"}, {6, 4, Dressed, "the grass by the river"},
		{12, 8, Dressed, "the grass by the sea"}} {
		if got := cells[c.y*16+c.x]; got != c.want {
			t.Errorf("%s holds detail %d, want %d", c.what, got, c.want)
		}
	}
}

// an island of 96 by 64 cells as the benchmarks' (bench/board_test.go): earth, sand by the coast
// and rock up high round the sea, rivers drained down it as ways, the sea and the rivers shining
func island(tb testing.TB) (*Painter, *board.BoardWrite, render.AtlasSource, grid.Grid) {
	tb.Helper()
	const w, h, size = 96, 64, 32
	st := map[cell.Name]Style{}
	g := grid.DefaultGrids{}.Square(w, h, size)
	brd, brdW := newBoard(g)
	kinds := map[string]cell.Kind{
		"sea": styled(st, cell.Kind{Name: cell.Named("sea"), Allows: cell.Water, SpriteID: 1},
			Style{Shine: 0.9, Under: true}),
		"earth": styled(st, cell.Kind{Name: cell.Named("earth"), Allows: cell.Land, SpriteID: 2},
			Style{Spread: 0.3}),
		"sand": styled(st, cell.Kind{Name: cell.Named("sand"), Allows: cell.Land, SpriteID: 3},
			Style{Spread: 0.35}),
		"rock": styled(st, cell.Kind{Name: cell.Named("rock"), Allows: cell.Land, SpriteID: 4},
			Style{Spread: 0.25}),
		"stream": styled(st, cell.Kind{Name: cell.Named("stream"), Allows: cell.Land | cell.Water,
			SpriteID: 5}, Style{Shine: 0.9, Flow: 60}),
		"estuary": styled(st, cell.Kind{Name: cell.Named("estuary"), Allows: cell.Water,
			SpriteID: 6}, Style{Shine: 0.9, Flow: 45}),
	}
	inside := func(x, y float64) float64 { // 1 in the middle, 0 at the coast
		dx, dy := (x-w/2)/34, (y-h/2)/22
		return 1 - math.Hypot(dx, dy)*(1+0.1*math.Sin(5*math.Atan2(dy, dx)))
	}
	land := map[cell.ID]bool{}
	g.EachCell(func(c cell.ID) {
		x, y, _ := g.Coords(c)
		land[c] = inside(float64(x)+0.5, float64(y)+0.5) > 0
	})
	heights := func(q geom.Vec) float64 {
		x, y := q.X/size, q.Y/size
		for _, d := range [4][2]float64{{-0.5, -0.5}, {0.5, -0.5}, {-0.5, 0.5}, {0.5, 0.5}} {
			if c, ok := g.CellAt(geom.NewVec((x+d[0])*size, (y+d[1])*size)); !ok || !land[c] {
				return 0
			}
		}
		in := inside(x, y)
		return 8 + 160*in*in*(0.7+0.3*math.Sin(x/3)*math.Cos(y/4)) + 10*math.Sin(x/2+y/3)
	}
	rivers, err := drain.Drain(g, heights, func(c cell.ID) bool { return !land[c] }, drain.Config{
		BrookAt: 30, StreamAt: 60, RiverAt: 170, BrookDepth: 1, StreamDepth: 2, RiverDepth: 5,
		WidthPerRoot: 1.4, Meander: 6, Plume: 0.25,
	})
	if err != nil {
		tb.Fatal(err)
	}
	reliefFor(brd).SetHeights(rivers.Carved(heights))
	brdW.SetAll(kinds["sea"])
	g.EachCell(func(c cell.ID) {
		if !land[c] {
			return
		}
		at := g.CellCenter(c)
		kind := "earth"
		switch in := inside(at.X/size, at.Y/size); {
		case in < 0.08:
			kind = "sand"
		case heights(at) > 110:
			kind = "rock"
		}
		brdW.Set(c, kinds[kind])
	})
	for _, e := range rivers.Net(map[drain.Course]string{drain.Brook: "stream",
		drain.Stream: "stream", drain.River: "stream", drain.Ford: "stream",
		drain.Mouth: "estuary"}).Ways() {
		brdW.SetWay(e.Cell, cell.Way{Kind: kinds[e.Kind], Width: e.Width, Links: e.Links,
			Fade: e.Fade, Mix: e.Mix})
	}
	colours := map[render.SpriteID]color.RGBA{}
	for _, k := range kinds {
		colours[k.SpriteID] = color.RGBA{uint8(40 * k.SpriteID), 150, 80, 255}
	}
	d := New(brd, reliefFor(brd), true, st)
	return d, brdW, solids(colours), g
}

// Benchmark_Painter_Bake paints the island anew and waits for the GPU: whole, a Style changed
// every time, and round a cell changed every time.
func Benchmark_Painter_Bake(b *testing.B) {
	gputest.Need(b)
	d, brdW, atlas, g := island(b)
	d.Surface(atlas)
	one, px := render.NewImage(1, 1), make([]byte, 4)
	wait := func() {
		one.CopyFrom(d.albedo.water.SubImage(image.Rect(0, 0, 1, 1)))
		one.ReadPixels(px)
	}
	wait()
	b.Run("whole", func(b *testing.B) {
		for i := 0; b.Loop(); i++ {
			d.Style("earth", Style{Spread: 0.3 + 0.01*float32(i%2)})
			d.Surface(atlas)
			wait()
		}
	})
	b.Run("cell", func(b *testing.B) {
		c := g.At(40, 30)
		kinds := [2]cell.Kind{d.board.Kind(c), d.board.Kind(g.At(48, 2))}
		for i := 0; b.Loop(); i++ {
			brdW.Set(c, kinds[1-i%2])
			d.Surface(atlas)
			wait()
		}
	})
}
