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

// Camera is what renderers, plain click/drag logic, and Runtime need:
// screen conversion, culling, viewport bounds, and control (move/zoom).
type Camera interface {
	// Projection is the mapping this camera draws through — TopDown unless configured otherwise.
	Projection() Projection
	// Project maps the world point (x, y) at height z to the screen; ToScreen is Project at 0.
	Project(x, y, z float32) (sx, sy float32)
	// Unproject inverts Project at height z; FromScreen is Unproject at 0.
	Unproject(sx, sy, z float32) (x, y float32)
	// Depth orders drawing through the projection: further back is smaller, drawn first.
	Depth(x, y, z float32) float32
	// Viewport is the screen the camera draws to, in pixels.
	Viewport() (w, h float32)
	// SetViewport resizes the screen the camera draws to — as whoever shows it gives it its size —
	// keeping the point in the middle of it; a screen larger than the world at the current zoom
	// raises the zoom until the world covers it; a camera keeping the whole world in view fits it.
	SetViewport(w, h float32)
	ToScreen(x, y float32) (float32, float32)
	// FromScreen inverts ToScreen: screen coordinates back to world coordinates.
	FromScreen(sx, sy float32) (float32, float32)
	// ToScreenQuads appends to dst the rectangle projected to screen space, split at a wrap seam.
	ToScreenQuads(x0, y0, x1, y1 float32, dst []Quad) []Quad
	Visible(box AABB) bool
	// Bounds returns the current effective (post-zoom) world-space viewport.
	Bounds() AABB
	// MoveTo repositions the visible window's top-left corner, keeping size.
	MoveTo(x, y float64)
	// CenterOn puts the world point (x, y) at height z in the middle of the screen, as far as the
	// window may go; a top-down camera ignores z.
	CenterOn(x, y, z float64)
	// Translate shifts the visible window by a signed delta in world units.
	Translate(dx, dy float64)
	// Pan shifts the visible window by a screen-space delta: the same pixels at any zoom.
	Pan(dx, dy float32)
	// Zoom returns the current zoom factor (1 = default).
	Zoom() float32
	// ZoomIn multiplies the zoom by factor, keeping the ground point (anchorX, anchorY) fixed on
	// screen — at its drawn height in a camera in relief, and as far as the camera's limits allow.
	ZoomIn(factor float32, anchorX, anchorY float32)
	// ZoomOut is ZoomIn(1/factor, anchorX, anchorY).
	ZoomOut(factor float32, anchorX, anchorY float32)
	// State returns the camera's current Viewport/Zoom.
	State() State
	// Persisted returns pointers to the live window, zoom and fastening for saves and loads.
	Persisted() []any
	// Restore rebuilds derived state after a Load has written through Persisted's pointers.
	Restore()
	// SetMinZoom raises ZoomOut's floor above the automatic world-fit one; 0 keeps only that.
	SetMinZoom(minZoom float32)
	// SetMaxZoom caps ZoomIn — 0 (the default) leaves zoom-in unrestricted.
	SetMaxZoom(maxZoom float32)
}

// Config is how a camera starts: its window on the world, what it follows. Its pixels are whoever
// shows it's (a render.Feed sizes it every frame). It is construction-time only and never
// persisted: a loaded camera is as it was saved.
type Config struct {
	// Zoom is the scale it starts at, screen pixels a world unit; 0 is 1.
	Zoom float32
	// Whole keeps the whole world in view at whatever size the camera is shown, centred, the
	// background in bars along the longer side: a minimap. Such a camera is not panned or zoomed.
	Whole bool
	// Follow is the name of the entity it starts fastened Centred over (its entity.Label), found
	// once it is in the world; empty, it starts fastened to nothing.
	Follow string
	// MinZoom raises ZoomOut's floor above the automatic world-fit one; 0 keeps only that.
	MinZoom float32
	// MaxZoom caps ZoomIn; 0 leaves it unrestricted.
	MaxZoom float32
	// MouseLook has the eye, riding in an entity (Inside), look round with the mouse, the cursor
	// captured (MouseLooker); a camera without first person ignores it.
	MouseLook bool
}

// Fitting is a camera that may keep the whole world in view (Config.Whole): Fits says whether it
// does, and the world's size — the proportions a picture of it keeps as it is laid out.
type Fitting interface {
	Fits() (worldWidth, worldHeight float32, ok bool)
}

// State is a Camera's persistable visible window and zoom.
type State struct {
	Viewport AABB
	Zoom     float32
}
