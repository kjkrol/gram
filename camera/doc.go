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
// (world.Plugin.Camera, NewCamera), built from a [Config] with a viewport size and zoom limits,
// through whichever projection the world's view gives it.
//
// # Projection
//
// A [Projection] is the arithmetic a Camera draws through; this package has [TopDown] (screen x and
// y are world x and y, height is not drawn), and a view plugin brings its own with its cameras —
// plugins/isometry keeps the isometric projection and camera private to itself. Every Camera
// exposes Project (a world point at a height), Unproject and Depth (further back is smaller), and
// ToScreen and FromScreen are the two at height 0; Viewport is the screen it draws to, in pixels.
// Sorts says whether what is drawn through it must go back to front, which a render.Composer asks;
// Wraps whether a world wrapping at its edges can be drawn through it; Toward the way towards the
// eye, which a glint needs.
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
