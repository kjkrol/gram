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

Every piece is drawn with one Kage shader. Kage is Ebitengine's shading language, Go in its
syntax, compiled when the program starts for whatever the graphics card takes (GLSL, HLSL, Metal):
a fragment shader, run for every pixel of every triangle drawn, from what the triangle's vertices
carry — where they lie, where they sample the sheet, a colour and four custom numbers, each blended
across the triangle — and uniforms, values for the whole draw (the sun, the time, the wind). One
shader for all keeps a sheet's pieces one draw, and the depth order of the isometric view whole.

The shader is put together from parts (`render.RegisterMaterials`, `render.ShaderSource`): the
composer's own, `render/compose.kage` — the frame's uniforms, the sprite with its fades, outlines
and blends, and a few functions any part may use (`noise`, `hash`, `sunWay`, `seen`, `faded`,
`blended`) — and every material a plugin brings in its own Kage: the landscape's clouds' shadows
(`plugins/landscape/overcast.kage`), the landscape's water (`plugins/landscape/water.kage`: the sea's glint and
running water). A plugin registers its materials as its package is set up; the composer compiles
them all once, before it first draws (`render.Compile`), a function handing each overlay to its
material written in at the end. An overlay (`Frame.Overlay`) carries 2 plus twice its material's
number plus a fraction 0 to 1 in alpha — a colour's is never over 1 — where it lies in the world in
green and blue, and what its material reads in red, the fraction and the customs; a material's
entry is `func Entry(p vec2, red, fraction float, custom vec4) vec4`.

The composer's own part draws the sheet's texel times the
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

What glints — a sea, ice — is worked out per pixel, by the landscape's `SeaGlint` material.
`landscape.Glint(f, world, shine, lit, shore)` after a sprite lays its overlay over it on the same
sheet, sampling its white texel: red is the shine, the fraction how much of the sun reaches the
corner, `Custom0..3` the shore — the way to it, how far, how near (`landscape.Shore`). The shader tilts the surface there by seven small waves running in
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

Running water is the landscape's `RunningWater` material. `landscape.Stream(f, world, shine, lit, flow)`
lays its overlay over a piece of any shape, with how fast the water runs at each corner in
`Custom0..1` (`landscape.Flow`, world units a second): the board hands it for a kind with a `Flow`
(the landscape's tile flow), down the slope of the cell as fast as the Flow by the square root of the
slope, each corner the mean of the running cells meeting there. Its ripples and flecks of foam are
a flow map: noise in the world carried down the current, sampled twice, each time the current has
carried it a fraction of a period on, half a period apart and crossfaded, so the pattern flows on
for ever without stretching and runs on without a seam from one piece of a river to the next; the
faster it runs the rougher, the more flecked and at last white (`whiteFrom` to `whiteFull`): a
rapid, a waterfall off a cliff.
A `board.Way` across a cell — a brook, a river — is drawn over its tile the same way
(`landscape`): out to halfway to each neighbour it runs on to, the two ways out to the
widest neighbours as one band curving round the middle of the cell (a quadratic curve through the
middles of the sides, so it runs on smoothly into the next cell) and any other curving in to join
it, each piece projected corner by corner at the height of the ground under it — a way with a
`Fade` drawn with `SpriteBlend` down to nothing, its water with it (`landscape.Stream` over a blended
sprite takes its weight in `Custom2` and its mark in `Custom3`), running on level ground the way it
fades —
on a tier just over the tiles (`Ground+5`) — and the last stretch of a band running slantwise,
which reaches into the cells either side of the corner, at the depth of the nearest of the four
cells meeting there, as a piece lying across cells takes the depth of its nearest end —
its clouds' shadows laid with `landscape.Overcast` and its water with `landscape.Stream`, running down the
band.

Two kinds of ground meet along a line, not along the edges of their cells. `Frame.SpriteBlend(…,
weight, soft)` draws a sprite with a weight in each corner's alpha and 10 plus how soft in
`Custom3`; the shader shows it where the weight, blended across the quad, is over a half, fading in
over `soft` either side. The board (`landscape`) lays, over each quarter of a tile, every
neighbouring kind that has a `Spread`, weighed at the tile's corners (the share of the four cells
meeting there that are of it), the middles of its sides (of the two) and its middle (none): the
weights at a point are the same from every tile, so the line runs on from tile to tile — a
staircase of cells becomes a slant, a cell alone a rounded diamond. A cloud's shadow is laid once
over the tile's top, after the pieces on it (`Frame.Last`, `Frame.OverlayOn`,
`landscape.OvercastOn`), so it shades them too at no piece each. Water keeps its glint by lying under: a tile of a kind that spreads
next to a kind whose `landscape.Style` lies `Under` is drawn as that kind (`Tile.Base`), glint and all, and its own
kind laid over it weighed by the share of cells not under; the water's tile has the land round it
laid over it the same way, in the sprite most of it is.

Nothing of this is worked out per frame. The board counts the changes to each cell
(`Board.CellVersion`); the renderer keeps what it read of a cell until the cell changes, a tile's
blends and way — placed as if the tile stood at 0, 0 — until a cell round it does, and a tile's
light until the terrain or the sun does; a frame lights the kept pieces and hands them on. Composing
the whole island takes 3.2 ms from above and 4.6 ms isometric, half what it did
(`Benchmark_Board_Island`, BENCHMARKS.md).

Far off, less is drawn. A tile's `Detail` (the landscape's) is how many pixels a cell spans
on screen, eased from none at 6 to all at 12: its water glints and runs only as far as it, so a far
view of the whole world lays no water overlays at all, and its shore is worked out only from 16 up
— far off the sea is open water. The shader knows how many world units a pixel spans (`Glint.z`)
and leaves out every wave, swell and ripple finer than a pixel or two (`seen`), which would only
flicker. The whole island far off composes in about 2 ms, both views.

Far off, too, a tile's blends and ways are not handed on piece by piece. Under 16 pixels a cell on
a square grid the landscape paints them once on a ground sheet: the board's atlas copied to its
top-left and the board's cells below it, 16 pixels each, painted with the composer's own shader
(`render.Paint`) from the kept pieces, each cell anew only when it or a cell round it changes. The
dressing hands the renderer that sheet as the tiles' (`board.Dressing.Sheet`), so a top, its faces
and what lies on it share one sheet and one call; a tile anything lies over draws it as one piece
of the sheet (`Frame.SpritePart`) in its light. The pieces a way's water shines in ease out between
24 and 16 pixels a cell, so nothing shows the change. In the island demos, the whole island on a
screen of 1024 by 768 took 10.5 ms of the CPU a frame isometric and 5.0 ms from above, against 8.5
and 2.95 ms before it had blends and ways; drawn from the sheet it takes 8.9 and 3.1 ms.

The weather is drawn the same way (`Frame.Weather`, which the board and the world set from
`world.Weather`). `landscape.Overcast(f, world)` after a piece — `landscape.OvercastOn` over a
tile's top, after all that lies on it — lays the landscape's `CloudShadow` overlay over it — only
under clouds — taking how faint the sprite under it is and how it fades or blends.
The material works out the
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
