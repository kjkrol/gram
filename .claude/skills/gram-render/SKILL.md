---
name: gram-render
description: How gram draws — the Stage's picture (world.Picture, ctx.Draws, ctx.Looks), the Composer, order by Z and the depth buffer (At, passes, places, mark levels), Direct sources with their own WGSL, Still, Feed, the world's and the board's atlases and looks (Add, Under, Turning, Facing, Animated, WhileMoving, Phased, Looped, Lying, Upright, Edges, Sheet, materials), the plugins' Look seams, telemetry. Use when touching render/, an atlas or a look in a plugin or a demo's scene.go, a shader, a renderer, or when something is drawn wrong.
---

# gram-render

## Atlases and looks

A `render.Atlas` sizes nothing up front: `Add(of, size, draw)` records a sprite in of's
slot (`render.Sprited`: `kind.Of`, `cell.Of`, a bullet `Ammo`, a bare `SpriteID`) at a texture size
of its own (the drawn size is the entity's box), `Close()` lays the sheet out and bakes it. The
world's atlas (`world.Plugin.NewAtlas`, `plugins/world/atlas.go`, a facade; the board's
`board.Plugin.NewAtlas(size)` is written the same way) is filled by the figures' `Look`s once
the Stage is set up (`ctx.Looks`: `NewAtlas().Looks().Close()`; `Close()` hands it to the
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

The world of a Stage is one `render.Composer`, `world.Plugin.Picture()`, over the
`render.Source`s its plugins gave `ctx.Draws` (2026-10-15, stage 3: no `Renderer()`, no
`picture()` in a demo, `Composer.Init` once however many scenes show it; `render.Layer` is
`render.Drawer`, `Scene.Layers` `Drawers`, `ui.Layer` `ui.Drawer`). The one other composer is a
minimap's (`plugins/minimap`): its own three sources, none in `ctx.Draws`, through an inner
camera of its own, drawn anew at most 30 times a second (a `render.Surface` returning its last
image in between; `ui.Image` lays it out every frame, so a resize alone forces a redraw).
`render.StrokePolygon` is one draw however many sides (a round element's border). A demo that makes a plugin
draws all it draws: selection and routes in bullet, platformer and split-screen, holds (the board's "Cells held" setting)
in every demo with a board. **What is drawn over what is Z's alone (2026-10-15), never a layer
nor the order a plugin registers in**: `render.Tier` is gone. Each
piece a source hands its `render.Frame` lies at a `render.At`: `f.At(x, y, z)` (a height in the
world; the key back to front is z from above, `cam.Depth` in a view that sorts), `f.Lifted(x, y,
z, lift)` (what lies on a surface, ordered among what lies there by its lift), `f.OnGround(lift)`
(the view's middle, for a Direct over the whole ground), `render.OnTop(level)` (a mark at its
level), `Furthest` (the sky), `Nearest` (the rain). The passes: the **opaque** ground first (the
board's `Still`, the terrain, the hex prisms: depth tested and written, any order), then the world
**seen through**, back to front by key, every piece tested against the depth (its depth worked out
on the GPU from its height: `gpu.Vertex.DstZ`, `gpu.Sight` from the camera's rays and
`SceneTransform`), then the **marks** by level, at one level in the order added, untested: the
selection's outline 1, its box being dragged 2, the cameras' scroll zones 3 (each plugin's constant,
`outlineLevel`, `marqueeLevel`, `zonesLevel`). Between the world and the marks lies what is
`At.OverFigures()`: drawn after every figure, tested against the ground's depth alone (the frame
reads the ground for it: a copy of the depth and a switch of buffer, some 0.15 ms a frame while
drawn, nothing while not), so a collision solid shows through its own billboard and a hill before
it hides it. Goals, holds, a bridge in hand and boxes are no
marks: they lie in the world. **A source whose pieces lie over the figures says so**
(2026-10-20, stage T: `render.Overlying{Overlies()}`: the world's figures, aloft's clouds, the
drops, the selection, collision's boxes, the cameras' layers; the cameras' zones are a
`render.Glass` instead: `DrawGlass(dst, cam)` called by `Feed.DrawGlass` from ui after the whole
screen is painted, onto the picture's part of it, so the HUD never covers them, 2026-10-20), and the composer is
`render.Laid` (2026-10-20, stage U, in place of `Grounded`): `Lay(cam, apart)` composes every
source (the ground's alone when apart) once, in cam's pixels, the feed having cropped the camera
to the whole frame at every pixel (`Crop(camera.Window{W, H}, 1)`, exact, `Cropped()` false);
`DrawLayer(layer, cam, at, hole)` draws what was composed into `layer`, the camera cropped to the
frame's window `at`, leaving out the frame's window `hole` where the frame has a ground (R2,
2026-10-20: a layer but the innermost is handed the next one's window inset by `LayerFeather`
plus two of its own pixels on the fading sides, `Feed.holeOf`, and `gpu.FillDepth`,
`depth_fill.wgsl`, writes the nearest depth over it right after the layer's depth is cleared, so
nothing depth-tested is drawn where the layer over it covers it whole: the preset's layers draw
about 0.52 of the frame's pixels, not 0.71; zero, no hole), the pieces placed by the window's
scale and offset (`gpu.Draw.Place`, which
`quad.wgsl` applies before `heightDepth`, so a placed vertex takes its depth from the layer
camera's sight read at render time) and the Directs drawing through the cropped camera (so a
Direct gathering at Compose draws as often as there are layers: `Sprites.Draw` resets no more,
`Begin` does; the routes reset at Compose and place their rectangles into the drawing camera's
frame); its opaque pass goes into a Readable depth kept a layer (`keptDepths` 12, by size and
layer index: two layers of one size keep their grounds apart); `DrawOver(screen, cam)` places
every laid depth into the screen's (`Image.PlaceDepth`, `gpu.PlaceDepth(target, from, to, at)`:
`depth.wgsl` reads `depthAt(p.xy * place.xy + place.zw)` under a scissor of the window, the last
laid over the first) and composes and draws the Overlying ones onto the screen tested against
it, the figures writing into a copy and `OverFigures` bound to the placed one; the over pass
inherits the uniforms and images the ground pass set (the sun, the weather, `Fog`,
`GroundHeights`, `GroundShade`), put back on the frame before the Overlying sources compose (the
over pass hands its Directs `Target.Coarse`: the most of the screen's pixels each way a pixel of
a laid ground spans, `max(at.W/w, at.H/h)` over the layers laid, 0 in every other pass, so a soft
Direct works itself out at the ground's pixels: aloft, 2026-10-20, R1), so
`Uniformed` sees them and every Direct is handed them; one asking `DrawOpaque` there is not
heard. A `Feed` draws a `camera.Figured` camera so whenever its picture is coarse somewhere —
whole at a resolution under 1, or in layers one of which is — and soft layers from one `Lay`
too; a picture that is not Laid is drawn whole a layer. `Frame.Edge` is a line
whose ends carry heights of their own
(`item.own`; the composer zeroes them where the frame has no depth, as a z outside 0..1 would be
clipped). A frame with no opaque ground (a world without heights) has no depth at all, as before,
the order alone deciding. Equal Z: the later wins (GreaterEqual for everyone); among the figures the
lying first (`Sprites.Layer(at, under)`), so the standing covers the lying. A `render.Direct` (no
`Tier()`) asks in Compose where it draws: `f.DrawOpaque()`, `f.DrawAt(at)` (a place among the
pieces), `f.ReadGround()` (it reads the ground's depth: the opaque pass is drawn into a readable
buffer, `Image.CopyDepth`, a draw since a texture copy of a depth is wrong on GLES and Vulkan in
gogpu, gives the frame's buffer; the reader gets `Target.Ground` and writes `frag_depth` = the
ground's own, testing GreaterEqual, so what stands nearer hides it; dear: the opaque pass into a
readable buffer cost the topography ~0.6 ms a frame, 2026-10-16 — what needs only where the
ground stands reads `GroundHeights`, below); `Target.Pass` says which pass
it is drawing; one asking for nothing is not drawn, `Frame.Places` lists the asks for a test. Lifts
(world units over the ground): what a `look.GroundDressing` lays over tiles composed into the frame
0.02, board materials 0.05, grid lines 0.1, holds 0.15, sight 0.25, routes 0.75, route goals
0.8, a bridge in hand 0.9 (over its deck's level), figures `world.FigureLift` 2 (topography's
`spriteLift` is the same 2), an entity's box `FigureLift`+0.5 over its altitude in the order, its
base's vertices 0.05 over it and its solid's at its altitude and top (collision's "Entities' boxes" setting: bases,
solids in a world with heights, none). The figures (`render.Sprites`, the world's flat look) are
drawn once, among what is seen through, after the opaque ground, at the lowest figure's place, back
to front among themselves; where the frame has a depth their visible pixels (alpha over half a step)
write it, so a decal drawn after and behind them hides, and one before shows under their soft edges.
`render.Sprites` (`Fog(amount)` for the pieces next: the instance's `lift.y`, mixed into the
frame's `U.Fog` in `sprites.wgsl`; `Draw(t, cam, u)` takes the frame's uniforms for it) lays world
rectangles on the GPU (the camera's `SceneTransform` matrix, the box
round the projected corners like `ToScreenQuads`, over a wrapping world the copy `windowed` nearest
the window and one past each seam, six vertices a copy in one call): no per-figure projection, wrap
splitting or `Visible` on the CPU any more (`Benchmark_World_Draw` −35…−50%). One WGSL shader
(`render/shaders/compose.wgsl` over the library and every material registered with
`render.RegisterMaterials`) draws every piece; `Frame.Line`/ `Soft` fade their edges; a piece
across cells takes the place of its nearest end. A
`render.Still` is a picture composed once in world units **in the order its pieces come** (no sort:
the board composes tiles and what lies on them, then `Dressing.Across`, the bands and the ways, in a
second round), kept on the GPU (`gpu.Kept`, its vertices holding their heights), drawn every frame
scaled, moved and lit (`gpu.Draw.Place`/ `Tint`), into the depth where the target has one.
`render.Frame.Time` is the clock's `Shown` (`render.Clocked`), so animations move every frame. A
plugin adds telemetry lines through a `render.Reporter`
(`telemetry.NewPlugin(w).With(p.Reporter())`, the telemetry panel). `GRAM_SHOT=<file.png>` (and
`GRAM_SHOT_FRAME`, 120) has a game save that frame and quit, the world taking one step a frame till
then: shoot a baseline tree and the working one and compare (the telemetry panel and random weather
or scenes are noise).

**Frame images** (2026-10-16, stage 6): what every shader of a frame reads that is too big for
uniforms — 4096 bytes a draw in all (`gpu/device.go` `uniformBlock`), the shared set ~600 of it;
**the terrain's block is at the edge of what an Intel GPU pushes in registers**: four vec4
uniforms more cost it 1.5 ms a frame, 184 to 144 FPS (2026-10-20), so a small table goes into
a texel the shader loads (the details' header row), never into uniforms
(no shader holds a big array any more: the terrain's blocks are instances) — is an image the frame
hands like a uniform. `render.RegisterImage
(name)` in a package's init (panics once a shader is built, twice, past `gpu.FrameImages` = 8) has
the library declare `@group(3) @binding(k) var <name>: texture_2d<f32>;`; WGSL reads it by name,
through `linear` (clamped) or `wrapped` (group 3's sampler, repeating: a ring buffer's seam). A
source's Compose hands it, `f.Image(name, img)`; the composer boxes it with the uniforms
(`c.boxed`, nil where a frame set none), so every Direct gets it through `u.Into(opts.Uniforms)`
untouched, `Shader.pack` skips it and `DrawMesh`/`DrawTrianglesShader` bind it (`frameOf`); the
gpu side is `Draw.Frame`/`MeshDraw.Frame`, one cached group a set of textures. An image whose
pixels need the GPU — baked, or written from the CPU — is `img.Defer(fill)`: filled the next time
a draw reads it (frame image or image slot), so a Compose without a device (tests) never touches
the GPU. Uses: `GroundDetail` (the terrain's details sheet, `painter.Details`, handed in its own
`opts.Uniforms` by name, as a Direct may hand an image of its own to its draw alone), `CloudTile` (`air.Tile()`, baked once, read by the sky and the terrain through
`cloudTileAt`); `AirMasses` (`air.DomeImage.Of(dome)`, held by the sky's renderer, each level's
cells in order so one filtered sample reads a point, a level written as its window moves or a
patch of it changes, read by `airAt(p)` in every cloud shader: the clouds' cover and billow are
the mass's at the point, never one number for the world); `GroundHeights`, render's own
(`Frame.GroundHeights(render.GroundHeights{...})`, uniforms `GroundLattice`, `GroundSpan`, WGSL
`groundHeightAt(p)` blended between the corners, `groundHighest()`, `groundNone` in a frame
without it), handed by whoever draws the ground (the terrain: its lattice image, 16 bits in red
and green, the heights as the last frame drew them), read by the clouds from above to hide a
slice inside the ground at a few loads a pixel instead of the ground's depth (16 bits in two
channels do not blend in the sampler: four loads, so read it at a coarse resolution, never per
screen pixel). The sun on the ground is the second: `Frame.GroundShade(render.GroundShade{Now,
Was, Area, Span, Band, Mix})`, images `GroundShade` and `GroundShadeWas`, uniforms
`GroundShadeArea` (its width 0: none), `GroundShadeSpan` (span, band height), `GroundShadeMix0..3`
(16 bands' fades); WGSL `groundShadeOf(p)` (the texel, each band faded from `Was`; `(1,0,1,1)`
without it), `groundShadeAt(p, over, soft)` (the relief's shadow at a height over the ground),
`standingShadeAt(p, over, soft)` (the blocks'); handed by the light plugin, which never binds it
while baking into it (it deletes both from the bake's uniforms) and reads the heights as the mesh
draws them (`groundDrawnAt`, `groundSplit`). What a drawn thing takes of the shade is its
`render.Taking` bits (`takes(t, bit)`, `takesRelief|takesBlocks|takesClouds|takesLamps`; `Shines`
none), carried in a sprite's instance with the sun's share of its light (`Sprites.Take`, `Sun`;
`render.Illumination{Light, Lamps, Direct, Takes}` as the world's `Lighting` hands it), honoured by
`sprites.wgsl` (the flat sprites, from above in relief too), `billboard.wgsl`, `lying.wgsl` and the
grounds. `DrawMeshOptions.Multiply` (gpu `Multiply`, dst × src) lays a shade or a glow over what
is drawn. **A frame image read per pixel is
read once**: let the sampler blend (lay the picture so it can), never four `textureLoad`s and
arithmetic by hand. gogpu
binds a group through the bound pipeline's layout, all ours share one, so `bindFrame` sets group
3 once a pass till it changes. **Pixels written into an image are drawn in place**: up to 65536
(`gpu.maxPut`), a part or the whole, `Texture.WritePixels` draws them from the draws' uniforms
(the `put` program, 1020 pixels a draw) among the frame's draws, after every draw made before,
with no submission. gogpu's own ways are traps: `Queue.WriteTexture` discards what the texture
held (an UNDEFINED transition) and needs a submission of its own (~1 ms mid-frame: a dozen a
frame took topography from 160 to 11 FPS); a copy from a buffer into a texture drawn in the same
commands is refused (a usage conflict; gogpu's tracker treats a whole encoder as one scope). A
bigger write still goes those ways: the whole texture after a submission, a part through a
texture of its own. The gathered buffers grow to what a frame gathered, so a frame of many
draws is submitted once.
Image slots of a draw stay its own (4, group 1); the terrain uses all four.

## The plugins' renderers and styles per kind

A scene names nothing that draws and no atlas (2026-10-14: `WithRenderer` went; 2026-10-15:
`Renderer()` went): every plugin makes its sources in `NewPlugin` or `Install` and gives them
`ctx.Draws`; the world's and the board's are dressed by `ctx.Looks` once the Stage is set up
(`NewAtlas().Looks().Close()`, the board's sprites the cell's bounds across; `Close` hands the
atlas to `SetAtlas`, which a test may call with a foreign `render.AtlasSource`). A game declaring
looks by hand closes its own `NewAtlas` in a section before. The user's word (2026-10-14): an
atlas made and closed at once in a scene is ceremony; a size given by another channel than the
constructor is noise. A figure's `Look` styles what the plugins draw about its kind with
`Slot.Styled(s)` — `selection.Highlight{Line, Width}`, `vision.Cone{Color}`,
`navigation.RouteStyle{Line, Goal, Width}` — kept in `render.Styles` on the world's atlas
(`world.Plugin.Styles()` once it is closed), read with `render.StyleOf[S](styles, sprite)`; the
renderers carry the colour per instance on the GPU. `board.ShowGrid(bool)` is the grid option;
the grid's line and the holds' width are the board's surface (`board.Surface`, `Atlas.Grid`,
`Atlas.Holds`): the board's `SetAtlas` copies them into `Res.Render`, which stays nil before
(`RenderState.Grid()` reads nil as no grid), and the flat grid, the terrain and the hex prisms
draw `rgb·(1 − a·e) + line·e`, e a line `Width` pixels wide fading over a pixel either side.

What each renderer draws is a look of an entity under a state, not of the plugin: navigation the
routes and goals of an entity's `MoveOrder`; vision the cone of an entity's `Sight` (`WithStyle`,
`WithShadow`; `WithViews(render.Show(…))` picks whose); selection the outline of the `Selected`
tag through `world.Look.Footprint` plus the marquee, a player's gesture; bridges the bridge in
hand, a future entity drawn from its deck's slot; collision the boxes; the board the cells held,
under a `Lighting` its `Still` drawn as it is and the frame's light multiplied over it right
after (`look/shade.go`, 2026-10-19) and, under an atmosphere (`board.WithAtmosphere`), its clouds'
shadows as part of its ground (2026-10-15, stage 4: `look.GroundDressing` drawn after, in the
opaque pass, `board/internal/draw.Clouds`, the 8-px mesh the atmosphere's `overcast` was; what
lies over the ground is drawn unshaded, as the relief's terrain shades its own ground).

**The sky's colours come from one place** (2026-10-15): the atmosphere's backdrop hands every
frame the sun (`sky.Sun.Frame`) and the weather (`air.Weather.Frame`: `Fog` the vault's horizon,
`SkyOverhead` its zenith, WGSL `skyAt(up)` between, which the vault and the relief's water both
use); topography under no atmosphere hands its still sky through a source that gives way to one
already on the frame (`Frame.Uniformed`). The light by the sun's height is `sky.Config.Daylight`
(`EarthDaylight()`), its sun colour on `Heavens.SunColor`; the heights' thresholds are
`celestial.Up/Risen/Gone/Night`.

**The sky is the game's look** (2026-10-16, stage 5): a `backdrop.Dome` of four WGSL parts
(vault, sun, moon, stars) assembled into one shader with the clouds; a game's `atmosphere.Sky`
sets the parts it draws its own way, the Earth's in the rest. The vault is handed `s.air`, the
air's colour the fog and the water share: a vault far from it at the horizon shows a seam with
the ground's fog, and the water reflects the air, never the vault. gogpu's compiler (naga
v0.19.0 and v0.19.1) leaves a WGSL `var` without an initialiser undefined (garbage, not zero: a
`var c: CloudOver;` returned early by a branch came out as stray full shadows in single pixels,
2026-10-16; write `var c = CloudOver(...)`) and
reads a module-scope vector `const` as zero where it meets a runtime value (`c * 1.0` folds and
works, `c * light` is zero): gram's own WGSL has no vector consts; write them inside the function
or as scalars (the low clouds' bands came out at the ground, 2026-10-17, till they were). Its
SPIR-V writer has no `all()`/`any()` ("unsupported expression kind: ir.ExprRelational", a panic
at the first draw): test a vector by its components (`max(max(abs(v.x), …)) <= 0.0`).

**A mesh may be drawn by ranges of its indices** (`DrawMeshOptions.Ranges`, none all): one
call, one setup of the pass, a `DrawIndexed` a range; the relief draws the sectors a camera sees
so. A call a run cost a setup each, twice when `Primed`.

**A depth buffer is read at the size it was drawn at** (`DrawMeshOptions.ReadDepth`, `depthAt`
in the pixels of the texture it was drawn with, 2026-10-17): a Direct may draw a coarse pass with
a Readable depth of its own (`frag_depth`) and read it laying the result over the screen
(aloft's clouds lie as deep as they begin). Till then `gpu.Mesh` refit the buffer read to every
target reading it — a new texture each draw, its content lost (read as 1, nearest), ~2 ms a
frame for aloft's two passes; the ground's copy (`t.Ground`) is the screen's size and unchanged.

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
