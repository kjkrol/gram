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
// between its corners and raised by its kind's Height, the faces towards the viewer where it stands
// above its neighbour, all lit by the world's sun as the board says, the grid as each top's outline. Make it right after the world, before
// anything asks for a camera; a world that wraps cannot be seen this way.
//
// The view changes how things lie on the screen, not what the world is: heights are the world's
// (world.Config.Quasi3D) and are drawn from above too, as slopes and shadows of sight; the data a
// renderer draws stays its plugin's, and only the plugin's Look — world.Look, board.Look — is this
// package's.
package isometry
