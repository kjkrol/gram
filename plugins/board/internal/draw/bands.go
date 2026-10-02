package draw

import (
	"image/color"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/render"
)

// bandTier puts the ways' bands over the tiles and under the grid's lines.
const bandTier = render.Ground + 5

// Ways is what Bands read of a board: where each cell's neighbours lie and what runs across it.
type Ways interface {
	Toward(c cell.ID, i int) (cell.ID, bool)
	CellCenter(c cell.ID) geom.Vec
	Way(c cell.ID) cell.Way
	Crossing(c cell.ID) cell.Crossing
}

// Bands is the simple map's Dressing: it lays the ways and the crossings over the tiles as plain
// bands in their kinds' Colors, from the cell's middle out to the edge towards each neighbour the
// way runs on to, as wide as the way, faded as far as it has faded. The tiles are drawn as they
// are; a sky over the board (plugins/atmosphere) lights them.
type Bands struct {
	board Ways
	pts   [][2]float32
}

var _ look.Dressing = (*Bands)(nil)
var _ look.EvenLit = (*Bands)(nil)

// NewBands lays the ways of b.
func NewBands(b Ways) *Bands { return &Bands{board: b} }

func (*Bands) Begin(*render.Frame, camera.Camera)                {}
func (*Bands) Sheet(atlas render.AtlasSource) render.AtlasSource { return atlas }
func (*Bands) Base(t *look.Tile) render.SpriteID                 { return t.Sprite() }
func (*Bands) FaceLight(*look.Tile, int, int) render.Light       { return render.Light{1, 1, 1} }
func (*Bands) Covers(*look.Tile) bool                            { return false }

func (*Bands) Light(*look.Tile) render.Shade { return render.Even(1) }

// EvenLight is white: the simple map's tiles are drawn as they are.
func (*Bands) EvenLight() (render.Light, bool) { return render.Light{1, 1, 1}, true }

func (d *Bands) Dress(f *render.Frame, cam camera.Camera, t *look.Tile, x0, y0, x1, y1, depth float32) {
	if w := d.board.Way(t.ID); w.Runs() {
		d.bands(f, cam, t, w, depth)
	}
	if x := d.board.Crossing(t.ID); x.Runs() {
		d.bands(f, cam, t, x.Way, depth+1e-3)
	}
}

// bands lays w's bands over t: a hub at the cell's middle and a band out to each neighbour's edge.
func (d *Bands) bands(f *render.Frame, cam camera.Camera, t *look.Tile, w cell.Way, depth float32) {
	c := w.Kind.Color
	if c.A == 0 {
		c = color.RGBA{R: 128, G: 128, B: 128, A: 255} // a kind of no colour: grey
	}
	if w.Fade > 0 {
		keep := 1 - min(w.Fade, 1)
		c = color.RGBA{R: uint8(float32(c.R) * keep), G: uint8(float32(c.G) * keep), B: uint8(float32(c.B) * keep), A: uint8(float32(c.A) * keep)}
	}
	cx, cy := (t.X0+t.X1)/2, (t.Y0+t.Y1)/2
	half := w.Width / 2
	reach := max(t.X1-t.X0, t.Y1-t.Y0) * 1.5
	for i := range 8 {
		if w.Links&(1<<i) == 0 {
			continue
		}
		n, ok := d.board.Toward(t.ID, i)
		if !ok {
			continue
		}
		nc := d.board.CellCenter(n)
		dx, dy := float32(nc.X)-cx, float32(nc.Y)-cy
		if abs32(dx) > reach || abs32(dy) > reach {
			continue // across a wrapping seam: the neighbour draws its own half
		}
		l := float32(math.Hypot(float64(dx), float64(dy)))
		if l == 0 {
			continue
		}
		px, py := -dy/l*half, dx/l*half // across the band
		mx, my := cx+dx/2, cy+dy/2
		var dst render.Corners
		for k, p := range [4][2]float32{{cx + px, cy + py}, {cx - px, cy - py}, {mx + px, my + py}, {mx - px, my - py}} {
			dst[k][0], dst[k][1] = cam.Project(p[0], p[1], 0)
		}
		f.Soft(bandTier, depth, dst, c, render.Fade{})
	}
	// the hub: an octagon of the band's width, so the bands meet round without a notch
	d.pts = d.pts[:0]
	for k := range 8 {
		a := float64(k) * math.Pi / 4
		sx, sy := cam.Project(cx+half*float32(math.Cos(a)), cy+half*float32(math.Sin(a)), 0)
		d.pts = append(d.pts, [2]float32{sx, sy})
	}
	f.Fan(bandTier, depth, d.pts, c)
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
