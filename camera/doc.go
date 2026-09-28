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
// plugins/topography keeps the isometric projection and camera private to itself. Every Camera
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
// # Modes and vanishing points
//
// A camera may say more of itself through small interfaces: a [Rider] rides in an entity, which
// [ModeOf] reads as [FirstPerson] — the Mode bindings hold in (control.Binding.In) — and a
// [Vanisher] has vanishing points, where a direction is drawn: a perspective's, where the sky puts
// the sun. An [Eyed] camera has an eye at a point, from which the air far off is hazed. A
// [Picker] finds the ground under a screen point itself, walking the line of sight over the
// heights it draws, which control.Context.World asks before anything else. A
// [Scaler] draws a world unit larger near the eye than far off, and nothing behind it;
// [ScaleAt] is the scale at a point through any camera — a Scaler's own, else the Zoom — which is
// what sizes what is drawn where it lies: detail, the grid, soft edges, billboards.
//
// # FromScreenRect
//
// [FromScreenRect] converts a screen rectangle to world space, for click and drag logic; on a
// torus the result may need wrapping.
package camera
