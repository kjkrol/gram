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
`board.Plugin.NewAtlas(size)` is written the same way) is declared in one place, a scene's Layers,
and `WithRenderer` applies its rules itself: `Add(kind, size, look)` — a `render.Look` is a
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
composer's uniforms: the terrain, the sky, the rain, the world's sprites (`render.Sprites`), the
views of sight, the routes, the clouds' shadows. A `render.Still` is a frame composed once in
world units and kept on the GPU (`gpu.Kept`), drawn every frame scaled, moved and lit
(`gpu.Draw.Place`/`Tint`): the board's flat map. `render.Frame.Time` is the clock's `Shown`
(`render.Clocked`), so animations move every frame. A plugin adds telemetry lines through a
`render.Reporter` (`telemetry.NewPlugin(w).With(p.Reporter())`, the telemetry panel).

How things lie on the screen is a plugin's `Look`, swappable: `world.Look` (an entity's sprite,
where it is picked, its footprint) and `look.Look` (a cell as a `look.Tile`), both flat from above
by default; the topography sets both for relief. The renderers keep their data and ask the Look
for geometry only. Pieces carry a `render.Shade` (a `render.Light` per corner; `render.Even`,
`render.Lit`).
