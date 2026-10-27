// Package ui composes a scene's screen out of elements: panels, labels, windows, buttons and
// pictures of the world, laid out in a tree. It knows no cameras: the world seen through a camera
// is a render.Feed, which ui shows as any other picture.
//
// # Scene
//
// A [Scene] is a game.Scene whose screen is a tree of elements, made by [NewScene] once the world
// is there. [Scene.Input] hands its input to the players' bindings; [Scene.Show], [Scene.Hide] and
// [Scene.Toggle] show and hide elements by name ([Element.Named]). Its drawer is a
// plugin.Publisher: once a frame, after the ticks, it reads off the game where its pinned
// elements' entities stand, every Text's words, every control's value and what is hidden, and
// the frame is laid, drawn and clicked as that says; an element put in since reads them as it
// is laid. It is laid on the frame's barrier, where nothing ticks or draws (game.Laying), and
// drawing only paints it.
//
// # Layout
//
// An [Element] is given a box by its parent. [Layers] lays its elements over the whole box, each
// covering those before it; [Columns] and [Rows] split it, each element taking its [Part]: a
// [Share] of what is left, [Fixed] pixels, or what it needs ([Fit]). An anchor ([TopLeft],
// [Center], [BottomRight] and the rest) places its element at its point of the box, at the
// [Element.Size] asked for or the [Element.Fraction] of the box, [Element.Margin] pixels off the
// edges.
//
// # Elements
//
// [Panel] is a background and a border round an element; [Label] a line of text ([Heading],
// [Subheading] and [Title] its weights); [Image] a
// render.Surface filling its box, whose source may draw on the glass over it (a render.Feed
// with a render.Glass source), last of all, over whatever lies over the picture; [Window] a panel with a title over its elements, which
// [Element.Modal] makes hold the input while shown; [Element.Passing] lets the pointer through an
// element and all in it to what lies under, a button or a control inside aside; [Blank] a gap;
// [Drawer] a screen renderer;
// [Scroll] an element taller than its box, scrolled by its bar or the wheel. Any
// element takes [Element.Fill], [Element.Border] and [Element.Padding]. [LabelOf], [ButtonOf] and
// [WindowOf] ([HeadingOf] too) read their words off a [Text] every frame, for the entity a pinned
// element is shown for; [Of] is a Text read off a component of that entity, [While] one saying
// its words while a function says so. A Text saying nothing leaves its element out.
//
// # Controls
//
// A control shows a value read through a function every frame and gives, as it is used, the
// command a second function builds for the new value, the way a Button gives its commands: a
// [Checkbox] on or off, a [Slider] dragged along its track or stepped by the wheel over it, a
// [Dropdown] opening its options over everything, the one clicked chosen. A control reads no
// entity: pinned, every instance shows the same. The settings screen (players) is made of them.
// An element given [Element.Live] is live while its function says so, else drawn dimmed and
// passed by the pointer, with everything in it: a slider beside a checkbox that is off.
//
// # Theme
//
// A [Theme] is how a scene's elements look where they say nothing of their own, its Accent the
// controls' marks. [Scene.Theme] sets it; an element's own Fill or Border wins.
//
// # Input
//
// A click goes to the topmost element it hits. A [Button] gives its commands: the scene's own
// ([Show], [Hide], [Toggle]) or any other through [Scene.Issue], else as nobody through the
// Runtime. A picture with an [Input] ([Element.Input]) lets the click through to [Scene.Input],
// unless its Input is [Clicked] and takes the press itself (a minimap); any other element keeps
// it; a shown modal element holds every click and the wheel outside it. A slider pressed holds
// the cursor till the button is let go, wherever that is; a dropdown's open options take every
// click, one outside them closing them, as Esc does ([Scene.Unfolded]). A [Hovered] Input hears
// whether the cursor lies on its picture uncovered. [Scene.Keys] are the scene's own keys;
// [Element.Show] and [Element.Hide] show and hide an element from code.
//
// # Pinned to entities
//
// An element [Element.Under] an effect is shown once for every entity the effect is on, pinned to
// it; [Element.On] pins it to the entities a name or a group calls, [Element.Where] to those a
// [Pin] holds ([Tagged]). It stands [Element.Above], [Element.Below] or [Element.Beside] its
// entity, moved by [Element.Offset], in every picture that has it in sight and shows it ([Owner]).
// Out of sight everywhere it does what [Element.OffScreen] says ([PointAtIt], [ShowIt], [GoToIt]),
// moving the camera through the picture's [Looker]. A pinned element's button gives a command for
// [It], or an [About], for the entity it is shown for.
//
// # Shapes
//
// [Element.Masked] cuts an element to a [Mask], [Circle] or a convex [Polygon]: its background,
// its picture and what hits it alike.
package ui
