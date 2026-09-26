package board

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
)

// Look is how the board's cells lie on the screen through a camera: the board's way of drawing its
// ground. The board starts with a flat look, seen from above; a view plugin puts its own in with
// Plugin.SetLook.
type Look interface {
	// Cell hands f the visible cell t.
	Cell(f *render.Frame, cam camera.Camera, t *Tile)
}

// Tile is one visible cell as the board's renderer hands it to a Look, good for that call.
type Tile struct {
	ID             CellID
	X0, Y0, X1, Y1 float32 // its box in the world
	Atlas          render.AtlasSource
	// Outlined asks the Look to outline the tile along its own edges: the grid is on.
	Outlined bool

	r *Renderer
}

// Sprite is the sprite of the cell's kind.
func (t *Tile) Sprite() render.SpriteID { return t.r.topOf(t.ID).sprite }

// Top is the height of the tile's corners — top-left, top-right, bottom-left, bottom-right — with
// its kind standing on them, and its ground level.
func (t *Tile) Top() (corners [4]float32, level float32) {
	c := t.r.topOf(t.ID)
	return c.z, c.alt
}

// Beside is the Top corners of the cell dx, dy cells away; sea level 0 off the board.
func (t *Tile) Beside(dx, dy int) [4]float32 {
	w, h := t.X1-t.X0, t.Y1-t.Y0
	x, y := (t.X0+t.X1)/2+float32(dx)*w, (t.Y0+t.Y1)/2+float32(dy)*h
	c, ok := t.r.board.CellAt(geom.NewVec(float64(x), float64(y)))
	if !ok {
		return [4]float32{}
	}
	return t.r.topOf(c).z
}

// flatLook is the board seen from above: each cell's sprite over its box, split at a wrap seam.
type flatLook struct{}

func (flatLook) Cell(f *render.Frame, _ camera.Camera, t *Tile) {
	if t.Outlined {
		f.TileRect(render.Ground, 0, t.Atlas, t.Sprite(), t.X0, t.Y0, t.X1, t.Y1)
		return
	}
	f.SpriteRect(render.Ground, 0, t.Atlas, t.Sprite(), t.X0, t.Y0, t.X1, t.Y1)
}
