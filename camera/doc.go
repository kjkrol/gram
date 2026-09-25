// Package camera is the view onto a world: screen conversion, culling, the visible window and its
// control. It is a root package, not a plugin; the world plugin builds one from its space and
// every renderer draws through it.
//
// # Camera
//
// A [Camera] converts between world and screen (ToScreen, FromScreen, ToScreenQuads, which splits
// a rectangle into the [Quad]s its wrapped images project to), culls (Visible, Bounds) and is
// controlled (MoveTo and Translate in world units, CenterOn a world point at a height in the middle
// of the screen, Pan in screen pixels — the same at any zoom —, SetViewport when the window is
// resized, keeping the middle and raising the zoom until the world covers the new screen —
// ZoomIn, ZoomOut, with min and max zoom). It keeps its own window
// arithmetic: wrapping on a wrapping axis of the world, held inside the world on any other.
// The cameras themselves live in internal/camera; a game gets one from the world plugin
// (world.Plugin.Camera, NewCamera), built from a [Config] with a viewport size, zoom limits and a
// projection.
//
// # Projection
//
// A [Projection] is the arithmetic a Camera draws through: [TopDown] (screen x and y are world x
// and y, the default) or [Isometric] (the 2:1 view of Transport Tycoon: a Cell-sized square is a
// TileW x TileH diamond, heights lift a point HeightUnit screen units per world unit). Every Camera
// exposes Project (a world point at a height), Unproject and Depth (further back is smaller), and
// ToScreen and FromScreen are the two at height 0; Viewport is the screen it draws to, in pixels. Config.Projection picks it; an Isometric camera
// keeps a screen window over the projected world instead of a world rectangle, refuses a wrapping
// world, and ToScreenQuads gives the rectangle round the diamond a world box projects to.
//
// # State
//
// [State] is the persistable part — the viewport and zoom — which the Camera hands to saves
// through Persisted and takes back through Restore. Config is construction-time only.
//
// # FromScreenRect
//
// [FromScreenRect] converts a screen rectangle to world space, for click and drag logic; on a
// torus the result may need wrapping.
package camera
