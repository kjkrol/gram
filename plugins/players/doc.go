// Package players is whoever acts in the game: a [Player] with a camera, a View through it and —
// at this keyboard — the bindings that turn its input into commands, carried to the plugin that
// defined each one.
//
// # A carrier to the command handlers
//
// [NewPlugin] takes the world and every plugin.CommandHandler the game uses (selection, navigation,
// a game with commands of its own): the world carries their queues — its one carrier of commands,
// which the entities give theirs to as well (world.Plugin.Commands) — and the players gather
// their default bindings ([Plugin.Defaults]). The plugins never know players; they know plugin.CommandHandler and
// the vocabulary in package control. [Plugin.Issue] is how a command comes in — from a binding, an
// AI, a network — and a type no handler defines is [ErrUnknownCommand]. Players' own commands are
// [Pan] and [Zoom], carried out on the issuing player's camera; [CameraBindings] are their
// defaults: the wheel zooms, a middle drag pans, W, A, S and D held and the cursor at an edge
// scroll — on the screen, so in a turned isometric view along the screen too.
//
// # Owners
//
// A unit belongs to the player it was given to: the command [Give], which the unit gives itself
// as it is made (kind.Entry.Told) or later — captured, converted — sets its one owner, a tag of
// the owners' family (plugins/players/owner) every unit carries. [NewPlugin] registers the family with the world's kinds, a tag a player,
// saved by name. The plugins that take commands read the tag through the leaf package owner
// (owner.Obeys), never through this plugin: a player selects, orders and rides its own units
// alone; a unit nobody owns belongs to the virtual player control.Nobody, whom the game's code, a
// script or an AI run as nobody speaks for. A side of its own — the wild, a rival — is a player
// without a keyboard ([Plugin.Add]) owning its units.
//
// # Viewports
//
// A player looks at the world through its camera, in its part of the screen. [Plugin.Viewports] is
// what a Scene showing the world gives the engine as its game.Viewer: one viewport per camera the
// local players look through, side by side in equal columns, the world's camera over the whole
// screen when nobody is at this keyboard. What is drawn is the Scene's to say — its layers, the
// commands' handlers' renderers among them; players draw nothing.
//
// # Players
//
// [Plugin.Local] adds a player at this keyboard, looking through the world's camera; [Plugin.Add]
// one without a keyboard — an AI, a remote client — whose commands come through Issue. A game
// binds a player with [Player.Bind]: the Defaults whole, single entries of its own, or fewer. Two
// bindings on one rule holding in one camera mode are refused at Bind; a binding whose command
// nobody defines is refused when the Stage is set up. [Player.Bindings] is the list a help screen
// draws. A binding may hold in some camera modes only (control.Binding.In, camera.ModeOf): the
// camera's own WASD, middle drag and edge scroll hold while the camera is Free, and a camera riding
// in an entity (camera.FirstPerson) leaves those keys to the plugin that steers the entity; only
// the bindings holding in the camera's mode fire, and the shortcuts list only those. While a local
// player's camera rides, the window's cursor is captured and control.CursorMove reaches that player
// wherever the cursor is — looking round with the mouse; the pass it is caught or let go no move
// is taken.
//
// # From input to commands
//
// [Plugin.EventHandler] is the layer between the device and the game. The Scene hands it each
// pass's control.InputEvents; it keeps, per local player, what it has seen of the keys and buttons
// (the cursor in the player's part of the screen, the buttons down and where they went down, the
// keys held), matches the events against the player's bindings' control.Rules, builds each
// matching binding's command from a control.Context (Binding.Build) and hands it to [Plugin.Issue],
// which puts it in the queue of the handler that defined its type. A control.KeyHeld is the one
// rule fired from [Plugin.RunPlan] instead, once a tick while its key is down.
//
// # Split screen
//
// [Player.OwnCamera] gives a player a camera of its own over the world (world.Plugin.NewCamera),
// saved with the game ([Plugin.Serializable], [Plugin.Restore]); call it before Use. Local players
// with cameras of their own share the screen as [Plugin.Viewports] lays it out ([Columns], or a
// [Layout] given WithLayout), and each keeps its part, [Player.Area]. Every key reaches every local
// player, each with bindings of its own — WASD for one, the arrows for another, a
// control.KeyHeld firing once a tick while its key is down — and the mouse, there being one,
// reaches the player whose part of the screen it is over, in the pixels of that part.
//
// # Order within a tick
//
// The active Scene hands the tick's input to [Plugin.EventHandler], which runs every local
// player's bindings and fills the queues; the command handlers drain theirs in their RunPlan;
// [Plugin.RunPlan], called last, carries out Pan and Zoom. Nothing is dropped: a command given
// after its handler's pass — by an entity's rule in a later plugin's — waits for the next
// frame's.
//
// # Scene keys and the shortcuts
//
// A scene showing the world hands its input to [Plugin.Handle] and nothing else: the bindings
// turn it into commands, and the two of the players' own that need the engine are carried out
// there — [Quit] (Shift+Esc), [ShowShortcuts] (K) and, in a game that said where it saves
// ([Plugin.WithSaves]), [Save] (F5): default keys like the camera's, the same in every game. Keys
// that are no command to a plugin — a debug toggle — are the game's own: [SceneKeys] lists them
// with labels and what they do, given to [Plugin.OwnKeys], and Handle runs them. [Shortcuts] is
// the players' own scene listing every key of the game — the local players' bindings, grouped by
// the plugin whose command each issues, and under "Game" the game's own keys, Quit and the list's
// — over the dimmed screen, the game held in the engine's pause while it is up; every Stage that
// uses the players has it in its stack ([Plugin.Scenes], game.Scenic), and Esc or K closes it.
// [Written] is a rule as such a list writes it.
//
// # Commands
//
// The players' own commands are in commands.go: [Pan] and [Zoom] move a player's camera, [Give]
// makes an entity a player's, [Quit] ends the game, [ShowShortcuts] opens the list of keys and
// [Save] writes the game. [GameBindings] are the keys of the last three's first two, for a game
// that binds its own camera keys.
package players
