// Package control is the input vocabulary: what the engine captures each frame and what a
// handler is told.
//
// # InputEvents
//
// [InputEvents] is one tick's input: the mouse position and its delta, which modifiers and the
// middle button are down, whether the window fills the screen, the scroll delta, and a queue of
// [ClickEvent]s and [KeyEvent]s, each a key or button with a [KeyAction] (press or release).
// The engine fills it from the platform and clears the transient part (ResetTransient) after
// every tick.
//
// # EventHandler
//
// [EventHandler] reacts to this tick's input. The engine calls the active Scene's HandleEvents
// once per tick, after capture and before the Stage's Update; the players plugin's EventHandler,
// which turns input into commands, runs the same way.
//
// # Commands and bindings
//
// A command is an intention in the game's vocabulary, as data (selection.Select,
// navigation.MoveTo): a type is one by its method Command, and every command type says so where it
// is defined — var _ control.Command = MoveTo{} — so only a [Command] goes where commands go: a
// Queue, a Binding, a carrier, an entity's own Order, a button. Some commands have a trait of their
// own besides, asked for where it matters: [Routed], [Contextual], plugin.Aimed, ui.About. The
// plugin that defines a command's type handles it: it keeps a [Queue] of
// it and drains it in its own pass ([Queue.Drain], every [Issued] with the [PlayerID] that gave it,
// [Nobody] for none — or, ByEntity, the entity that gave it itself); [CommandQueue] is a Queue
// with the type erased, as a carrier sorts commands into them, and [Carrier] is one: the world's
// takes the players' commands and those its entities give themselves (Order in a rule or a
// plan) to the queues of their types. A [Binding] is a [Trigger] — [KeyPress], [KeyHeld],
// [ButtonPress], [Drag], [Wheel], [ButtonHeld], [CursorAtEdge], [CursorMove], [CursorOver], each
// with exactly its [Mods] — the command [Build] makes from a [Context] or [Give] names (the
// player, its camera, the cursor, a drag's start and end, World and WorldBox through the camera —
// a camera.Picker's own pick of the ground, else, over Ground, on the ground: a click on a hill
// lands on the hill — and Point, what was picked and its height: a bridge's deck, or the ground
// under it) and a label for a help screen. plugin.CommandHandler is what defines and carries out
// commands, plugins/players what brings them.
//
// # Tools
//
// A [Tool] is what a player holds in hand — a bridge to build — taken with [Take] and put down
// with [Drop], commands the players plugin carries out. A binding With a tool holds only while
// the player holds it and takes its trigger from the bindings with none: a left click lays a
// bridge's head while the player holds the bridge, and selects while it holds nothing.
package control
