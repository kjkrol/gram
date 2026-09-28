package board

import (
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/render"
)

// Look is how the board's cells lie on the screen through a camera: the board's way of drawing its
// ground, its Map's — the simple map's flat look, seen from above, or a topography's.
type Look interface {
	// Cell hands f the visible cell t.
	Cell(f *render.Frame, cam camera.Camera, t *Tile)
}

// Dressing is what a Map lays over the board's tiles beyond their sprites: the light on them and
// whatever lies on them, given its turn by the board's renderer and its Look. The simple map's
// lays the ways as plain bands; a topography's the light on the relief, water, blends and more.
type Dressing interface {
	// Begin readies the dressing for a frame through cam, before any tile.
	Begin(f *render.Frame, cam camera.Camera)
	// Sheet is what the tiles are drawn from this frame, the board's atlas given: it, or a sheet
	// of the dressing's with the atlas's sprites where they are on it and more.
	Sheet(atlas render.AtlasSource) render.AtlasSource
	// Base is the sprite t's top is drawn in first.
	Base(t *Tile) render.SpriteID
	// Light is the light on t's top at its corners.
	Light(t *Tile) render.Shade
	// FaceLight is the light on t's upright face looking dx, dy cells away.
	FaceLight(t *Tile, dx, dy int) render.Light
	// Covers reports whether Dress lays over t's top what would hide its outline: a Look draws the
	// top without one then, and the dressing lays the outline over what it covers.
	Covers(t *Tile) bool
	// Dress lays over t's top, drawn over the box x0..x1, y0..y1 at depth, what lies on it.
	Dress(f *render.Frame, cam camera.Camera, t *Tile, x0, y0, x1, y1, depth float32)
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

// Base is the sprite the tile's top is drawn in first: its own kind's, unless the Dressing says
// otherwise — the sea under a coast.
func (t *Tile) Base() render.SpriteID {
	if d := t.r.dressing(); d != nil {
		return d.Base(t)
	}
	return t.Sprite()
}

// Kind is the cell's kind as whoever crosses it meets it.
func (t *Tile) Kind() CellKind { return t.r.board.Kind(t.ID) }

// Light is the light on the tile's top at its corners: the Dressing's; without one the sun's on
// level ground where the world is sunlit, even otherwise.
func (t *Tile) Light() render.Shade {
	if d := t.r.dressing(); d != nil {
		return d.Light(t)
	}
	if t.r.sunlit != nil && t.r.sunlit() {
		return render.Lit(t.r.sun().Light(0, 0, 1))
	}
	return render.Even(1)
}

// FaceLight is the light on an upright face of the tile looking dx, dy cells away — towards a
// neighbour it stands above: the Dressing's, even without one.
func (t *Tile) FaceLight(dx, dy int) render.Light {
	if d := t.r.dressing(); d != nil {
		return d.FaceLight(t, dx, dy)
	}
	return render.Light{1, 1, 1}
}

// Covered reports whether the Dressing lays over the tile's top what would hide its outline, and
// outlines it over that itself where it is Outlined: its Look draws the top without an outline.
func (t *Tile) Covered() bool {
	if d := t.r.dressing(); d != nil {
		return d.Covers(t)
	}
	return false
}

// Dress lays over the tile's top, just drawn over the box x0..x1, y0..y1 at depth, what the
// Dressing has lie on it; nothing without one.
func (t *Tile) Dress(f *render.Frame, cam camera.Camera, x0, y0, x1, y1, depth float32) {
	if d := t.r.dressing(); d != nil {
		d.Dress(f, cam, t, x0, y0, x1, y1, depth)
	}
}

// Sway is how much what stands on the cell bends in the wind — its kind's Sway — and how high it
// stands over the ground (its kind's Height), which is how far its top leans; nothing in a flat
// world, where nothing stands at a height.
func (t *Tile) Sway() (amount, rise float32) {
	c := t.r.topOf(t.ID)
	if c.sway <= 0 || !t.r.board.quasi3D {
		return 0, 0
	}
	return c.sway, c.height
}

// flatLook is the board seen from above: each cell's sprite over its box, split at a wrap seam, in
// the Dressing's light and dressed by it.
type flatLook struct{}

// FlatLook is the board seen from above, the simple map's Look: each cell's sprite over its box,
// in the Dressing's light and dressed by it — for another Map to lay its cells so where it looks
// from above.
func FlatLook() Look { return flatLook{} }

func (flatLook) Cell(f *render.Frame, cam camera.Camera, t *Tile) {
	x0, y0, x1, y1 := t.X0, t.Y0, t.X1, t.Y1
	// what sways is seen from above by its top, leaning with the wind
	if amount, rise := t.Sway(); amount > 0 {
		lx, ly := render.Sway(f.Time(), f.Wind(), (x0+x1)/2, (y0+y1)/2, amount)
		x0, y0, x1, y1 = x0+lx*rise, y0+ly*rise, x1+lx*rise, y1+ly*rise
	}
	if t.Outlined && !t.Covered() {
		f.TileRect(render.Ground, 0, t.Atlas, t.Base(), x0, y0, x1, y1, t.Light())
	} else {
		f.SpriteRect(render.Ground, 0, t.Atlas, t.Base(), x0, y0, x1, y1, t.Light())
	}
	t.Dress(f, cam, x0, y0, x1, y1, 0)
}
