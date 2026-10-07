// Package cameras makes the cameras a game looks at its world through and moves them.
//
// # Making cameras
//
// [NewPlugin] takes the world, a [Maker] and the camera.Config every camera is made with: the
// zero Config sees the window's size at zoom 1, sized to it as the plugin is installed. [TopDown]
// is the plain camera from above over a flat world — wrapping on a wrapping axis, held inside the
// world on any other; a view plugin gives its own (topography.Plugin.Views: from above,
// isometric, in perspective). The plugin makes the main camera at once ([Plugin.Main]) and any
// other on demand ([Plugin.New]): a second player's in a split screen, a minimap's. A local player
// looks through the camera it is given (players.Plugin.Local), so a game says which camera is
// whose. Every camera made is saved with the game, in the order made.
//
// # Commands
//
// [Pan] moves a camera by screen pixels and lets go of whatever it was fastened to; [Zoom] scales
// it about a world point; [Follow] fastens it Centred over an entity — kept in the middle of the
// screen as it goes, let go once the entity is gone — or lets it go. Each names its camera: a
// binding takes it from the context (control.Context.Camera), so the plugin never knows the
// players. Given by an entity for itself (kind.Entry.Told), a Follow fastens the camera it names
// over that entity from its first step. Who is followed is whoever builds the Follow: the
// selection's key (selection.Plugin.FollowKey) follows the one unit the player has chosen. A camera
// fastened Behind or Inside an entity is the view plugin's to keep (plugins/topography).
//
// # Keys
//
// [Keys] are a player's keys to its camera — four keys held scrolling it on the screen, the wheel
// zooming about the cursor, the middle drag panning, the cursor at an edge scrolling — and
// [DefaultKeys] W, S, A, D with all three; [Plugin.DefaultBindings] are those. A game that wants
// other keys, or several players at one keyboard, binds its own Keys.
package cameras
