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
- Passes: the **opaque** ground (depth written, any order); the **cutouts** (`render.Cutout`,
  `Frame.DrawCutout`, 2026-11-09: Directs drawing what stands whole, depth written, cut at half
  the alpha, any order; after the ground's depth is copied for what lies over the figures, so a
  tree never hides an `OverFigures` piece, and in `DrawOver` too); the world **seen through**,
  back to front by key, depth-tested (its depth worked out on the GPU from its height; a piece
  across cells at its nearest end's place); what is `At.OverFigures()`; the **marks** by level,
  untested, at one level in the order added. `render.Sprites.Cutout(on)` names the pieces of the
  cutout pass; `Draw` partitions by `Target.Pass`, the cutouts front to back for early Z, the
  points in the pass seen through alone; `Cut` in `sprites.wgsl` is the half-alpha test, only
  with a depth. The billboards partition their batches the same (`batchOf(atlas, cutout)`).
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
  drawn layer `Slot.Cutout`, not coarse, not a canopy veiled this frame) and `DrawAt(lowest)` for
  the rest; cutouts never move `lowest`; `Draw` in the cutout pass draws the sprites alone, no
  points or labels.
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
  honours a drawn thing's `render.Taking` bits. Eight names at most, all eight in use (the
  world's `Sheet` and `SheetRelief` the last two, 2026-11-05: a ninth needs a name freed); a
  draw's own image slots are four, so a renderer short of them binds a frame image per draw
  through `opts.Uniforms` (the terrain's `GroundDetail`); the same name carries the L4 image or
  its L5 half by the draw. No mipmaps (`MipLevelCount` 1, no `MipmapFilter`): a smaller copy is
  drawn (`DrawImage` at 0.5 with `FilterLinear` is an exact 2×2 mean; its GeoM counts in the
  texture's pixels, so a `SubImage` target is translated by `Bounds().Min`) or made by a shader.
  A height in an RGBA8 image goes as f16 through `pack2x16float` (naga 0.20.5 has it on every
  backend), the low byte in R, the high in G, read back with `textureLoad` (the sheet's relief).
- Navigation's route line and goal are marks `OnTop` (route < goal < the selection's outline),
  never lifts in the world (2026-11-04); the GPU route lines `Untested` with `ReadGround` kept,
  so the ground still hides a route behind a hill while figures and a canopy do not.
- The Look seams beside `Sprite`: `FoggyLook.Fog`, `CanopyLook.Veil(opacity)` (a tree's crown
  over a unit from above: Flat takes the frame path, `SpriteRectUV`/`SpritePart` then
  `Frame.Glazed`, since `render.Sprites` has no free float for an opacity; `Frame.fog` skips a
  glazed sprite, so a veiled crown takes no sight fog) and `CutoutLook.Cutout(on)`, each set
  before a figure and reset after it, the zero value whole (a look never begun must draw whole:
  the flat look's `hidden` is 0, not an opacity of 1). The veiled set is the publish's
  (`figures.veiled`: a point query of the space over every moving figure's middle when a camera
  sees from above and the setting is on), never worked out in Compose, which reads the front
  alone.
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
- **What stands goes to the GPU once** (2026-11-12): a Direct's instances that change only as
  the map does are a `render.Instances` (`PackInstances` a sector a group, fine then coarse),
  drawn `Kept` in `Spans` joined where sectors follow each other, laid in Publish and put in
  place in Swap, the old one released; `DrawMeshOptions.Instances` is for what changes every
  frame. Sent every frame, Funchal's 55 000 instances halved its frame rate seen whole, on the
  GPU's side more than on the CPU's.
- **The sky is the game's look** (a `backdrop.Dome`'s parts an `atmosphere.Sky` sets) **and its
  colours come from one place** (2026-10-15, `skyAt`): a vault far from the air's colour at the
  horizon shows a seam with the fog; the water reflects the air, never the vault; the clouds'
  cover is the air mass's at the point, never one number for the world.
- `render` and `ui` are to leave gram as a module of their own (`kjkrol/uxgpu`): they import
  nothing more of gram than they do now.
- An atlas made and closed at once in a scene is ceremony; a size given by another channel than
  the constructor is noise (the user's word, 2026-10-14).

## Traps

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
  `@invariant`; a shader writing `frag_depth` should draw with a depth buffer.
- naga's MSL writer leaves a function parameter indexed at run time (`t[k]`, `near[j].sprite`)
  as a bounds-checked ternary without parentheses, which Metal reads as
  `DefaultConstructible() == …` and refuses (naga v0.20.5 still; `msl_test.go` says when it is
  fixed): load such a parameter from a `var` first or take an array by
  `ptr<function, array<T, N>>`; a load from a var or a pointer is baked before its operator.
  Without a Mac, a shader's Metal verdict is naga's `msl.Compile` of its `Source()` read for the
  pattern, or clang on the output.
- Before v0.20.4 naga left a `var` without an initialiser undefined (stray shadows in single
  pixels; gram's `var c = T(...)` stays harmless); before v0.20.3 it read a module-scope vector
  `const` as zero against a runtime value, miscompiled `+=` on a local array's element and took no
  `all()`/`any()` (`compile_test.go`); gram's WGSL still tests a vector by its components.
  Re-check at a bump.

## Dropped

- Projection, wrap splitting and culling per figure on the CPU: the sprites are laid on the GPU.
- A one-byte `SpriteID`: the wire demo's figure alone took nearly 256 sprites.
