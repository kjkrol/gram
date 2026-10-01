// Package selection turns Select commands into a Selected tag on Selectable world entities;
// its default bindings make a left drag one (a click is a drag of no length, Shift adds), and F
// has the camera follow the one selected unit. WithRenderer outlines what is selected.
//
// # Selectable, Selected and SelectionSystem
//
// [Tags] are two bits of selection's tag [Family], from [Plugin.Tags]: Selectable marks an
// entity the player may select — a unit, not a stretch of terrain; give it with comp.Tagged —
// and Selected one the player has selected, flipped in place. A [Select] names entities by id or
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
// code, a script, an AI run as nobody — and only a Select nobody gave reaches them. Follow takes
// the one selected unit of the player who asked, and an [Apply] puts its effect — an ability, a
// sprint, a spell — on what the player who gave it has selected.
//
// # Followed and FollowSystem
//
// The third tag, Followed, is the unit a camera follows. A [Follow] command (C by default) tags
// the one Selected unit — none with several selected — or, when one is followed already, untags
// it; its Camera is the one of the player who asked. Every tick the [FollowSystem] centres that
// camera on the followed unit at its
// altitude (camera.Camera.CenterOn). A player who moves the camera by hand ends the following: at
// an unchanged zoom, the point centred the tick before is drawn elsewhere; zooming does not.
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
