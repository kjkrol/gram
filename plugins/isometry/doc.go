// Package isometry is the isometric view of a world, as a plugin a game adds or leaves out: without
// it everything is drawn from above, with it through the 2:1 view of Transport Tycoon.
//
// [NewPlugin] takes the world and the view's [Config] — a cell's size in the world and on the
// screen, how far a unit of height lifts a point — and puts the world in it at once: the world makes
// its cameras through the plugin's own isometric projection and camera, private to it (its own
// camera anew, every player's after it), and lays
// its entities as billboards standing upright on their centres at their altitudes, at the depth of
// that centre, which a render.Composer sorts by; picking and the selection's outline follow, since
// they ask the world's Look. [Plugin.WithBoard] lays the board's cells as blocks: each top sloped
// between its corners and raised by its kind's Height, its top leant with the wind if its kind sways, the faces turned towards the viewer where it stands
// above its neighbour, all lit by the world's sun as the board says, the grid as each top's outline. Make it right after the world, before
// anything asks for a camera; a world that wraps cannot be seen this way.
//
// The camera turns by any angle: the projection turns the ground round the vertical, the world
// clockwise on the screen as the heading grows, keeping the ground point in the middle of the
// screen; the depth a render.Composer sorts by is how far down the screen the middle of a cell
// lies, the blocks show whichever faces look towards the eye, and the heading is saved with the
// camera.
//
// # Commands
//
// The plugin is a plugin.CommandHandler, its commands carrying the camera of whoever gave them
// (control.Context.Camera), so it turns a player's camera without knowing players; a camera of
// another view stays as it is. [Turn] turns the view (Q and E held, [TurnStep] a tick); [Tilt] has
// it look down more or less steeply (PageUp and PageDown, [TiltStep] a tick), from ten degrees over
// the ground to straight down, the 2:1 view looking down at asin(TileH/TileW). [Follow],
// given the selection ([Plugin.WithSelection], whose Selected tag it reads as navigation does),
// fastens the camera behind the one selected unit (V): every tick the camera is centred on it at
// its altitude and turned, eased, until the way it walks — world.Base's Vel.Dir, kept when it
// stops — runs up the screen. It holds whatever else is done — other units selected and ordered,
// the camera panned or turned — until V again lets it go, or the unit is gone; the lower the eye,
// the lower on the screen the unit stands, over its shoulder. [Drive] (the arrows) steers the unit
// a camera is fastened to: the camera system keeps a world.Driven on it while fastened, writes the
// keys into it every tick and stops it when let go; navigation carries it out on the ground. That
// is a game walked behind a character's back as much as a map turned round. Call [Plugin.RunPlan] after the
// world has moved and before the players' RunPlan.
//
// The view changes how things lie on the screen, not what the world is: heights are the world's
// (world.Config.Quasi3D) and are drawn from above too, as slopes and shadows of sight; the data a
// renderer draws stays its plugin's, and only the plugin's Look — world.Look, board.Look — is this
// package's.
package isometry
