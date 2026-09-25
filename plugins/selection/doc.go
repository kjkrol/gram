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
// # Followed and FollowSystem
//
// The third tag, Followed, is the unit a camera follows. A [Follow] command (F by default) tags
// the one Selected unit — none with several selected — or, when one is followed already, untags
// it; its Camera is the one of the player who asked. Every tick the [FollowSystem] centres that
// camera on the followed unit at its
// altitude (camera.Camera.CenterOn). A player who moves the camera by hand ends the following: at
// an unchanged zoom, the point centred the tick before is drawn elsewhere; zooming does not.
//
// # Renderer
//
// [Plugin.WithRenderer] builds the [Renderer] outlining every Selected entity in a
// [HighlightStyle] ([DefaultHighlightStyle] is a thin red outline; [HighlightStyleFn] adapts a
// function); the marquee of a drag in progress is the players plugin's to draw.
package selection
