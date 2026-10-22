package look

import (
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/render"
)

// Look is how the board's cells lie on the screen through a camera.
type Look interface {
	// Cell hands f the visible cell t.
	Cell(f *render.Frame, cam camera.Camera, t *Tile)
}

// Dressing is what a Map lays over the board's tiles beyond their sprites: the light on them and
// whatever lies on them.
type Dressing interface {
	// Begin readies the dressing for a frame through cam, before any tile.
	Begin(f *render.Frame, cam camera.Camera)
	// Sheet is what the tiles are drawn from this frame, the board's atlas given: it, or a sheet
	// of the dressing's with the atlas's sprites on it.
	Sheet(atlas render.AtlasSource) render.AtlasSource
	// Base is the sprite t's top is drawn in first.
	Base(t *Tile) render.SpriteID
	// Light is the light on t's top at its corners.
	Light(t *Tile) render.Shade
	// FaceLight is the light on t's upright face looking dx, dy cells away.
	FaceLight(t *Tile, dx, dy int) render.Light
	// Dress lays over t's top, drawn over the box x0..x1, y0..y1 lying at at, what lies on it.
	Dress(f *render.Frame, cam camera.Camera, t *Tile, x0, y0, x1, y1 float32, at render.At)
	// Across lays over t what runs across the board, the ways: over every tile's top and what
	// Dress laid on it.
	Across(f *render.Frame, cam camera.Camera, t *Tile)
}

// GroundDressing is a Dressing that lays something over the drawn ground itself, part of it:
// the clouds' shadows of an atmosphere.
type GroundDressing interface {
	// DressGround draws over the ground just drawn into t through cam, under the frame's u.
	DressGround(t render.Target, cam camera.Camera, u render.Uniforms)
}

// EvenLit is a Dressing that lights every tile of a flat map alike, so the map is composed once
// and drawn in EvenLight every frame.
type EvenLit interface {
	// EvenLight is the light every tile is lit in this frame; false where the tiles are lit apart
	// or dressed anew every frame.
	EvenLight() (render.Light, bool)
}

// Parallel is a Dressing that dresses tiles on several goroutines at once.
type Parallel interface {
	Dressing
	// Warm works out, on the frame's goroutine, what dressing t reads that is shared between
	// tiles, so no Worker writes what another reads.
	Warm(t *Tile)
	// Ready readies the Workers once every visible tile is Warmed, on the frame's goroutine.
	Ready()
	// Worker is the k-th Dressing for one goroutine of the frame, with scratch of its own. Called
	// on the frame's goroutine, after Ready.
	Worker(k int) Dressing
}

// ParallelLook is a Look that lays tiles on several goroutines at once, a Worker of it each.
type ParallelLook interface {
	Look
	// Worker is the k-th Look for one goroutine of the frame, drawing by the Parallel Dressing's
	// k-th Worker d. Called on the frame's goroutine.
	Worker(k int, d Dressing) Look
}

// Tile is one visible cell as the board's renderer hands it to a Look, good for that call.
type Tile struct {
	ID             cell.ID
	X0, Y0, X1, Y1 float32 // its box in the world
	Atlas          render.AtlasSource
	// Coarse, the tile is drawn plainly, undressed; Fog is how far into the fog before the
	// camera's sight it lies, 0 to 1
	Coarse bool
	Fog    float32

	r     *Renderer
	dress Dressing
	board Board // what the tile's goroutine reads the board through
}

// Sprite is the sprite of the cell's kind.
func (t *Tile) Sprite() render.SpriteID { return t.r.topOf(t.ID).sprite }

// Base is the sprite the tile's top is drawn in first: its own kind's, unless the Dressing says
// otherwise.
func (t *Tile) Base() render.SpriteID {
	if d := t.dress; d != nil {
		return d.Base(t)
	}
	return t.Sprite()
}

// Kind is the cell's kind as whoever crosses it meets it.
func (t *Tile) Kind() cell.Kind { return t.board.Kind(t.ID) }

// Light is the light on the tile's top at its corners: the Dressing's, even without one, under
// the sun as the renderer's Lighting has it.
func (t *Tile) Light() render.Shade {
	if t.r.white {
		return render.Even(1) // composed once, lit on the GPU
	}
	shade := render.Even(1)
	if d := t.dress; d != nil {
		shade = d.Light(t)
	}
	lighting := t.r.lit()
	if lighting == nil {
		return shade
	}
	lit := lighting.At((t.X0+t.X1)/2, (t.Y0+t.Y1)/2, 0).Light
	for c := range shade {
		for k := range shade[c] {
			shade[c][k] *= lit[k]
		}
	}
	return shade
}

// FaceLight is the light on an upright face of the tile looking dx, dy cells away: the
// Dressing's, even without one.
func (t *Tile) FaceLight(dx, dy int) render.Light {
	if d := t.dress; d != nil {
		return d.FaceLight(t, dx, dy)
	}
	return render.Light{1, 1, 1}
}

// Dress lays over the tile's top, drawn over the box x0..x1, y0..y1 lying at at, what the
// Dressing has lie on it; nothing without one.
func (t *Tile) Dress(f *render.Frame, cam camera.Camera, x0, y0, x1, y1 float32, at render.At) {
	if d := t.dress; d != nil && !t.Coarse {
		d.Dress(f, cam, t, x0, y0, x1, y1, at)
	}
}

// Sway is how much what stands on the cell bends in the wind and how high it stands over the
// ground; nothing where nothing stands at a height.
func (t *Tile) Sway() (amount, rise float32) {
	c := t.r.topOf(t.ID)
	if c.sway <= 0 || c.height <= 0 {
		return 0, 0
	}
	return c.sway, c.height
}

// Nothing is a Look laying no tile, for a Map whose ground is drawn some other way.
var Nothing Look = nothing{}

type nothing struct{}

func (nothing) Cell(*render.Frame, camera.Camera, *Tile) {}

type flatLook struct{}

// FlatLook is the board seen from above, the simple map's Look: each cell's sprite over its box,
// in the Dressing's light and dressed by it.
func FlatLook() Look { return flatLook{} }

var _ ParallelLook = flatLook{}

// Worker is the flat look itself: it keeps nothing between tiles.
func (l flatLook) Worker(int, Dressing) Look { return l }

func (flatLook) Cell(f *render.Frame, cam camera.Camera, t *Tile) {
	x0, y0, x1, y1 := t.X0, t.Y0, t.X1, t.Y1
	at := f.At((x0+x1)/2, (y0+y1)/2, 0)
	f.SpriteRect(at, t.Atlas, t.Base(), x0, y0, x1, y1, t.Light())
	f.Fog([4]float32{t.Fog, t.Fog, t.Fog, t.Fog})
	t.Dress(f, cam, x0, y0, x1, y1, at)
}
