package draw

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/render"
)

// over the tiles, under the grid's lines
const bandTier = render.Ground + 5

// Ways is what Bands read of a board: where each cell's neighbours lie and what runs across it.
type Ways interface {
	Toward(c cell.ID, i int) (cell.ID, bool)
	Directions() int
	Offset(i int) (da, db int)
	Step(i int) geom.Vec
	CellCenter(c cell.ID) geom.Vec
	Way(c cell.ID) cell.Way
	// States are the markers of the effects on a cell, Shape the grid's: what the covers read.
	States(c cell.ID) tag.Tags[effect.States]
	Shape() (grid.Shape, bool)
}

// Bands is the simple map's Dressing: what lies on the cells under an effect (Cover), then the
// ways over the tiles as bands cut out of their kinds' sprites, as wide as the way, faded as far
// as it has faded.
type Bands struct {
	board  Ways
	pts    [][2]float32
	covers []Cover
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
	d.covered(f, cam, t, depth)
	if w := d.board.Way(t.ID); w.Runs() {
		d.bands(f, cam, t, w, depth)
	}
}

func (d *Bands) bands(f *render.Frame, cam camera.Camera, t *look.Tile, w cell.Way, depth float32) {
	keep := 1 - min(max(w.Fade, 0), 1)
	var box [4]float32
	box[0], box[1] = cam.Project(t.X0, t.Y0, 0)
	box[2], box[3] = cam.Project(t.X1, t.Y1, 0)
	cx, cy := (t.X0+t.X1)/2, (t.Y0+t.Y1)/2
	half := w.Width / 2
	for i := range d.board.Directions() {
		if !w.Links.Has(i) {
			continue
		}
		if _, ok := d.board.Toward(t.ID, i); !ok {
			continue
		}
		s := d.board.Step(i)
		dx, dy := float32(s.X), float32(s.Y)
		l := float32(math.Hypot(float64(dx), float64(dy)))
		if l == 0 {
			continue
		}
		px, py := -dy/l*half, dx/l*half // across the band
		mx, my := cx+dx/2, cy+dy/2
		d.pts = d.pts[:0]
		for _, p := range [4][2]float32{{cx + px, cy + py}, {mx + px, my + py}, {mx - px, my - py}, {cx - px, cy - py}} {
			sx, sy := cam.Project(p[0], p[1], 0)
			d.pts = append(d.pts, [2]float32{sx, sy})
		}
		f.SpriteFan(bandTier, depth, t.Atlas, w.Kind.SpriteID, d.pts, box, keep)
	}
	// the hub: an octagon of the band's width, so the bands meet round without a notch
	d.pts = d.pts[:0]
	for k := range 8 {
		a := float64(k) * math.Pi / 4
		sx, sy := cam.Project(cx+half*float32(math.Cos(a)), cy+half*float32(math.Sin(a)), 0)
		d.pts = append(d.pts, [2]float32{sx, sy})
	}
	f.SpriteFan(bandTier, depth, t.Atlas, w.Kind.SpriteID, d.pts, box, keep)
}
