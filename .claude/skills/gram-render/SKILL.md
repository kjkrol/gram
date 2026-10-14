---
name: gram-render
description: How gram draws — the Composer, tiers and depth, Direct sources with their own WGSL, Still, Feed, the world's and the board's atlases and looks (Add, Under, Turning, Facing, Animated, WhileMoving, Phased, Looped, Lying, Upright, Edges, Sheet, materials), the plugins' Look seams, telemetry. Use when touching render/, an atlas or a look in a plugin or a demo's scene.go, a shader, a renderer, or when something is drawn wrong.
---

# gram-render

## Atlases and looks

A `render.Atlas` sizes nothing up front: `Add(of, size, draw)` records a sprite in of's
slot (`render.Sprited`: `kind.Of`, `cell.Of`, a bullet `Ammo`, a bare `SpriteID`) at a texture size
of its own (the drawn size is the entity's box), `Close()` lays the sheet out and bakes it. The
world's atlas (`world.Plugin.NewAtlas`, `plugins/world/atlas.go`, a facade; the board's
`board.Plugin.NewAtlas(size)` is written the same way) is filled by the figures' `Look`s when
`Renderer()` is first asked (`NewAtlas().Looks().Close()` inside; `Close()` hands it to the
plugin, `SetAtlas`, which applies its rules itself): `Add(kind, size, look)` — a `render.Look`
is a
SpriteDrawer drawn once, or a `MaterialID` worked out per pixel every frame (one `Frame.Material`
quad in the box; `Custom` = {middle, half a side, heading in radians}, `Red` = speed in own lengths
a second up to 1, `World` the box), or `render.Phases`/`Loops` — `.Under(effect, look)` the look
while the effect's marker is on (a twin slot swapped in by `render.Swap(twins, mark.In)`;
`render.Phased(n, draw)`: n frames along the effect's course, never for an effect without `Lasts`;
`render.Looped(n, period, draw)`: frames by the clock; a material under an effect reads how far it
has run as its `fraction`; on one entity under two effects the look declared later wins),
`.Turning()` the sprite turned the way its entity is headed (`Appearance.Angle` from `Vel.Dir`;
degrees, 0 east, against the clock with y down, `render.Arrow`'s convention; a square box whose
content fits the inscribed circle), `.Facing(n, draw)` its directional twins — declared first, it
gives every later look a way: `.Animated(n, period, draw)` (a drawer of the frame, or of the angle
and the frame), `.WhileMoving()` (standing it is drawn as it faces),
`render.PhasedFacing`/`LoopedFacing` under an effect — `.Lying()` a sprite laid flat at the entity's
`Altitude` (a deck, a ford; `world.LyingLook`; an entity carrying a `world.Lie{Length, Width, Edged,
Edge}` is laid over that rectangle headed its `Vel.Dir`, `.Edges(draw)` the sprite standing along
its long sides while `Edged`), `.Upright()` a sprite standing true in relief (`world.UprightLook`: a
pier). Frames come off a `render.Sheet` (`NewSheet(img, w, h)`, `DecodeSheet(name, png, w, h)`,
`At(col, row)`, through `render.FromImage`; a frame no square is drawn true in a box of its shape,
`board.Shape{Size, Width}`). The board's `Covering(e)` is the cover laid over cells under an effect,
`Slot.Under(effect, draw)` on the board's atlas.

A periodic cosmetic — a blink every 3 s, a foot stamped every 4 s — is a timed look, not an
effect: `render.Timed(beat, draw)`/`TimedFacing` go round a `Beat` of named frames each for its
span (a blink: two beats of two frames; two periods at once: one round of their common
multiple), so few sprites are drawn; `Loops.Frame(now)` picks the frame. `render.SpriteID` is
two bytes: `render.MaxSprites` (65 536) sprites an atlas, every frame of every way under every
effect among them — `Kinds().NewSprite()` and the board's panic past that (a byte until
2026-10-14: the wire demo's figure alone took nearly 256).

## Rendering

The world of a scene is one `render.Composer` over the plugins' renderers, `render.Source`s:
each hands its pieces to a `render.Frame` in screen pixels with a `Tier` (`Ground` 100,
`Objects` 200, `Overlays` 300, `Air` 350, `Marks` 400; `Backdrop` 0; a game may use the gaps)
and a depth; the composer draws tiers in order and, when the camera's projection `Sorts()`,
everything below `Marks` back to front by depth (ties by tier, then arrival). One WGSL shader
(`render/shaders/compose.wgsl` over the library and every material registered with
`render.RegisterMaterials`) draws every piece; `Frame.Line`/`Soft` fade their edges;
`Frame.Tile`/`TileRect` outline a tile; a piece across cells takes the depth of its nearest end.
A `render.Direct` source draws with a WGSL shader of its own (`render.NewMeshShaderWith`,
`Instanced(n)`, depth test and write, `ReadDepth`) at its tier, handed a `render.Target{Screen,
Depth}` (the frame's shared depth buffer, reversed, cleared as the frame begins) and the
composer's uniforms: the terrain, the sky, the rain, the world's sprites (`render.Sprites`: by
`Layer(depth, under)`, the lower first and at one depth the lying, sorted on buffers kept from
frame to frame; the world's flat look layers a standing sprite at `Z.Top()` and a lying one at
its `Altitude`, so seen from above a world with heights draws what is higher over what is lower
and a flat one, with no `Z`, as it came), the
views of sight, the routes, the clouds' shadows. A `render.Still` is a frame composed once in
world units and kept on the GPU (`gpu.Kept`), drawn every frame scaled, moved and lit
(`gpu.Draw.Place`/`Tint`): the board's flat map. `render.Frame.Time` is the clock's `Shown`
(`render.Clocked`), so animations move every frame. A plugin adds telemetry lines through a
`render.Reporter` (`telemetry.NewPlugin(w).With(p.Reporter())`, the telemetry panel).

## The plugins' renderers and styles per kind (done 2026-10-14; the findings below led to it)

`WithRenderer` is gone and a scene names no atlas: `world.Renderer()` and `board.Renderer()`
dress themselves from the figures and tiles the first time they are asked (`NewAtlas().Looks().
Close()` inside, the board's sprites the cell's bounds across; `Close` hands the atlas to
`SetAtlas`, which a test may call with a foreign `render.AtlasSource`); every other plugin
makes its renderer in `NewPlugin`. A game declaring looks by hand closes its own `NewAtlas`
before asking. The user's word (2026-10-14): an atlas made and closed at once in a scene is
ceremony; a size given by another channel than the constructor is noise.
A figure's `Look` styles what the plugins draw about its kind with `Slot.Styled(s)`
— `selection.Highlight{Line, Width}`, `vision.Cone{Color}`, `navigation.RouteStyle{Line, Goal,
Width}` — kept in `render.Styles` on the world's atlas (`world.Plugin.Styles()` once it is
closed), read with `render.StyleOf[S](styles, sprite)`; the renderers carry the colour
per instance on the GPU. `board.ShowGrid(bool)` is the grid option.

- Only scenes call `plugin.Plugin.WithRenderer(atlas)`; the engine never does. Three kinds of
  implementor: **needs the atlas** — `world` (applies the atlas's swaps and rules, panics on a
  bare `render.NewAtlas` for `Looks`) and `board` (panics on nil; sets
  `Res.Render.ShowGridLines = true` itself, so a scene's `= true` is dead and `= false` is the
  real option); **ignores it** — `collision`, `atmosphere`, `topography` (`WithRenderer` a no-op,
  `Renderer()` made by the plugin); **uses it as a construction step only** — `navigation`,
  `vision`, `selection`, `bridges` (`Renderer()` is nil before `WithRenderer(nil)`): 40
  `WithRenderer(nil)` calls in the demos are that boilerplate.
- What each renderer draws is a look of an entity under a state, not of the plugin: navigation
  the routes and goals of an entity's `MoveOrder` (`WithRouteStyle`, one style for all); vision
  the cone of an entity's `Sight` (`WithStyle`, `WithShadow`; `WithViews(render.Show(…))` picks
  whose); selection the outline of the `Selected` tag through `world.Look.Footprint`
  (`HighlightStyle`, no way for a game to set it today) plus the marquee, a player's gesture;
  bridges the bridge in hand, a future entity drawn from its deck's slot; collision the boxes.
- The composer's order matters only on ties within a tier (tier, then depth, then arrival).
- Decided direction (done, as above): these customisations become figure-style hooks declared in a
  figure's `Look` on the world's atlas — looks `Under` a tag as under an effect, per-kind
  attributes other plugins read (`Sight(style)`, `Route(style)`) — the plugin keeping the
  algorithm, the figure the style, defaults where a figure says nothing; `WithRenderer` leaves
  the `plugin.Plugin` contract, renderers without an atlas are made by their plugin. First step:
  `Under` a tag and the selection's outline; then vision and navigation; the composer gathered
  from the plugins made (`world.Made()`) is a separate talk (a minimap omits layers).

**A `color.RGBA` given to gram is straight alpha** (2026-10-14, after a zone at `A: 10` came out
as bright as an opaque one): `render.Frame`'s pieces, `Image.Fill`, the shapes, text and the
sprite `Canvas` premultiply it for the GPU (`premultiplied`, `rgbaOf`, `straight` →
`color.NRGBA` for `image/draw`), as do the routes', views' and grid's own shaders. Go defines
`color.RGBA` as premultiplied — a value above its alpha then adds light — so a colour handed
to the GPU without the conversion glows; any new path from a colour to floats must multiply
R, G, B by A.

How things lie on the screen is a plugin's `Look`, swappable: `world.Look` (an entity's sprite,
where it is picked, its footprint) and `look.Look` (a cell as a `look.Tile`), both flat from above
by default; the topography sets both for relief. The renderers keep their data and ask the Look
for geometry only. Pieces carry a `render.Shade` (a `render.Light` per corner; `render.Even`,
`render.Lit`).
