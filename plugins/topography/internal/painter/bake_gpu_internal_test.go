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
