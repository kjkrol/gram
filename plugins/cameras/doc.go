// Package cameras makes the cameras a game looks at its world through and moves them.
//
// # Making cameras
//
// [NewPlugin] takes the world and makes no camera: each is made where a scene shows it
// ([Plugin.New], render.NewFeed), with a [Maker] and a camera.Config of its own, and sized by
// whoever shows it. [TopDown] is the plain camera from above over a flat world; in a world in
// relief the cameras are the view plugin's (topography.Plugin.Views). camera.Config.Follow names
// the entity a camera starts fastened Centred over. Every camera made is saved with the game, in
// the order made, and one made after a Load takes the state of the camera made in its place.
//
// # Commands
//
// [Pan] moves a camera by screen pixels and lets go of whatever it was fastened to; [Zoom] scales
// it about a world point; [Follow] fastens it Centred over an entity, kept in the middle of the
// screen as it goes, or lets it go; [LookAt] moves it once onto an entity, fastened to nothing.
// Each names its camera, taken from the context by its binding, so the plugin never knows the
// players. A camera riding Inside an entity looks round with the mouse, the cursor captured. A camera fastened Behind or Inside an entity is the view
// plugin's to keep.
//
// # Keys
//
// [Keys] are a player's keys to its camera: four keys held scrolling it, the wheel zooming about
// the cursor, the middle drag panning, the cursor at an edge scrolling. [DefaultKeys] are W, S,
// A, D with all three, and [Plugin.DefaultBindings] are those; a game wanting other keys, or
// several players at one keyboard, binds its own Keys.
//
// # Vocabulary
//
// Commands: [Pan], [Zoom], [LookAt] (a player's through its camera); [Follow] (a player's, or an
// entity's own fastening the camera over itself).
package cameras
