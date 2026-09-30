package board

import (
	"image/color"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
)

// Map is what a board is drawn and priced by beyond what its cells say: how the cells lie on the
// screen (Look), what lies over them beyond their sprites (Dressing), how high they stand (Top)
// and what a step costs beyond its kind's cost (Climb, Least, Slope). The board's own map is the
// simple map: a flat world seen from above, every kind in its Color or drawn sprite, ways and
// crossings as plain bands, a step at its kind's cost times the distance. plugins/topography is
// the other: a map in relief, lit and shaded, seen from above or isometrically.
type Map interface {
	// Look is how the cells lie on the screen through a camera.
	Look() Look
	// Heights is the height of the ground at any point, for sight and navigation; nil on a flat map.
	Heights() Heights
	// Dressing lays over the tiles what lies on them beyond their sprites; nil for nothing.
	Dressing() Dressing
	// Top is the height of c's corners as drawn — top-left, top-right, bottom-left, bottom-right —
	// and of its ground: zero on a flat map.
	Top(c CellID) (corners [4]float32, level float32)
	// Climb is how many times as long the step from one cell to its neighbour takes whoever moves
	// in d as it would on the flat: the slope's; 1 on a flat map.
	Climb(from, to CellID, d Domain) float64
	// Least is the smallest Climb for d — the planner's estimate counts on it; 1 on a flat map.
	Least(d Domain) float64
	// Slope is how many times as long moving at p towards dir takes whoever moves in d: the slope
	// under the entity; 1 on a flat map.
	Slope(p, dir geom.Vec, d Domain) float64
}

// simpleMap is the board's own Map: flat, from above, plain bands for the ways.
type simpleMap struct {
	look     flatLook
	dressing simpleDressing
}

// newSimpleMap is the simple map over brd.
func newSimpleMap(brd *Board) *simpleMap {
	return &simpleMap{dressing: simpleDressing{board: brd}}
}

func (m *simpleMap) Look() Look                                   { return m.look }
func (*simpleMap) Heights() Heights                               { return nil }
func (m *simpleMap) Dressing() Dressing                           { return &m.dressing }
func (*simpleMap) Top(CellID) (corners [4]float32, level float32) { return corners, 0 }
func (*simpleMap) Climb(CellID, CellID, Domain) float64           { return 1 }
func (*simpleMap) Least(Domain) float64                           { return 1 }
func (*simpleMap) Slope(geom.Vec, geom.Vec, Domain) float64       { return 1 }

// bandTier puts the ways' bands over the tiles and under the grid's lines.
const bandTier = render.Ground + 5

// simpleDressing lays the ways and the crossings over the tiles as plain bands in their kinds'
// Colors: from the cell's middle out to the edge towards each neighbour the way runs on to, as
// wide as the way, faded as far as it has faded. The tiles are drawn as they are; a sky over the
// board (plugins/atmosphere) lights them.
type simpleDressing struct {
	board *Board
	pts   [][2]float32
}

var _ Dressing = (*simpleDressing)(nil)

func (*simpleDressing) Begin(*render.Frame, camera.Camera)                {}
func (*simpleDressing) Sheet(atlas render.AtlasSource) render.AtlasSource { return atlas }
func (*simpleDressing) Base(t *Tile) render.SpriteID                      { return t.Sprite() }
func (*simpleDressing) FaceLight(*Tile, int, int) render.Light            { return render.Light{1, 1, 1} }
func (*simpleDressing) Covers(*Tile) bool                                 { return false }

func (*simpleDressing) Light(*Tile) render.Shade { return render.Even(1) }

// EvenLight is white: the simple map's tiles are drawn as they are.
func (*simpleDressing) EvenLight() (render.Light, bool) { return render.Light{1, 1, 1}, true }

func (d *simpleDressing) Dress(f *render.Frame, cam camera.Camera, t *Tile, x0, y0, x1, y1, depth float32) {
	if w := d.board.Way(t.ID); w.Runs() {
		d.bands(f, cam, t, w, depth)
	}
	if x := d.board.Crossing(t.ID); x.Runs() {
		d.bands(f, cam, t, x.Way, depth+1e-3)
	}
}

// bands lays w's bands over t: a hub at the cell's middle and a band out to each neighbour's edge.
func (d *simpleDressing) bands(f *render.Frame, cam camera.Camera, t *Tile, w Way, depth float32) {
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
		n, ok := Toward(d.board.Grid, t.ID, i)
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
