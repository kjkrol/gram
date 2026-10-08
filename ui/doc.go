// Package ui composes a scene's screen out of elements: panels, labels, windows, buttons and
// pictures of the world, laid out in a tree. It knows no cameras: the world seen through a camera
// is a render.Feed, which ui shows as any other picture.
//
// # Scene
//
// A [Scene] is a game.Scene whose screen is a tree of elements: [NewScene] takes its name and the
// tree, made once the world is there (the Stage's Scenes) — the pictures of the world, a Composer
// of the plugins' renderers as a rule, and the cameras they are seen through made beside the
// elements that show them; each picture its feeds show is initialised once however many show it.
// Every frame the tree is laid over the screen and drawn. [Scene.Input] hands the
// scene's input to the players' bindings; [Scene.Show], [Scene.Hide] and [Scene.Toggle] show and
// hide elements by name ([Element.Named]), [Scene.Element] finds one, [Scene.Shown] asks.
//
// # Layout
//
// An [Element] is given a box by its parent. [Layers] lays its elements over the whole box from the
// bottom up, each covering those before it; [Columns] and [Rows] split it, none covering another,
// each element taking its [Part]: a [Share] of what is left, [Fixed] pixels, or what it needs
// ([Fit]). An anchor wraps an element and places it at its point of the box — [TopLeft],
// [TopMiddle], [TopRight], [MiddleLeft], [Center], [MiddleRight], [BottomLeft], [BottomMiddle],
// [BottomRight] — at the [Element.Size] asked for, or the [Element.Fraction] of the box, kept
// [Element.Margin] pixels off the edges. A Fraction keeps its share as the window changes; an axis
// asked for as 0 follows the proportions of the world a feed shows whole (camera.Config.Whole,
// render.Feed.Proportions): a minimap a fifth of the screen wide, as high as the world's shape says.
//
// # Elements
//
// [Panel] is a background and a border round an element, padded; [Label] a line of text; [Image] a
// render.Surface — a feed of the world, a picture — filling its box, the box's size given to it;
// [Window] a panel with a title over its elements, one under another, which [Element.Modal] makes
// hold the input while it is shown. Any element takes a background ([Element.Fill]), a border
// ([Element.Border]) and [Element.Padding].
//
// [LabelOf], [ButtonOf] and [WindowOf] read their words off a [Text] every frame, for the entity
// a pinned element is shown for: a name over a unit, a conversation's line. A Text saying nothing
// leaves its element out — it takes no room in a split and nothing hits it.
//
// # Theme
//
// A [Theme] is how a scene's elements look where they say nothing of their own: the font and the
// colour of their text (render.DefaultFont, Polish letters and all), and the colours of panels,
// windows' titles and buttons. [Scene.Theme] sets the scene's; an element's own Fill or Border wins.
//
// # Input
//
// A click goes to the topmost element it hits. A [Button] gives its command: one of the scene's own
// — [Show], [Hide], [Toggle] an element by name — or any other through [Scene.Issue], the way a key
// gives it (players.Plugin.IssueAs). A picture with an [Input] ([Element.Input]: players.Plugin.Through
// for a player) lets the click through to [Scene.Input], the players' bindings, and is told, before
// each pass of input of the active scene, where it lies and what it shows, so the mouse over it is
// that player's and its commands go through the picture's camera; any other element keeps the
// click. A
// shown [Element.Modal] element — a window, or an anchor round one — holds every click and the
// wheel outside it. [Scene.Keys] are the scene's own keys, giving commands the same way.
//
// # Pinned to entities
//
// An element [Element.Under] an effect is shown once for every entity the effect is on, pinned to
// it; [Element.On] pins it to the entities a name or a group calls (entity.Named, entity.Group). The
// scene finds them itself every frame — no rule or renderer is told of the element. It stands
// [Element.Above] the entity (the default), [Element.Below] or [Element.Beside] it, moved by
// [Element.Offset], in every picture of the world on the screen that has it in sight and shows it
// — an [Owner] (players.Plugin.Through) shows a player's own entities and nobody's, so a unit's
// element is in its owner's half of a split screen — kept on the screen: it goes with the camera.
// While the entity is out of sight in every picture it does what [Element.OffScreen] says:
// [PointAtIt] stands at the edge on its side with an arrow, [ShowIt] in the middle with a button
// moving the camera onto it, [GoToIt] moves the camera onto it as it appears —
// through the picture's [Input], a [Looker] (players.Plugin.Through: cameras.LookAt). An entity with
// no place in the world — the world's own, a plugin's — has its element where its parent lays it: an
// anchor's point. A modal pinned element is shown for one entity at a time, the rest waiting. A
// command for [It] — rule.Lift(greeting).On(ui.It), defined in the register as any — is given by a
// pinned element's button for the entity it is shown for (entity.ID): one window serves them all;
// so is a plugin's own command that is an [About] (dialog.Choose). [Element.Where] pins an element
// to the entities a [Pin] holds: [Tagged], those carrying a tag of any family — the one under the
// cursor (selection's Hovered).
//
// # Shapes
//
// Layout gives boxes; an element's shape is how it is drawn and hit inside its box.
// [Element.Masked] cuts it to a [Mask] — [Circle], or a convex [Polygon] — its background, its
// picture and what hits it ([Element.Hits]) alike: a round clock, a hexagonal minimap. A click in
// the corner of a round element's box goes to what lies under it.
package ui
