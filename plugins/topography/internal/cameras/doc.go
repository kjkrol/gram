// Package cameras is the views of a map in relief: the cameras from above, isometric and in
// perspective, an eye riding in a unit among them, and their control.
//
// [Maker] makes the world's cameras as a [Config] says; [Switch] turns a camera to the next
// view. A [Control] ([NewControl]) carries out the commands as its System runs, reading them as
// [Orders], and keeps every camera its [Control.Maker] made fastened behind or inside its entity
// as the camera's fastening says (camera.Fastening).
//
// # Views
//
// The world is seen from above, isometrically (the 2:1 view of Transport Tycoon, a cell a
// diamond, heights lifting what stands) or, given Config.Perspective, in perspective; switching
// keeps the ground point in the middle of the screen, the heading, the pitch and the scale
// there, and the view is saved with the camera. From above and isometrically the whole screen
// stays over the world at sea level: a pan stops at the edge, turning or bowing the view in
// relief by the part stopped (drawn back it lies down again as far as it bowed), the cursor in a
// corner of the window turns the view as Q and E do at a quarter of their pace, and zooming out
// stops where the screen just
// fits the world. The
// perspective is an eye flying over the world, never lower than two cells over its highest
// ground, keeping the ground point in the middle of the screen over the world; LookFrom and
// LookAt put the eye anywhere. On a world with a scale the ground far off sinks under the eye's
// level and hazes.
//
// Riding in a unit (fastened camera.Inside) the eye is the unit's world.Eye, going with it and
// looking the way it faces; Look turns the view and the unit, up and down it tilts the unit's
// Eye.Tilt (given back on letting go), Pan and Turn do nothing.
//
// Every camera is a camera.Picker: the ground under a screen point is found along the line of
// sight, over the top of the ground as it is drawn and the decks over it. Bounds and Visible
// hold the ground the screen may show at any height of the relief, not only at sea level.
//
// # Commands
//
// The commands carry the camera of whoever gave them, so the control knows no players; a camera
// of another view stays as it is. Ride takes a camera fastened Centred over its entity Behind it
// (centred and turned, eased, until the way it walks runs up the screen), then Inside it where
// the game reaches the perspective, then over it again; a fastening changed, let go or its
// entity gone ends the keeping. The control writes only the eye's part of the unit's
// steering.Driven (Look, Flown, Climb) and clears it on letting go.
package cameras
