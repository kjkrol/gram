# Rendering: one composer per view

[← Back to README](../README.md)

> How the world is drawn: one `render.Composer` per scene, drawing every plugin's pieces as one
> picture per viewport. It started as a proposal; the answers to its questions are in §5.

## 1. Screen layers and the world

- A **Scene** lists its layers bottom to top (`game.Scene.Layers`). A `render.Renderer` draws once
  on the screen, in pixels: a background, a menu, telemetry. A `render.WorldRenderer` shows the
  world and is drawn once per `render.Viewport` — a camera and a rectangle of the screen — of a
  Scene that is a `game.Viewer`; the engine hands it the viewport's camera and an image the size of
  the viewport. No renderer keeps a camera.
- **The world is one layer**: a `render.Composer` over the plugins' renderers, which are
  `render.Source`s. The board, the entities, sight, the routes and the selection do not draw; each
  hands its pieces to the composer's `render.Frame`, and the composer orders them and draws them.
  A split screen and a minimap run the same composer through another camera.

```go
render.NewComposer(board.Renderer(), world.Renderer(), vision.Renderer(), selection.Renderer(), nav.Renderer())
```

The order of the sources does not matter; tiers and depths do.

## 2. Pieces, tiers and depth

A source hands the frame pieces in screen pixels, each with a **tier** and a **depth**:

| Piece | What for |
|:--|:--|
| `Sprite(tier, depth, atlas, id, corners, shade)` | a sprite over four projected corners: a tile, a face, a billboard; `shade` is the light it is drawn in per corner (`render.Shade` of `render.Light`s — red, green, blue — `render.Even`, `render.Lit`) |
| `SpriteRect(tier, depth, atlas, id, x0, y0, x1, y1, shade)` | a sprite over a world box from above, split where it crosses a wrap seam, the shade carried onto the pieces |
| `Line(tier, depth, x0, y0, x1, y1, width, colour)` | a line whose sides fade over a pixel instead of stepping |
| `Fan(tier, depth, points, colour)` | a filled polygon every point of which sees the first one whole |
| `Soft(tier, depth, corners, colour, fade)` | a quad fading towards the sides `fade` names, over so many pixels each |
| `Tile(…)`, `TileRect(…)` | `Sprite` and `SpriteRect` outlined along their own edges: the board's grid at no cost of its own |

Tiers are numbers with room between them, drawn in order:

| Tier | Value | What is on it |
|:--|--:|:--|
| `Backdrop` | 0 | the sky behind the world (`sky.Plugin.Renderer`), at depth −∞ |
| `Ground` | 100 | tiles and the faces of raised ground; a hex grid's lines at 110 |
| `Objects` | 200 | entities |
| `Overlays` | 300 | what lies on the world: routes, cones of sight and their shadows |
| `Air` | 350 | what falls before the eye: rain, snow (`climate.Plugin.Renderer`), at depth +∞ |
| `Marks` | 400 | what must always show: the selection's outline, the box being dragged |

A game may put its own pieces between (250, say) without touching the engine.

- **From above** depth does not matter: tiers in order, and within a tier the order pieces came in.
- **When the camera's projection sorts** (`camera.Projection.Sorts`; the isometric view of
  `plugins/isometry` does) everything below `Marks` is drawn back to front by depth, ties by tier,
  then arrival; `Marks` and above come last, by tier. The depth is the camera's
  (`camera.Camera.Depth`); the isometric one is how far down the screen the middle of the cell
  under a point lies — its diagonal row, unturned — so
  everything in one cell ties with its tile, and what stands on it (a higher tier) is drawn over it
  and under the row in front. A mountain in front hides the route and the cone behind it; the
  selection is never hidden.
- **A piece lying across cells takes the depth of its nearest end.** It is drawn after every tile
  it lies on; with the depth of its middle the nearer tile would cover half of it. This is why the
  cone's edges and its shadows are cut into pieces a ground step long, each draped over the ground
  with a depth of its own, and why a hex grid's edges take the depth of the neighbours they border.
  A square grid needs none of it: each tile outlines itself.

## 3. One shader, few calls

Every piece is drawn with one Kage shader (`render/compose.kage`): the sheet's texel times the
vertex colour, times a fade towards up to four edges. A plain colour — a line, a fan, a shadow —
samples its sheet's white texel (`AtlasSource.White`; `Atlas.Close` bakes a white patch in), so
the pieces of one sheet go in one `DrawTrianglesShader` call whatever mix of sprites and colours
they are. A colour that comes before any sheet uses the composer's own white image. Consecutive
quads with the same tier, depth and sheet are kept as one item, so a frame of a thousand sprites
from above is sorted as one.

The fade is in the vertices: `Custom0..3` hold, per edge, 1 plus the distance to it in units of its
fade, minus 1 minus the distance in pixels to an edge outlined, or 0 for an edge left alone — so a
plain sprite sets nothing. A `Line` fades its two sides over half a pixel each, which stands in for
anti-aliasing; a `Soft` quad fades the sides it is asked to, by as many pixels as it is asked; a
`Tile` darkens the pixel along each of its edges, so two tiles side by side share a line a pixel
wide. The grid of a square board is that: the tiles it is drawn on anyway, a few numbers more in
their vertices (CPU drawing of island-isometric-demo with the grid on: 2.5 ms with lines, 1.9 ms
outlined; navigation-demo from above: 0.67 ms, 0.13 ms).

What glints — a sea, ice — is worked out per pixel. `Frame.Glint(x0, y0, x1, y1, shine, shore)`
after a sprite lays a quad over it on the same sheet, sampling its white texel: red is the shine,
green and blue where each corner lies in the world, alpha 2 plus how much of the sun reaches the
corner (a colour's alpha is never over 1, so the shader tells them apart), `Custom0..3` the shore —
the way to it, how far, how near. The shader tilts the surface there by seven small waves running in
different directions, moved on by the composer's clock; near a shore the waves give way to a swell
whose crests follow the distance to it, so they face the shore and roll in, their phase drifting
slowly along the coast so they do not reach it everywhere at once. What the surface throws back of
the frame's sun (`Frame.Daylight`, which the board sets: the sun's way, strength and colour, the
sky's colour and the light from it) towards the eye (`camera.Projection.Toward`),
gathered tightly round the perfect reflection, is added to the tile. A glint alone would show the
swell only where its slopes face halfway between the sun and the eye — never on a coast it runs
across — so each crest also breaks into foam the last cell before the shore, as bright as the tile
in the sky's light and the sun's and laid over it with its own alpha: the surf shows on every
shore, whatever the sun. Water also reflects the sky, the more the flatter the eye looks at it
(Fresnel, over a normal tilted by only a third of the waves, so fine waves far off do not stripe
it): from above it keeps its own colour, looked along it takes the sky's. Sparse points twinkle under
a high sun and all but vanish under a low one — and none where the sun stands behind the eye, as
it does all day over the south in the isometric view (hence `sky.Config.NoonWay`, north-west by
default). The tile keeps
its outline; the uniforms are written over in place, so a frame allocates nothing for them.

The weather is drawn the same way (`Frame.Weather`, which the board and the world set from
`world.Weather`). `Frame.Overcast(box)` after a tile lays a quad over it — only under clouds —
whose vertices carry where they lie in green and blue and 4 in alpha. The shader works out the
clouds over each pixel: noise in a few sizes, spread out so there are clouds and clear sky between
them, carried by the wind's drift, taking up to half the sun's light, none at night. Snow and ice
are not drawn apart: they are kinds of the ground an effect puts on a cell, drawn as any other.
The shadow falls straight under its cloud: the clouds themselves are not drawn, and a shadow cast
off them towards the sun would jump with every step the sun takes. The same clouds put out the sun's glint on water, the wind turns the waves its way and makes
them as steep as it blows, and the sky water reflects (and the sky's backdrop) greys the more the
clouds cover it (`render.Overcast`). Kage shades pixels and cannot move vertices, so what sways in
the wind — a `CellKind.Sway`, a `world.Appearance.Sway`, which an effect sets — is leant by its look on the CPU from the frame's wind
and clock (`Frame.Wind`, `Frame.Time`, `render.Sway`): its top moves with the wind, rocking as gusts
roll through downwind.

## 4. Sight on the ground

The vision renderer hands its style a ring of `vision.ConePoint`s: the observer at its altitude,
out along one edge of the cone in steps of the ground, round the boundary and back down the other
edge, every point on the ground under it with its screen position and depth. `DefaultConeStyle`
strokes it with lines, each at the depth of its nearer end.

The ground out of sight (the shadows of aabbworld's `View.Shadows`) is veiled in `Soft` pieces:
per angle, per band, cut every ground step. A piece fades in at the near end of its band and at the
far end, and sideways only where the next angle has no shadow at all — so neighbouring angles join
without seams and the silhouette of a shadow is soft. `vision.Shadow{Color, Fade}` sets the veil
(`Plugin.WithShadow`); the fade is in world units, scaled by the zoom.

## 5. The questions it answered

1. **Layers.** Named tiers on a scale with gaps (100, 200, …), not a fixed set.
2. **Depth of overlays.** Routes and cones lie on the ground and sort with it, so what stands in
   front hides them; the selection and the dragged box are always on top (`Marks`).
3. **Migration.** At once: `render.Sorted`, `Submitter`, `Sink`, `Overlayer`, `QuadBatch` and
   `LineBatch` are gone, and every plugin renderer is a `Source`.
4. **Shaders.** One shader for everything, with soft edges for the shadows and the lines.

## 6. What it cost and bought

Measured on 2026-09-26, the old and the new tree run alternately, 300 frames each, in a virtual
session with software GL:

| Demo, view | Frame before | Frame after | CPU drawing before | after |
|:--|--:|--:|--:|--:|
| island-demo, whole island | 20.2 ms | 12.1 ms | 2.0 ms | 1.4 ms |
| island-demo, close | 22.5 ms | 9.1 ms | 1.3 ms | 0.5 ms |
| island-isometric-demo, whole island | 18.0 ms | 13.7 ms | 4.0 ms | 3.6 ms |
| island-isometric-demo, close | 27.1 ms | 16.4 ms | 3.3 ms | 3.1 ms |

The frames are shorter because far fewer calls reach the GPU: the vector paths of the cones, the
shadows, the routes and the outlines each used to be draws of their own. Gathering alone costs a
little more than it did — `Benchmark_World_Draw`, 5,000 sprites composed without a screen, is about
18% slower for the ordering the composer keeps (see BENCHMARKS.md).
