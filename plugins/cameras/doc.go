// Package cameras makes the cameras a game looks at its world through and moves them.
//
// # Making cameras
//
// [NewPlugin] takes the world and makes no camera: each is made where it is given to a player
// ([Plugin.New]), with a [Maker] and a camera.Config of its own — one player looking from above,
// another isometrically, one looking round with the mouse, another not; a minimap's the same way.
// The zero Config sees the window's size at zoom 1. [TopDown] is the plain camera from above over
// a flat world — wrapping on a wrapping axis, held inside the world on any other; in a world in
// relief the cameras are the view plugin's (topography.Plugin.Views: starting from above or
// isometrically, Tab going round the views), whose drawing asks for their lines of sight. A local
// player looks through the camera it is given (players.Plugin.Local). Every camera made is saved
// with the game, in the order made.
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
// fastened Behind or Inside an entity is the view plugin's to keep (plugins/topography). [LookAt]
// moves a camera once to have an entity in the middle of the screen, fastened to nothing: what a
// window about a place far off gives to show the place (ui).
// [MouseLook] switches whether a camera riding inside an entity looks round with the mouse, the
// cursor captured (camera.MouseLooker; camera.Config.MouseLook says it at the start, off by
// default); [MouseLookKey] binds it to a key.
//
// # Keys
//
// [Keys] are a player's keys to its camera — four keys held scrolling it on the screen, the wheel
// zooming about the cursor, the middle drag panning, the cursor at an edge scrolling — and
// [DefaultKeys] W, S, A, D with all three; [Plugin.DefaultBindings] are those. A game that wants
// other keys, or several players at one keyboard, binds its own Keys.
package cameras
