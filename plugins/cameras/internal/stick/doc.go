// Package stick is the cameras plugin's views of a world in relief: a camera on a stick, seen
// through orthographically or in perspective, an eye riding in a unit, and their control.
//
// [Maker] makes the world's cameras as a [Config] says, beginning as a [Start] says, [Over] a
// [Cage]; [Switch], [Turn] and [Tilt] do what Tab, Q and E, R and F do. A [Control]
// ([NewControl]) carries out the commands through its [Keeper],
// reading them as [Orders] and the entities as [Entities], and keeps every camera its
// [Control.Maker] made fastened behind or inside its entity as the camera's fastening says
// (camera.Fastening).
//
// # Views
//
// A camera hangs on a stick whose foot is the point in the middle of the screen: a pan slides the
// foot over the world at its height, Turn turns the stick about the upright through it, Tilt
// leans it from straight down (the map, a world unit a pixel at zoom 1) to MinPitch over the
// ground; the 2:1 view of Transport Tycoon (a cell a diamond, heights lifting what stands) is the
// stick turned an eighth, leaning at asin(TileH/TileW). The view is orthographic, Zoom screen
// units a world unit, or, given Config.Perspective, in perspective; switching keeps the foot, the
// turn, the pitch and the scale there, and the view is saved with the camera. Orthographically
// the whole screen stays over the world at sea level: a pan stops at the edge, and zooming out
// stops where the screen just fits the world. In perspective the eye stands at the stick's end,
// as far from the foot as the focal length over the scale it had; the wheel draws the eye and the
// foot towards the cursor, the stick no shorter than a cell nor than where the eye stays free,
// no longer than the world's diagonal. Where the cage is not free at the eye or a quarter of a
// cell under it, the spring lifts the eye straight up at once, still looking at the foot, and
// lets it down as the way clears. LookFrom puts the eye anywhere, LookAt the foot. On a world
// with a scale the ground far off sinks under the eye's level.
//
// Riding in a unit (fastened camera.Inside) the eye is the unit's world.Eye, going with it and
// looking the way it faces; Look turns the view and the unit, up and down it tilts the unit's
// Eye.Tilt (given back on letting go), Pan and Turn do nothing.
//
// Every camera is a camera.Picker: the point under a screen point is found along the line of
// sight, where the cage is first not free. Bounds and Visible
// hold the ground the screen may show at any height of the relief, not only at sea level.
//
// # Commands
//
// The commands carry the camera of whoever gave them, so the control knows no players; a camera
// of another view stays as it is. Ride takes a camera fastened Centred over its entity Behind it
// (centred and turned, eased, until the way it walks runs up the screen, as near as an arm two
// cells long in either projection, nothing zooming it, the tactical scale and eye height given
// back on letting go; in perspective its eye on the arm, shortened at once where the cage puts
// something solid between, eased back out), then Inside it where the game reaches the
// perspective, then over it again; a fastening changed, let go or its entity gone ends the
// keeping. The control writes only the eye's part of the unit's steering.Driven (Look, Flown,
// Climb) and clears it on letting go.
package stick
