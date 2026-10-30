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
- Passes: the **opaque** ground (depth written, any order); the world **seen through**, back to
  front by key, depth-tested (its depth worked out on the GPU from its height; a piece across
  cells at its nearest end's place); what is `At.OverFigures()`; the **marks** by level,
  untested, at one level in the order added.
- Lifts over a surface, low to high: a ground dressing 0.02, board materials 0.05, grid 0.1,
  holds 0.15, sight 0.25, routes 0.75, route goals 0.8, a bridge in hand 0.9, figures
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
- A `Direct` asks in Compose where it draws: `DrawOpaque`, `DrawAt(at)`, `ReadGround` (it gets
  `Target.Ground` and writes `frag_depth` = the ground's own); one asking nothing is not drawn.
- Frame images (`render.RegisterImage`, `f.Image` in Compose) are read in WGSL by name through
  `linear` or `wrapped`; the light never binds `GroundShade` while it bakes into it. A new shader
  honours a drawn thing's `render.Taking` bits.
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
  are a `render.Glass`, drawn after the whole screen, so the HUD never covers them (2026-10-20).
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

- gogpu's `Queue.WriteTexture` discards what the texture held and needs a submission (~1 ms
  mid-frame: a dozen a frame took the topography from 160 to 11 FPS); a buffer copy into a texture
  drawn in the same commands is refused. `Texture.WritePixels` draws pixels in place instead;
  only a bigger write goes gogpu's ways. The gathered buffers grow, so a frame is submitted once.
- A texture copy of a depth is wrong on GLES and Vulkan in gogpu: `Image.CopyDepth` is a draw.
- A depth buffer is read at the size it was drawn at (`DrawMeshOptions.ReadDepth`); refit to each
  reader, it was a new texture a draw, its content lost.
- The SPIR-V cache is Vulkan's alone: Metal takes no SPIR-V (every demo panicked on a Mac when
  every backend got it, 2026-10-27); Metal, DX12 and GL get the WGSL. wgpu keeps the driver's
  pipeline cache on disk itself.
- A fragment shader takes `@builtin(position)` once, in its input struct, never also as a
  parameter: naga and Vulkan let it by, Metal refuses (`wgsl_test.go`). On Metal gogpu ignores
  `@invariant`; a shader writing `frag_depth` should draw with a depth buffer.
- naga leaves a `var` without an initialiser undefined, garbage (stray shadows in single pixels):
  write `var c = T(...)`. Before v0.20.3 it read a module-scope vector `const` as zero against a
  runtime value, miscompiled `+=` on a local array's element and took no `all()`/`any()`
  (`compile_test.go`); gram's WGSL still tests a vector by its components. Re-check at a bump.

## Dropped

- Projection, wrap splitting and culling per figure on the CPU: the sprites are laid on the GPU.
- A one-byte `SpriteID`: the wire demo's figure alone took nearly 256 sprites.
