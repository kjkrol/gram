---
name: gram-render
description: How gram draws — the Stage's one picture (ctx.Draws, Publisher, Bound), order by Z (At, lifts, passes, mark levels, OverFigures, Overlying, Glass), layers (Laid), Direct sources with their own WGSL, frame images, looks under effects, timed looks, styles per kind, the plugins' Look seams; the invariants and the GPU's, naga's and Metal's traps. Use when touching render/, an atlas or a look in a plugin or a demo's scene.go, a shader, a renderer, or when something is drawn wrong.
---

# gram-render

`render` draws knowing no Stage, Scene or plugin; its API is `render/doc.go` and godoc, the
world's looks `plugins/world/doc.go` ("Drawing and Look"). The house rules — a scene names nothing
that draws, Z alone orders, render lights, styles and surfaces in a figure's `Look` — are
CLAUDE.md's; what a frame costs is `gram-perf-picture`.

## Model

- The packages of `render/internal` reach each other through exported functions, never methods
  (`gfx.GPU(img)`), so no alias in `render` shows them; their tests never import `render`.
- The Stage's world is one `render.Composer`, `world.Read(0).Picture()`, of the sources its
  plugins gave `ctx.Draws`; a demo that makes a plugin draws all it draws. A `plugin.Bound` source
  is readied once however many scenes show it; a `plugin.Publisher` publishes once a frame after
  the ticks and swaps a `render.Buffered`: drawing reads the front alone, never live state.
- A feed draws, crops and pins through the camera's copy (`camera.Snapped`, taken as the feed is
  laid out on the barrier); the camera the ticks move is never cropped; what is keyed by a camera
  is looked up by `camera.OriginOf(cam)`.
- Passes: the **opaque** ground (depth written, any order); the **uncovered**
  (`render.Uncovered`, `Frame.DrawUncovered`, 2026-11-18: Directs drawing what must show under
  what covers it into a depth of the composer's own, `Target.Uncovered` in the passes after;
  only in a frame with a depth, the buffer made as a source first asks, one a size and layer
  as the other depths); the **cutouts** (`render.Cutout`,
  `Frame.DrawCutout`, 2026-11-09: Directs drawing what stands whole, depth written, cut at half
  the alpha, any order; after the ground's depth is copied for what lies over the figures, so a
  tree never hides an `OverFigures` piece, and in `DrawOver` too); the world **seen through**,
  back to front by key, depth-tested (its depth worked out on the GPU from its height; a piece
  across cells at its nearest end's place); what is `At.OverFigures()`; the **marks** by level,
  untested, at one level in the order added. `render.Sprites.Cutout(on)` names the pieces of the
  cutout pass; `Draw` partitions by `Target.Pass`, the cutouts front to back for early Z, the
  points in the pass seen through alone; `Cut` in `sprites.wgsl` is the half-alpha test, only
  with a depth. The billboards partition their batches the same (`batchOf(atlas, cutout)`).
- **What gives way** (2026-11-18, in place of the world's canopies): a source draws a disc
  round each thing it has uncovered, in the pass of the uncovered, as `1 - depth` (the
  library's `uncoveredAt`: `GreaterEqual` then keeps the farthest over a pixel, and a cleared
  0 is nothing), in an ordered pattern of pixels thinning outwards (`uncoveredOpen`), the
  colour left (a zero premultiplied colour). Whatever gives way carries the library's
  `givesWay` (256) past the `Taking` bits in the float they ride in (no instance had a float
  to spare; a whole body's card has no varying either, so its haze carries it as two more),
  reads `depthAt` (the draw's `ReadDepth`, `givesWayAt`) and is discarded where it lies
  nearer, in the cutout pass and the one seen through alike, so every piece over a pixel opens
  at the same pixels and whatever number of them shows what is under: no sorting, no
  blending, nothing asked of the space on the CPU. It reads only within the rectangle the
  uncovered lie in (`Target.UncoveredIn`, grown by each source in its pass; `render.Sprites`
  tells it an instance, in the vertex stage). `render.Sprites` does both from above
  (`uncovered.wgsl`), the topography's `sprites` in relief (its own `uncovered.wgsl`: the
  billboards, the cards, the bodies whole, what lies). **A depth is told apart along one line
  of sight alone**: through these cameras it is the height, so a disc given its middle's depth
  at every pixel opened what stood behind it and higher; a disc lies in its own plane facing
  the eye, half its reach before the thing, and carries that plane's depth pixel by pixel.
  A plugin's own solid gives way the same (`world.Write.GivesWay`, 2026-11-19: the buildings
  near): the world lays the uncovered then though no look of its own gives way, the source
  asks `Read.GivesWayDrawn(cam)` in Compose and discards by `givesWayAt`; a room an uncovered
  unit stands in goes whole into the uncovered depth, so all over it opens.
  The topography's disc is left out where the ground hides the thing's middle from the eye
  (`hidden`: the ground's heights met along the way towards the eye, half a cell a step in the
  vertex stage), or what stands before a hill would open on the hill. What is drawn with the
  ground, in its layers and before the figures are composed (a plugin's solids), cannot read
  it.
- Lifts over a surface, low to high: a ground dressing 0.02, board materials 0.05, grid 0.1,
  holds 0.15, sight 0.25, a field's piece 0.5, a bridge in hand 0.9 (routes and goals are
  marks `OnTop` 0.5/0.75 since 2026-11-04), figures
  `world.FigureLift` 2 (topography's `spriteLift` the same), a box `FigureLift`+0.5. Mark levels:
  the selection's outline 1 and marquee 2, the cameras' layers 4. Goals, holds and boxes lie in
  the world, never as marks.
- Layers (`render.Laid`): `Lay` composes every source once in the camera's pixels; `DrawLayer`
  leaves out the inner layer's window (the nearest depth filled there), each layer's depth kept
  by size and layer index; `DrawOver` draws the `Overlying` sources against the placed depths,
  inheriting the ground pass's uniforms and images, its Directs handed `Target.Coarse` and
  unheard asking `DrawOpaque`.
- One shader, `compose.wgsl` over the library and the registered materials, draws every piece;
  `render.Sprites` lays the figures' rectangles on the GPU, copies at a wrap's seams.
- A `Direct` asks in Compose where it draws: `DrawOpaque`, `DrawCutout`, `DrawAt(at)`,
  `ReadGround` (it gets `Target.Ground` and writes `frag_depth` = the ground's own); one asking
  nothing is not drawn. The world renderer asks `DrawCutout` once any figure is a cutout (every
  drawn layer `Slot.Cutout`, not coarse), `DrawUncovered` once it lays a figure uncovered, and
  `DrawAt(lowest)` for the rest; cutouts never move `lowest`; `Draw` in the cutout pass and in
  that of the uncovered draws the sprites alone, no points or labels.
- The ground's shade (`compose/bake.go`) is one image over the whole relief, baked by two lanes
  into the same pixels: the sun's turn by 16 bands with a fade (`GroundShadeMix`), a band once
  the sun has turned 1/16° since the last (`shadeStep`), at most one a frame, a leap over 1° (the
  hour jumped) baking them from the top one a frame, a relief's new version, a span change and
  the first bake all at once (2026-11-08: a band a frame over 31 000 blocks cost a third of the
  frame rate through the iGPU's shared power budget); and the content by tiles of 256 px queued
  where the shadows of a changed set of blocks (`Lights.Casts`, a set a sector, versioned by the
  light) reached before and reach now, at most 8 a frame, no fade. A shader reading the shade
  reads one image and the bands' mixes, so a change of the bake changes no shader. A lying thing
  (a bridge's handle) is one block (2026-11-07). Every shadow — live, baked, a cloud's, the sun's
  or a lamp's — reaches at most `render.ShadowReach` (6) heights of its caster (2026-11-10, a
  rule of the house): the one constant, in Go and as `shadowReach` in the shaders that need it;
  a baked block's quad also ends where its shadow's line meets the ground, found by a march like
  the relief's, never at the map's lowest point. The distant light on a surface facing up and
  its share are worked out once a frame in `lighting.begin`, the lamps per sprite only within
  the box of their reach (2026-11-08).
- Frame images (`render.RegisterImage`, `f.Image` in Compose) are read in WGSL by name through
  `linear` or `wrapped`; the light never binds `GroundShade` while it bakes into it. A new shader
  honours a drawn thing's `render.Taking` bits. Nine names, all in use (the ninth `Shelters`,
  render's own, 2026-11-19: with a draw's four and its depth, 14 of the 16 a stage samples on a
  device's default limits; a tenth needs a name freed or the limits asked); a draw's own image
  slots are four, so a renderer short of them binds a frame image per draw through
  `opts.Uniforms` (the terrain's `GroundDetail`); the same name carries the L4 image or its L5
  half by the draw. No mipmaps (`MipLevelCount` 1, no `MipmapFilter`): a smaller copy is
  drawn (`DrawImage` at 0.5 with `FilterLinear` is an exact 2×2 mean; its GeoM counts in the
  texture's pixels, so a `SubImage` target is translated by `Bounds().Min`) or made by a shader.
  A height in an RGBA8 image goes as f16 through `pack2x16float` (naga 0.20.5 has it on every
  backend), the low byte in R, the high in G, read back with `textureLoad` (the sheet's relief);
  a float32 goes whole in a texel's four bytes, read back
  `bitcast<f32>(pack4x8unorm(textureLoad(...)))` (the shelters' rows).
- **A thing under a roof is lit on the GPU** (2026-11-19, the user's word): `Lights.Shelters`
  laid in the frame's `Shelters` image as their version moves on, a row a shelter; its place
  and one more ride in the float of a Taking past `givesWay` (×512, `shelterOf`); such a
  thing's instance carries the frame's lamps alone, `Illumination.Sheltered`, and the library's
  `shelteredLight` adds at its pixel the sun where the way to it leaves by an opening, the sky's
  and the thrown round's shares and the shelter's lamp. Every shader lighting a figure takes it:
  `render.Sprites`, the topography's billboards and lying things a pixel at a time, the bodies a
  part at a time (no varying to spare); the far points stay in the open's light.
- Navigation's route line and goal are marks `OnTop` (route < goal < the selection's outline),
  never lifts in the world (2026-11-04); the GPU route lines `Untested` with `ReadGround` kept,
  so the ground still hides a route behind a hill while what stands over it does not.
- The Look seams beside `Sprite`: `FoggyLook.Fog`, `UncoveringLook.Uncover(on)` and
  `GiveWay(on)` (told a layer, from the atlas's `Slot.Uncovered` and `Slot.GivesWay`, only
  before a camera the world's settings let it: read once a frame at the publish,
  `figures.covers` and `givesRiding`, never in Compose, which reads the front alone) and
  `CutoutLook.Cutout(on)`, each set before a figure and reset after it, the zero value whole
  (a look never begun must draw whole).
- A field aggregate's piece (a canopy sheet at L4) lies on the ground at `look.PieceLift` (0.5),
  under every figure (`FigureLift` 2): the point and material paths place it so; through a Look it
  goes `Lie`, never `Upright`/`Lay`, still at the Look's own lift (an interface change to pass a
  lift is open, 2026-11-04).
- What lies under the ground is a Direct of its plugin (`underground/renderer.go`, `tube.wgsl`):
  a stretch an instance (three vec4: its ends on the floor, half its width, its room, its walls
  standing), the faces built in the vertex stage from the vertex index, opaque, lit by a share
  of the air's light, the frame's lamps and its own strips under the roof, none of the sun. The
  ground over a mouth is the terrain's to cut (`mouthed` in `mesh.wgsl`, in `fs_prime` too, so
  the primed depth has the hole).
- A stage is readied before its first frame (`render.Prepare`, then a frame drawn and thrown
  away): nothing is compiled or baked in a frame shown.

## Atlases and looks

- Under two effects at once an entity is drawn in the look declared later (`Under` order).
- A periodic cosmetic (a blink every 3 s) is a timed look, `render.Timed`, never an effect; two
  periods at once are one round of their common multiple.
- `render.MaxSprites` counts every frame of every way under every effect; past it `NewSprite`
  panics.
- A style (`Slot.Styled`, `render.StyleOf[S]`) falls back to the plugin's default; its colour
  goes per instance on the GPU.
- **A version a level** (`lod`): a kind's table is
  `{own, own, own, own, blob, nothing, nothing}` by default, the blob `render.Blob` of its L2
  drawer's mean colour (a disc by its coverage) made at `Close`, one sprite a kind;
  `Slot.At(level, look)` declares a look from that level out to the next declared,
  `Slot.Nothing(level)` vanishes from there; `Standard` (L2, the kind's own look) and `Omitted`
  (L6) refused (L2 would drop the looks under effects), timed looks refused (the frame rules run
  at publishing, before the version is picked); every twin, facing and frame resolves to its
  kind's table. The world's renderer picks the version by `view.Level` at the entity's middle
  held inside the view's bounds (an entity half on screen stays drawn), the corners where a
  turned picture cuts it; at L3 no `Lay` edges and no `Upright`; at L4 the figure is a point
  without the look (`Sprites.Point`: one instance, projected on the GPU so every layer gets its
  own pixel, the box's extent clamped to a pixel, alpha the coverage under a pixel, the sprite's
  middle read under 2 px, the distant light without lamps, no cast; overlay layers dropped; the
  haze and the clouds' shade on figures do not reach it). The ground's variants are the
  topography's (`gram-plugins`).
- **A kind with a body** (`world.Body`, `Atlas.AddBody`, 2026-11-18; the rule is CLAUDE.md's): the
  world's renderer hands it to `BodyLook.Body` in a view in relief, whole or as a card by the
  reach its publish took (`figures.reach`; a whole body is a cutout whatever its level), its turn
  the entity's heading or, standing, a hash of its place. The shapes, the paints, the dots and
  the tones are one WGSL, the world's `body.wgsl` (prefix `shape`: the air's clouds own
  `body*`), registered with `render` for every shader (`RegisterMaterials` with no entry, as
  the air's wind is): the photograph's shader (`body_shot.wgsl`) and the look's (`billboards`'
  `body.wgsl`, and `billboard.wgsl` for the cards' tones) call it. Handed over by a Go function
  of the world's, it was the one WGSL in gram passed from plugin to plugin (2026-11-18; the
  rule is CLAUDE.md's). A part is a card on its front, met in the pixel in the body's own
  frame, so its dots turn with it and a card's dots are its body's.
  The photographs are `render.Atlas.AddShot` slots, filled by the sheet's one `Defer`: one depth
  buffer for all of them, cleared before the first, since they lie apart. A shot lays its
  pixels over (premultiplied), so a clear pixel of a nearer part hides nothing: the glow of a
  body's shining paints (`Bodied.Glow`, laid over `Above` with `render.Shines` in a view from
  above) takes its depth whole first, then those paints at an equal depth (2026-11-19). The
  ways' blue holds a cone's height up it in its lower half and a shining paint as a whole one
  (`shapeWaysBlue`): read through the sampler, an edge between the two may shine a pixel. A card is a billboard
  whose instance's last two floats say where the ways out of the body lie on the atlas (one past
  each, 0 a plain sprite); `cardFacing` rebuilds the way out in the frame the picture was taken
  in, the light told in that frame once a vertex (`cardSun`). A card's instance holds its body's
  own feet and height, the shader adding the picture's margin (`shapeMargin`): told the
  picture's lowered foot, the shade was read under the ground and a low body's card stood dark
  (2026-11-18). `Bodied.Card` picks the card: the side nearest the eye's way (its angle in
  eighths of a turn by a cheap arc tangent, `eighths`: the real one a card, thousands in view,
  was a fifth of the frame's composing), or `Top`/`TopWays`, the photograph from above, from
  the steepness `Body.steep` works out a kind. A standing card follows the eye past the cards'
  elevation (`shapeRow`): it leans back about `TopAt` over its foot (the instance's `light.w`)
  by as much as the eye looks steeper, its rows spread as a ball's outline grows on an upright
  plane (`Body.grown`: a spheroid `Wide` across reaching `Tall - TopAt` over that height, exact
  for a ball), its depth still `upright`'s at its feet: what it holds there, a body's head,
  stays as tall on the screen as the body's, and what stands under it, its post, goes under
  the ground as the body's goes under the head.
  Left standing as photographed, the head showed half as tall from 66°; grown on the upright
  plane itself, the same picture in the parallel camera, it was stretched by the perspective
  under a near eye (2026-11-18). It lies from 70° at the most (`steepest`), sooner where the
  card from above is truer (`untrue`: each part's reach up the picture against the body's own,
  by the part's width, nothing of a card showing under the ground: some 60° for a crown on a
  trunk, 50° to 55° for a low wide body, which loses its front standing). Held standing till
  the one from above was twice as true, to keep its trunk, Madeira's forest seen from 65°
  covered less and stood darker than its whole bodies, the lying cards nearer them: a card
  leaning back still shows a crown's side, lit as a side. What a single view cannot give: a
  low body's front under its foot's row is cut by the ground at any elevation, a fir's skirt
  as the eye rises (a second row of views is the roadmap's). The card from above is the same
  billboard told to lie (`shape.z`, the body's turn a whole turn on): held `TopAt` over the
  foot, the middle of what shows from above, turned as the body stands and tipped to face the
  eye, at its own depths, shaded as a part of the body as high. Level, cards as high tied and
  the later instance won: in the parallel camera depth is height alone (`TransformOf` counts
  along the rays from a level plane); at the depths of `upright`, its lower half went under
  the ground (2026-11-18). A body's own sprite, the photograph from the side, has no
  pixels on the CPU: far off it is a blob of its paints' mean. Straight from above, where the
  parallel camera sorts nothing and the flat look draws, the renderer swaps in `Bodied.Above`,
  turned by the body's turn (an `Appearance.Angle` runs against the clock, a turn from x towards
  y: the negative); drawn as its own sprite there, a tree lay on its side (2026-11-18).
- How things lie is a plugin's swappable `Look` (`world.Look`, the board's `look.Look`), flat
  from above by default, the topography setting both for relief; renderers keep their data and
  ask it for geometry alone. What a renderer draws is a look of an entity under a state (a
  `MoveOrder`'s route, a `Sight`'s cone, the `Selected` outline), not of the plugin.

## Invariants

- **Z alone orders the world** (the user's word, 2026-10-15). Equal Z: the later wins
  (GreaterEqual everywhere); among the figures the lying first, so the standing cover them. A
  frame without an opaque ground (a flat world) has no depth: the order decides.
- The figures are drawn once among what is seen through, at the lowest figure's place; their
  visible pixels (alpha over half) write depth, so a decal after and behind them hides.
- `OverFigures` is drawn after every figure against the ground's depth alone: a collision solid
  shows through its own billboard, a hill before it hides it.
- **A source whose pieces lie over the figures says so** (`render.Overlying`). The cameras' zones
  are a `render.Glass`, drawn after the whole screen, so the HUD never covers them (2026-10-20);
  nothing of the world is: its counts of far units were a Glass till 2026-11-08 (`f/counts`) and
  are `render.Labels` now (`render/internal/instanced/labels.go`, `labels.wgsl`: glyph pieces
  and badges from one instance stream at a world point, a fixed size in shown pixels, the same
  projection as the L4 points; `Font.Draw` shares `lay`/`ready` with it). A label stands upright on
  its point (`f/badge`, 2026-11-08): the vertex shader gives each corner the depth of its height
  over the point from how many pixels a world unit rises there, so what stands behind the point
  hides nothing of it and what stands before it still does. A badge (`render.BadgeDisc`,
  `BadgeDiamond`) is a distance field in the fragment stage, a soft pixel at its edge, the rim
  blended in, its data flat varyings (no uniform, no indexing: Metal); `render.BadgeWidth` grows
  it to hold its number, measured on the publishing goroutine with the number face's own copy
  under a mutex (`text.NumberInk`), the drawing taking `text.NumberFont()`, so the two goroutines
  never share a font. `Labels.Text` takes a `color.RGBA`: through `color.Color` it allocated a
  label a frame.
- **A Direct draws once a layer and keeps what Compose gathered**: reset in Compose or `Begin`,
  never in `Draw` (`Sprites`, the routes and vision's views each broke it: the outer layer alone
  had them).
- **A frame image read per pixel is read once**: lay it so the sampler blends, never four
  `textureLoad`s and arithmetic. `GroundHeights` (16 bits in two channels) does not blend: read
  it at a coarse resolution, never per screen pixel.
- **Small tables go into a texel, never the uniforms**: the terrain's block is at the edge of what
  an Intel GPU keeps in registers (four vec4 more cost 1.5 ms a frame); no shader holds a big
  array.
- `ReadGround` is dear (a readable opaque pass, ~0.6 ms on the topography): what needs only where
  the ground stands reads `GroundHeights`.
- **A colour given to gram is straight alpha** (2026-10-14; Go's `color.RGBA` is premultiplied):
  a new path from a colour to floats multiplies R, G, B by A, or the colour glows.
- Pixels that need the GPU are `img.Defer(fill)`: a Compose without a device (tests) never
  touches it.
- A Direct boxes its uniforms once (`own map[string]any`) and puts them after `u.Into` every
  frame: a `[]float32` put into a map is boxed anew, an allocation a call.
- All pipelines share one layout, so `bindFrame` sets group 3 once a pass. A draw has four image
  slots of its own, the terrain using all four. A mesh drawn in parts goes by
  `DrawMeshOptions.Ranges`, one call.
- **The sky is the game's look** (a `backdrop.Dome`'s parts an `atmosphere.Sky` sets) **and its
  colours come from one place** (2026-10-15, `skyAt`): a vault far from the air's colour at the
  horizon shows a seam with the fog; the water reflects the air, never the vault; the clouds'
  cover is the air mass's at the point, never one number for the world.
- `render` and `ui` are to leave gram as a module of their own (`kjkrol/uxgpu`): they import
  nothing more of gram than they do now.
- An atlas made and closed at once in a scene is ceremony; a size given by another channel than
  the constructor is noise (the user's word, 2026-10-14).

## Traps

- naga up to v0.22.6 leaves what a `bitcast<T>(...)` reads unresolved where it is declared later
  in the module (gogpu/naga#261: its dependency order skips a bitcast's argument): `SourceWith`
  declares the frame's images before the library. Re-check at a bump.
- **The world's y runs south**: its frame is left-handed seen with z up, so the way across a
  picture looking along `ahead` is `(-ahead.y, ahead.x)`, and a cross product written for a
  right-handed frame mirrors it (the bodies' first photographs were mirrors of what the camera
  saw; the test of a fruit on one side of a tree found it).
- A whole body takes its shade as a billboard does, blended between its foot's and its top's:
  taken at a part's own height it falls in the shadow of its own baked block and stands half as
  bright as its card.
- A fragment's word on its depth (`@early_depth_test(less_equal)`) is wgpu's on Vulkan alone:
  `gpu.Program` hands the other backends the WGSL without it. It needs the card on the part's
  front and the depth written clamped under the card's (`min(depth, position.z)`).
- A name in a WGSL given to several shaders must be free in the registered materials too
  (`bodyShade` was the clouds').
- gogpu's `Queue.WriteTexture` needs a submission (~1 ms mid-frame: a dozen a frame took the
  topography from 160 to 11 FPS); a buffer copy into a texture drawn in the same commands is
  refused. `Texture.WritePixels` draws pixels in place instead; only a bigger write goes gogpu's
  way, which keeps the rest of the texture since wgpu v0.34.6. The gathered buffers grow, so a
  frame is submitted once.
- A texture copy of a depth is wrong on GLES and Vulkan in gogpu: `Image.CopyDepth` is a draw.
- A depth buffer is read at the size it was drawn at (`DrawMeshOptions.ReadDepth`); refit to each
  reader, it was a new texture a draw, its content lost.
- The SPIR-V cache is Vulkan's alone: Metal takes no SPIR-V (every demo panicked on a Mac when
  every backend got it, 2026-10-27); Metal, DX12 and GL get the WGSL. wgpu keeps the driver's
  pipeline cache on disk itself.
- A fragment shader takes `@builtin(position)` once, in its input struct, never also as a
  parameter: naga and Vulkan let it by, Metal refuses (`wgsl_test.go`). On Metal gogpu ignores
  `@invariant`.
- WebGPU and Metal make no pipeline of a fragment writing `frag_depth` without a depth (Vulkan
  lets it by: forest-army's minimap, the board's miniature, fell on an M1 alone, 2026-11-19).
  The device draws such a mesh program given no buffer against a stand-in of the target's size,
  untested and unwritten (`device.standIn`, four sizes kept), and a pipeline asked without a
  depth for one panics on every backend, so a test on Linux finds a new way to it.
- One module, one namespace: naga refuses a name declared twice at module scope (since v0.21). A
  program is its fragment on the library, the bindings and every material registered, so a
  function, a struct, a constant, a sampler (`nearest`, `linear`, `wrapped`) and a frame image by
  its name (`Sheet`) are each declared once among them all. A clash shows only in a program that
  joins the two pieces (the sky's material with the bake's blocks): a new module-scope name is
  checked by running the demos, not one package's tests.
- gram's own SPIR-V (`compiled` in `spirv.go`) holds indices in range and bounds loops, naga's
  defaults, and leaves image loads and buffers to the device, as wgpu's own compile does where the
  device checks them (it turns robust access on wherever it can): a `textureLoad` keeps its
  coordinates in range itself. naga's check of every image load is what costs (the comment there);
  the options are part of a kept SPIR-V's name.
- Metal is not run here: a shader's Metal verdict is naga's `msl.Compile` of its `Source()` under
  wgpu's Metal policies (`msl_test.go`), or clang on the output. A freeze there with the GPU
  idle is read on the Mac with `sample <name> 5` and Ctrl+\ in the game's terminal (the Go
  stacks).
- On Metal a submission holds at most `metalPasses` render passes (`device.go`, the check in
  `ensure`): wgpu records two command buffers a pass and its Metal queue gives out 64 before one
  is committed (gogpu/wgpu#428), so a frame of more passes in one encoder froze. The stand-in
  goes with the upstream fix; a pass begun without `ensure` (a clear) is why it stands well
  under the limit.
- At a bump `compile_test.go`, `msl_test.go` and `write_test.go` guard what gram leans on, and the
  island's frame is measured against the tree before (the image loads were the cost at naga
  v0.22); gram's WGSL still tests a vector by its components. wgpu v0.35.0 takes `hal/*` out of
  its public packages: the `allbackends` import in `device.go` goes with it, on a gogpu built for
  that wgpu.

## Dropped

- Projection, wrap splitting and culling per figure on the CPU: the sprites are laid on the GPU.
- A one-byte `SpriteID`: the wire demo's figure alone took nearly 256 sprites.
