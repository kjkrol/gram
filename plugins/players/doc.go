// Package players is whoever acts in the game: a [Player] acting through a picture of the world a
// scene wires it to and, at this keyboard, the bindings that turn its input into commands,
// carried to the plugin that defined each one.
//
// # A carrier to the command handlers
//
// [NewPlugin] takes the world and every plugin.CommandHandler the game uses: the world carries
// their queues and the players gather their default bindings ([Plugin.Defaults]). The plugins
// never know players; they know plugin.CommandHandler and the vocabulary in package control.
// [Plugin.Issue] is how a command comes in, from a binding, an AI or a network; a type no handler
// defines is [ErrUnknownCommand].
//
// # Owners
//
// A unit belongs to the player it was given to: the command [Give], which the unit gives itself
// as it is made (kind.Entry.Told) or later, sets its one owner, a tag of the owners' family
// (plugins/players/owner) every unit carries. The plugins that take commands read the tag through
// owner.Obeys: a player selects, orders and drives its own units alone; a unit nobody owns is
// control.Nobody's. A side of its own, the wild or a rival, is a player without a keyboard
// ([Plugin.Add]) owning its units.
//
// # Pictures of the world
//
// A player owns no camera: it acts through a picture on a ui scene's screen, which the scene
// makes with a camera beside it. ui.Image(render.NewFeed(cam, picture)).Input([Plugin.Through](pl))
// is the wire: the active scene tells the player before each pass of input where its picture lies
// ([Player.Area]) and what it shows, so the mouse over it is the player's and every command it
// gives carries that picture's camera (control.Context.Camera). A scene gives a player one view:
// two of its pictures in one pass panic. [Plugin.IssueAs] is how the scene's buttons and keys give
// their commands as the player. Players draw nothing.
//
// # Players and bindings
//
// [Plugin.Local] adds a player at this keyboard, [Plugin.Add] one without. A game binds a player
// with [Player.Bind]: the Defaults whole, or entries of its own. Two bindings on one trigger
// holding in one fastening of the camera are refused at Bind; one whose command nobody defines
// when the Stage is set up. A binding may hold in some fastenings of the camera only
// (control.Binding.In): only those holding in the camera's How fire. A player holds a tool in
// hand (control.Take, control.Drop; [Player.Tool]): while it does, the bindings With that tool
// fire in place of those with no tool. While a local player's camera rides looking round with the
// mouse, the window's cursor is captured and control.CursorMove reaches that player.
//
// # From input to commands
//
// [Plugin.EventHandler] is the layer between the device and the game: it matches each pass's
// control.InputEvents against every local player's bindings and hands what they build to
// [Plugin.Issue]. Keys reach every local player, the mouse the one whose picture it is over, in
// that picture's pixels, so local players split the screen as the scene lays their pictures out.
// A control.KeyHeld fires from [Plugin.RunPlan] instead, once a tick while its key is down; call
// RunPlan last in Update. Nothing is dropped: a command given after its handler's pass waits for
// the next frame's.
//
// # Scene keys and the shortcuts
//
// A scene showing the world hands its input to [Plugin.Handle] and nothing else. [ShowShortcuts]
// (K) opens [Shortcuts], the players' own scene listing every key of the game, over the paused
// game; every Stage that uses the players has it in its stack ([Plugin.Scenes]), and Esc or K
// closes it. Keys that are no command to a plugin are the game's own: [SceneKeys], given to
// [Plugin.OwnKeys], run by Handle and listed under "Game" with what the Stage binds in its
// Controls section. [GameBindings] are K, F5 game.Save, F9 game.Load and Shift+Esc game.Quit, the
// same in every game. [Written] is a trigger as the list writes it.
package players
