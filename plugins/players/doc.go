// Package players is whoever acts in the game: a [Player] with a camera, a View through it and —
// at this keyboard — the bindings that turn its input into commands, carried to the plugin that
// defined each one.
//
// # A carrier to the command handlers
//
// [NewPlugin] takes the world and every plugin.CommandHandler the game uses (selection, navigation,
// a game with commands of its own): it gathers their queues by command type and their default
// bindings ([Plugin.Defaults]). The plugins never know players; they know plugin.CommandHandler and
// the vocabulary in package control. [Plugin.Issue] is how a command comes in — from a binding, an
// AI, a network — and a type no handler defines is [ErrUnknownCommand]. Players' own commands are
// [Pan] and [Zoom], carried out on the issuing player's camera; [CameraBindings] are their defaults.
//
// # Viewports
//
// A player looks at the world through its camera, in its part of the screen. [Plugin.Viewports] is
// what a Scene showing the world gives the engine as its game.Viewer: one viewport per camera the
// local players look through, side by side in equal columns, the world's camera over the whole
// screen when nobody is at this keyboard. What is drawn is the Scene's to say — its layers, the
// commands' handlers' renderers among them; players draw only the marquee, in its player's view.
//
// # Players
//
// [Plugin.Local] adds a player at this keyboard, looking through the world's camera; [Plugin.Add]
// one without a keyboard — an AI, a remote client — whose commands come through Issue. A game
// binds a player with [Player.Bind]: the Defaults whole, single entries of its own, or fewer. Two
// bindings on one trigger are refused at Bind; a binding whose command nobody defines is refused
// when the Stage is set up. [Player.Bindings] is the list a help screen draws, [Player.DragBox] the
// drag in progress, which [Plugin.WithRenderer] draws as a marquee.
//
// # Order within a tick
//
// The active Scene hands the tick's input to [Plugin.EventHandler], which runs every local
// player's bindings and fills the queues; the command handlers drain theirs in their RunPlan;
// [Plugin.RunPlan], called last, carries out Pan and Zoom and empties whatever is left. Keys that
// are not a move in the game — pause, quit, a debug toggle — stay in the Scene's HandleEvents.
package players
