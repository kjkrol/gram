package camera

import (
	"github.com/kjkrol/aabbworld/geom"
)

// AABB is geom.AABB, the world-coordinate rectangle camera and render speak in.
type AABB = geom.AABB

// Quad is one piece of a rectangle projected to screen space, with its UV sub-range in T0/T1.
type Quad struct {
	X0, Y0, X1, Y1     float32
	T0X, T1X, T0Y, T1Y float32
}

// FromScreenRect converts a screen rectangle to world space; on a torus it may need WrapAABB.
func FromScreenRect(cam Camera, sx0, sy0, sx1, sy1 float32) (x0, y0, x1, y1 float32) {
	x0, y0 = cam.FromScreen(sx0, sy0)
	scale := cam.Zoom()
	return x0, y0, x0 + (sx1-sx0)/scale, y0 + (sy1-sy0)/scale
}

// Camera is the view onto a world: screen conversion, culling, the visible window and its control.
type Camera interface {
	// Projection is the mapping this camera draws through.
	Projection() Projection
	// Project maps the world point (x, y) at height z to the screen.
	Project(x, y, z float32) (sx, sy float32)
	// Unproject inverts Project at height z.
	Unproject(sx, sy, z float32) (x, y float32)
	// Depth orders drawing through the projection: further back is smaller.
	Depth(x, y, z float32) float32
	// Viewport is the screen the camera draws to, in pixels.
	Viewport() (w, h float32)
	// SetViewport resizes the screen the camera draws to, keeping the point in the middle of it.
	// A screen larger than the world at the current zoom raises the zoom until the world covers it.
	SetViewport(w, h float32)
	// ToScreen is Project at height 0.
	ToScreen(x, y float32) (float32, float32)
	// FromScreen is Unproject at height 0.
	FromScreen(sx, sy float32) (float32, float32)
	// ToScreenQuads appends to dst the rectangle projected to screen space, split at a wrap seam.
	ToScreenQuads(x0, y0, x1, y1 float32, dst []Quad) []Quad
	// Visible reports whether box lies in view.
	Visible(box AABB) bool
	// Bounds is the visible window in world units.
	Bounds() AABB
	// MoveTo puts the visible window's top-left corner at (x, y), keeping its size.
	MoveTo(x, y float64)
	// CenterOn puts the world point (x, y) at height z in the middle of the screen, as far as the
	// window may go. A top-down camera ignores z.
	CenterOn(x, y, z float64)
	// Translate shifts the visible window by a delta in world units.
	Translate(dx, dy float64)
	// Pan shifts the visible window by a delta in screen pixels, the same at any zoom.
	Pan(dx, dy float32)
	// Zoom is the current zoom factor, screen pixels a world unit.
	Zoom() float32
	// ZoomIn multiplies the zoom by factor, keeping the ground point (anchorX, anchorY) fixed on
	// screen as far as the camera's limits allow.
	ZoomIn(factor float32, anchorX, anchorY float32)
	// ZoomOut is ZoomIn(1/factor, anchorX, anchorY).
	ZoomOut(factor float32, anchorX, anchorY float32)
	// State is the camera's current window and zoom.
	State() State
	// Persisted is pointers to the live window, zoom and fastening, for saves and loads.
	Persisted() []any
	// Restore rebuilds derived state after a load has written through Persisted's pointers.
	Restore()
	// SetMinZoom raises ZoomOut's floor above the world-fit one; 0 keeps only that.
	SetMinZoom(minZoom float32)
	// SetMaxZoom caps ZoomIn; 0 leaves it unrestricted.
	SetMaxZoom(maxZoom float32)
}

// Config is how a camera starts: its window on the world and what it follows. It is never
// persisted: a loaded camera is as it was saved.
type Config struct {
	// Zoom is the scale it starts at, screen pixels a world unit; 0 is 1.
	Zoom float32
	// Whole keeps the whole world in view at whatever size the camera is shown, centred, the
	// background in bars: a minimap. Such a camera is not panned or zoomed.
	Whole bool
	// Follow is the name of the entity it starts fastened Centred over; empty, it starts fastened
	// to nothing.
	Follow string
	// MinZoom raises ZoomOut's floor above the world-fit one; 0 keeps only that.
	MinZoom float32
	// MaxZoom caps ZoomIn; 0 leaves it unrestricted.
	MaxZoom float32
}

// Fitting is a camera that may keep the whole world in view (Config.Whole). Fits says whether it
// does, and the world's size, whose proportions a picture of it keeps.
type Fitting interface {
	Fits() (worldWidth, worldHeight float32, ok bool)
}

// State is a Camera's visible window and zoom.
type State struct {
	Viewport AABB
	Zoom     float32
}
