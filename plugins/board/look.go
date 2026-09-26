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

// Light is how brightly the world's sun lights the tile's top at its corners, from the slope of
// the ground there — the tile's own corners and its neighbours', so slopes run on smoothly from
// tile to tile; what stands on the cell is lit as the ground under it. A flat world is drawn as its
// sprites are.
func (t *Tile) Light() render.Shade {
	r := t.r
	if !r.board.quasi3D {
		return render.Even(1)
	}
	g := r.topOf(t.ID).ground
	left, lok := t.groundBeside(-1, 0)
	right, rok := t.groundBeside(1, 0)
	up, uok := t.groundBeside(0, -1)
	down, dok := t.groundBeside(0, 1)
	w, h := t.X1-t.X0, t.Y1-t.Y0
	// the ground's rise along x and y at each corner, from the corners either side of it
	slope := func(ahead, behind float32, aok, bok bool, own0, own1, step float32) float32 {
		switch {
		case aok && bok:
			return (ahead - behind) / (2 * step)
		case aok:
			return (ahead - own0) / step
		case bok:
			return (own1 - behind) / step
		}
		return (own1 - own0) / step
	}
	sun := r.lighted
	lit := t.sunlit()
	corner := func(k int, dx, dy float32) float32 { return sun.Shaded(-dx, -dy, 1, lit[k]) }
	return render.Shade{
		corner(0, slope(g[1], left[0], true, lok, g[0], g[1], w), slope(g[2], up[0], true, uok, g[0], g[2], h)),
		corner(1, slope(right[1], g[0], rok, true, g[0], g[1], w), slope(g[3], up[1], true, uok, g[1], g[3], h)),
		corner(2, slope(g[3], left[2], true, lok, g[2], g[3], w), slope(down[2], g[0], dok, true, g[0], g[2], h)),
		corner(3, slope(right[3], g[2], rok, true, g[2], g[3], w), slope(down[3], g[1], dok, true, g[1], g[3], h)),
	}
}

// Shine is how much of the sun the tile's top throws back at the eye at its corners — its kind's
// Shine where the sun reaches it, none at night — and false where the tile does not shine at all:
// a kind without shine, a flat world. A Look hands it to [render.Frame.Glint] after the top's
// sprite.
func (t *Tile) Shine() (render.Shade, bool) {
	shine := t.r.topOf(t.ID).shine
	if shine <= 0 || !t.r.board.quasi3D {
		return render.Shade{}, false
	}
	lit := t.sunlit()
	return render.Shade{shine * lit[0], shine * lit[1], shine * lit[2], shine * lit[3]}, true
}

// Shore is the way from each corner of the tile's top to the nearest cell within a few that does not
// shine — the shore of the water the tile is part of — how far it is and how near; open water
// beyond, and on a grid other than square. A Look hands it to [render.Frame.Glint] with the Shine.
func (t *Tile) Shore() render.Shore { return t.r.shoreOf(t.X0, t.Y0, t.X1, t.Y1) }

// sunlit is how much sun reaches each corner of the tile's top: all of it where the board casts no
// shadows.
func (t *Tile) sunlit() [4]float32 {
	r := t.r
	if !r.shadows || !r.board.sloped() {
		return [4]float32{1, 1, 1, 1}
	}
	return r.sunlitOf(t.ID, t.X0, t.Y0, t.X1, t.Y1)
}

// FaceLight is how brightly the world's sun lights an upright face of the tile looking dx, dy
// cells away — towards a neighbour it stands above — as much in the sun as the top's edge over it.
func (t *Tile) FaceLight(dx, dy int) float32 {
	if !t.r.board.quasi3D {
		return 1
	}
	lit := t.sunlit()
	var edge float32
	switch {
	case dx > 0:
		edge = (lit[1] + lit[3]) / 2
	case dx < 0:
		edge = (lit[0] + lit[2]) / 2
	case dy > 0:
		edge = (lit[2] + lit[3]) / 2
	default:
		edge = (lit[0] + lit[1]) / 2
	}
	return t.r.lighted.Shaded(float32(dx), float32(dy), 0, edge)
}

// groundBeside is the ground's corners of the cell dx, dy cells away; false off the board.
func (t *Tile) groundBeside(dx, dy int) ([4]float32, bool) {
	w, h := t.X1-t.X0, t.Y1-t.Y0
	x, y := (t.X0+t.X1)/2+float32(dx)*w, (t.Y0+t.Y1)/2+float32(dy)*h
	c, ok := t.r.board.CellAt(geom.NewVec(float64(x), float64(y)))
	if !ok {
		return [4]float32{}, false
	}
	return t.r.topOf(c).ground, true
}

// flatLook is the board seen from above: each cell's sprite over its box, split at a wrap seam, lit
// by the sun where the ground slopes.
type flatLook struct{}

func (flatLook) Cell(f *render.Frame, _ camera.Camera, t *Tile) {
	if t.Outlined {
		f.TileRect(render.Ground, 0, t.Atlas, t.Sprite(), t.X0, t.Y0, t.X1, t.Y1, t.Light())
	} else {
		f.SpriteRect(render.Ground, 0, t.Atlas, t.Sprite(), t.X0, t.Y0, t.X1, t.Y1, t.Light())
	}
	if shine, ok := t.Shine(); ok {
		f.Glint(t.X0, t.Y0, t.X1, t.Y1, shine, t.Shore())
	}
}
