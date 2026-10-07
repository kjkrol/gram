// Package ui composes a scene's screen out of elements: panels, labels, windows, buttons and
// pictures of the world, laid out in a tree. It knows no cameras: the world seen through a camera
// is a render.Feed, which ui shows as any other picture.
//
// # Scene
//
// A [Scene] is a game.Scene whose screen is a tree of elements: [NewScene] takes its name, its
// pictures of the world — render.WorldRenderers, a Composer of the plugins' renderers as a rule,
// each initialised once however many feeds show it — and its screen, both asked for once as the
// Stage is entered. Every frame the tree is laid over the screen and drawn. [Scene.Input] hands the
// scene's input to the players' bindings; [Scene.Show], [Scene.Hide] and [Scene.Toggle] show and
// hide elements by name ([Element.Named]).
//
// # Layout
//
// An [Element] is given a box by its parent. [Layers] lays its elements over the whole box from the
// bottom up, each covering those before it; [Columns] and [Rows] split it, none covering another,
// each element taking its [Part]: a [Share] of what is left, [Fixed] pixels, or what it needs
// ([Fit]). An anchor wraps an element and places it at its point of the box — [TopLeft],
// [TopMiddle], [TopRight], [MiddleLeft], [Center], [MiddleRight], [BottomLeft], [BottomMiddle],
// [BottomRight] — at the [Element.Size] asked for, kept [Element.Margin] pixels off the edges.
//
// # Elements
//
// [Panel] is a background and a border round an element, padded; [Label] a line of text; [Image] a
// render.Surface — a feed of the world, a picture — filling its box, the box's size given to it;
// [Window] a panel with a title over its elements, one under another, which [Element.Modal] makes
// hold the input while it is shown. Any element takes a background ([Element.Fill]), a border
// ([Element.Border]) and [Element.Padding].
//
// # Shapes
//
// Layout gives boxes; an element's shape is how it is drawn and hit inside its box.
// [Element.Masked] cuts it to a [Mask] — [Circle], or a convex [Polygon] — its background, its
// picture and what hits it ([Element.Hits]) alike: a round clock, a hexagonal minimap. A click in
// the corner of a round element's box goes to what lies under it.
package ui
