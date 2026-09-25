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
// navigation.MoveTo). The plugin that defines a command's type handles it: it keeps a [Queue] of
// it and drains it in its own pass ([Queue.Drain], every [Issued] with the [PlayerID] that gave it,
// [Nobody] for none); [CommandQueue] is a Queue with the type erased, as a carrier sorts commands
// into them. A [Binding] is a [Trigger] — [KeyPress], [KeyHeld], [ButtonPress], [Drag], [Wheel], [ButtonHeld],
// [CursorAtEdge], each with exactly its [Mods] — the command [Command] builds from a [Context] (the
// player, its camera, the cursor, a drag's start and end, World and WorldBox through the camera and,
// over Ground, on the ground: a click on a hill lands on the hill)
// and a label for a help screen. plugin.CommandHandler is what defines and carries out commands,
// plugins/players what brings them.
package control
