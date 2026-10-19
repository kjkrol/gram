// Package camera is the view onto a world: screen conversion, culling, the visible window and its
// control. It is a root package, not a plugin; every renderer draws through a camera, and a game
// gets one from the cameras plugin, built from a [Config] through the maker it was given.
//
// # Camera
//
// A [Camera] converts between world and screen, culls, and is moved, panned and zoomed within the
// world: wrapping on a wrapping axis, held inside the world on any other. One keeping the whole
// world in view ([Config].Whole, a minimap) fits it at every size instead and is not panned or
// zoomed; a [Fitting] camera says so. Its pixels are whoever shows it's: a render.Feed sizes it
// every frame.
//
// # Projection
//
// A [Projection] is the arithmetic a Camera draws through: [TopDown] here, the cameras plugin's
// own in relief. Sorts says whether what is drawn through it must go back to front, Wraps whether
// a wrapping world can be drawn through it, Toward the way towards the eye.
//
// # State
//
// [State] is the camera's window and zoom. Persisted hands saves the window, the zoom and the
// fastening, which Restore takes back.
//
// # Fastening and what a camera says of itself
//
// A [Fastenable] camera is fastened to an entity: its [Fastening] says to which and [How], and
// [HowOf] is what the bindings hold in. The rest are small interfaces a camera may implement: a
// [Picker] finds what is under a screen point and its height; a [Scaler] draws a world unit
// larger near the eye than far off, and [ScaleAt] is the scale at a point through any camera; a
// [Rayer] says which way a screen point looks, [Rays] every line of sight at once as a [RayField],
// an [Eyed] camera where its eye stands and a [Vanisher] where a direction vanishes, a [Focused]
// camera the world point it observes, a [Headed] one the way along the ground that runs up its
// screen. A [Transform] is a RayField as the GPU draws it.
//
// # Detail
//
// A [Detailed] camera says how far it sees and how finely ([Detail]): its Sight from its
// [Viewpoint], the eye or else the point it observes, the last stretch of it in the fog, and the
// scale below which it draws coarsely; [DetailOf] is any camera's, none for one without. A
// [Resolved] camera is drawn at a resolution of the pixels it is shown at, its Viewport, Zoom and
// screen conversions in those camera pixels and its window on the world the same; [ResolutionOf]
// is any camera's, 1 for one without.
//
// # FromScreenRect
//
// [FromScreenRect] converts a screen rectangle to world space, for click and drag logic; on a
// torus the result may need wrapping.
package camera
