// Package selection turns Select commands into a Selected tag on Selectable world entities. Its
// default bindings make a left drag one (Shift adds) and C has the camera follow the one selected
// unit; WithRenderer outlines what is selected.
//
// # Selectable, Selected and Hovered
//
// [Tags] are bits of selection's tag [Family], which every unit carries, all off: Selectable
// marks an entity the player may select, told [Allow] as it is made (kind.Entry.Told) or later and
// taken back by [Forbid]; Selected one the player has selected; Hovered the one the cursor points
// at, given by a [Hover] every tick the cursor lies over the world. [Plugin.Tags] is for the
// plugins reading them; a drawing rule asks [Plugin.IsSelected]. A [Select] names entities by id
// or by a box, additive or not, and with a Camera the screen rectangle the player drew picks them
// where that camera draws them; [SelectionSystem] carries it out.
//
// # Whose units
//
// A player selects its own units alone (owner.Obeys), so one Selected tag serves every player;
// only a Select nobody gave reaches the units nobody owns. [Plugin.FollowKey] (C by default)
// fastens the camera the player looks through over the one unit it has selected
// ([Plugin.Chosen]).
//
// # Selected and Pointed
//
// A command about an effect (rule.Put, Take, Toggle) says whom it is for with On, and the
// selection carries out two: [Plugin.Selected], the units the player who gives it has selected,
// those playing one of the roles named when any is, and [Plugin.Pointed], the entity drawn under
// the cursor as the command's key is pressed.
//
//	hasten := rule.Put(haste).On(s.selection.Selected(hasty))
//	freeze := rule.Put(frozen).On(s.selection.Pointed()).For(3 * time.Second)
//
// # Renderer
//
// [Plugin.WithRenderer] builds the [Renderer], a render.Source outlining every Selected entity on
// the render.Marks tier in a [HighlightStyle] ([DefaultHighlightStyle] a thin red outline), and
// the box of a selection being dragged ([Marquee]), one per camera.
package selection
