// Package control is the input vocabulary: what the engine captures each frame, and the commands
// and bindings a game is driven by.
//
// # InputEvents
//
// [InputEvents] is one tick's input: the mouse, the modifiers, the scroll delta and a queue of
// [ClickEvent]s and [KeyEvent]s. An [EventHandler] reacts to it once a tick; the active Scene's
// HandleEvents is one.
//
// # Commands and bindings
//
// A command is an intention in the game's vocabulary, as data: a type is a [command.Command] by its
// method command.Command, confirmed where it is defined (var _ command.Command = MoveTo{}). The plugin
// that defines a command's type keeps a [Queue] of it and drains it in its own pass, each
// [command.Issued] with the [command.Issuer] that gave it, or the entity that gave it itself; a [command.Carrier] sorts
// commands into the queues of their types. Traits a command may have besides are asked for
// where they matter: [command.Routed], [Contextual]. A [Binding] is a [Trigger] ([KeyPress],
// [KeyDown], [KeyHeld], [KeyReleased] — let go, [Context].Held how long it was down —
// [ButtonPress], [Drag], [Wheel], [ButtonHeld], [CursorAtEdge] — the cursor within [EdgeMargin]
// of the picture's edge, or in a corner's square of [CornerReach] a side, [CornerSide] for a
// small picture ([Edges], [Corner]) — [CursorMove], [CursorOver]) with exactly its [Mods], the
// command [Build] makes from a [Context] or [Give] names, and a label for the list of keys; a key
// held, down or let go fires with Shift, Ctrl or Alt held only [Binding.InChords]. A
// Context carries the player, its camera, the cursor and the point picked in the world through the
// camera.
//
// # Tools
//
// A [Tool] is what a player holds in hand, taken with [Take] and put down with [Drop]. A binding
// With a tool fires only while the player holds it, in place of the bindings with none.
package control
