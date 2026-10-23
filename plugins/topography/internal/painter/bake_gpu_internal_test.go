package painter

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
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
	grass := styled(st, cell.Kind{Name: cell.Named("b1"), Allows: cell.Land, SpriteID: 1},
		Style{Spread: 0.3})
	sand := styled(st, cell.Kind{Name: cell.Named("b2"), Allows: cell.Land, SpriteID: 2},
		Style{Spread: 0.2})
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
	reliefFor(brd).SetHeights(func(p geom.Vec) float64 { return 20 + p.X/16 - p.Y/32 })
	atlas := render.NewAtlas()
	for id, c := range map[render.SpriteID]color.RGBA{1: {90, 160, 70, 255},
		2: {210, 190, 120, 255}, 3: {120, 120, 130, 255}, 4: {40, 80, 170, 255}} {
		atlas.Add(id, 16, sprite(c))
	}
	atlas.Close()
	d := New(brd, reliefFor(brd), skyOf(sky.DefaultSun), true, st)
	d.kinds = kindsOf{grass, sand, rock, sea}
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

func TestPainter_BakesTheAlbedoOnTheGPUAsOnTheCPU(t *testing.T) {
	needGPU(t)
	d, brdW, atlas, g := bakedBoard(t)
	cpu, _, px, _ := d.Surface(atlas)
	laid := render.NewImage(cpu.Bounds().Dx(), cpu.Bounds().Dy())
	d.refresh(atlas)
	d.bakeAlbedo(laid, atlas, px, d.toPaint())
	if off := differ(t, laid, cpu); off > 0 {
		t.Errorf("%d pixels of the albedo differ from the CPU's, want none", off)
	}
	// a cell changed, painted anew round it alone
	brdW.Set(g.At(8, 1), d.kinds.(kindsOf)[3])
	cpu, _, _, _ = d.Surface(atlas)
	d.refresh(atlas)
	if at := d.toPaint(); len(at) != 4*20 {
		t.Errorf("one cell by the edge changed, %d cells painted anew, want the 20 within two "+
			"on the board", len(at)/4)
	}
	d.bakeAlbedo(laid, atlas, px, d.toPaint())
	if off := differ(t, laid, cpu); off > 0 {
		t.Errorf("a cell changed, %d pixels of the albedo differ from the CPU's, want none", off)
	}
}

// a board of land round a sea, ways across it: a road straight, turning and forking, a track
// fading out, a river turning into the sea and running into it, a road along the shore
func wayBoard(t *testing.T, g grid.Grid) (*Painter, *board.BoardWrite, render.AtlasSource) {
	t.Helper()
	st := map[cell.Name]Style{}
	brd, brdW := newBoard(g)
	grass := styled(st, cell.Kind{Name: cell.Named("w1"), Allows: cell.Land, SpriteID: 1},
		Style{Spread: 0.3})
	sea := styled(st, cell.Kind{Name: cell.Named("w2"), Allows: cell.Water, SpriteID: 2},
		Style{Under: true, Shine: 0.8})
	road := styled(st, cell.Kind{Name: cell.Named("w3"), Allows: cell.Land, SpriteID: 3},
		Style{})
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
	d := New(brd, reliefFor(brd), skyOf(sky.DefaultSun), true, st)
	d.kinds = kindsOf{grass, sea, road, river}
	return d, brdW, atlas
}

func TestPainter_BakesTheWaysOnTheGPUAsOnTheCPU(t *testing.T) {
	needGPU(t)
	g := grid.DefaultGrids{}.Square(16, 12, 32)
	d, brdW, atlas := wayBoard(t, g)
	cpu, _, px, _ := d.Surface(atlas)
	laid := render.NewImage(cpu.Bounds().Dx(), cpu.Bounds().Dy())
	bake := func() {
		d.refresh(atlas)
		at := d.toPaint()
		d.bakeAlbedo(laid, atlas, px, at)
		d.bakeWays(laid, atlas, px, at)
	}
	bake()
	if off := differ(t, laid, cpu); off > 0 {
		t.Errorf("%d pixels of the albedo with its ways differ from the CPU's, want none", off)
	}
	brdW.SetWay(g.At(8, 4), cell.Way{Kind: d.kinds.(kindsOf)[2], Width: 7, Links: 0x0f})
	cpu, _, _, _ = d.Surface(atlas)
	bake()
	if off := differ(t, laid, cpu); off > 0 {
		t.Errorf("a way laid, %d pixels of the albedo differ from the CPU's, want none", off)
	}
}
