// Package selection turns Select commands into a Selected tag on Selectable world entities;
// its default bindings make a left drag one (a click is a drag of no length, Shift adds), and C
// has the camera follow the one selected unit. WithRenderer outlines what is selected.
//
// # Selectable, Selected and SelectionSystem
//
// [Tags] are bits of selection's tag [Family], which every unit carries, all off: Selectable
// marks an entity the player may select — a unit told [Allow], as it is made (kind.Entry.Told) or
// later; [Forbid] takes it back — and Selected one the player has selected, flipped in place.
// [Plugin.Tags] is for the plugins reading them; a drawing rule asks [Plugin.IsSelected]. A [Select] names entities by id or
// by a box in world units, additive or not, and with a Camera the screen rectangle the player
// drew picks entities where that camera draws them; the plugin is a plugin.CommandHandler — [Plugin.Queues]
// is the queue, [Plugin.DefaultBindings] a left drag through the player's camera — and the
// [SelectionSystem] drains the queue into Selected tags.
//
// # Whose units
//
// A player selects its own units alone: a Select, carried with the player who gave it
// (control.Issued), hits and unselects only the Selectable entities that player owns
// (plugins/players/owner.Obeys), so another player's selection stays as it is and one Selected tag
// serves every player. Units nobody owns belong to the virtual player control.Nobody — the game's
// code, a script, an AI run as nobody — and only a Select nobody gave reaches them.
// [Plugin.FollowKey] (C by default) fastens the camera the player looks through over the one unit
// it has selected ([Plugin.Chosen]): a cameras.Follow the selection builds, so the cameras plugin
// never knows the selection.
//
// # Selected and Pointed
//
// A command about an effect (rule.Cast, Lift, Toggle) says whom it is for with On, and the
// selection knows two: [Plugin.Selected], the units the player who gives the command has
// selected — those playing one of the roles named, when any is, so with scouts and porters
// selected a haste for the hasty goes to the scouts alone — and [Plugin.Pointed], the entity drawn
// under the cursor as the command's key is pressed. The selection carries such commands out.
//
//	hasten := rule.Cast(haste).On(s.selection.Selected(hasty))
//	freeze := rule.Cast(frozen).On(s.selection.Pointed()).For(3 * time.Second)
//	s.player.Bind(control.Give(control.KeyPress{Key: control.KeyJ}, "Hasten the selected scouts", hasten))
//
// # Hovered
//
// Every tick the cursor lies over a player's picture of the world (control.CursorOver, among the
// default bindings) it gives a [Hover], and the entity drawn under it, picked as Pointed picks,
// carries Hovered until the next tick: what a ui element pinned Where the tag is carried stands by
// (ui.Tagged). One tag serves every player: two cursors over a split screen hover two entities.
//
// # Renderer
//
// [Plugin.WithRenderer] builds the [Renderer], a render.Source outlining every Selected entity on
// the render.Marks tier, over everything, in a [HighlightStyle] ([DefaultHighlightStyle] is a thin
// red outline; [HighlightStyleFn] adapts a function) round its footprint as the world's Look lays
// it, and the box of a selection being dragged in the viewport's camera. A click picks what the
// world's Look draws under the cursor. The box comes from
// the [Marquee] command, issued by a control.ButtonHeld of the left button while the drag lasts and
// hidden by the Select that ends it, one per camera, so a player on a split screen sees only its own.
package selection
