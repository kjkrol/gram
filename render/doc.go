// Package render is the drawing side of gram: pure primitives that know nothing of Stages,
// Scenes or plugins. A Scene's Layers and a plugin's Renderer are made of these. Everything is
// drawn on the GPU through WebGPU (render/gpu over gogpu), in WGSL shaders.
//
// # Layers: Renderer and WorldRenderer
//
// A Scene's layers are [Layer]s, Init once, at registration. A [Renderer] draws on the screen once
// a frame, in screen pixels: a background, a menu, a telemetry line. A [WorldRenderer] shows the
// world: the engine draws it once a frame per [Viewport] of its scene — a camera and a rectangle of
// the screen — handing it that camera, so one renderer serves a player's view, the halves of a
// split screen and a minimap alike; no renderer keeps a camera of its own. [Whole] is the one
// viewport of a camera over the whole screen. [SolidBackground] fills the screen with one color; [CachedRenderer] draws an inner Renderer once into an offscreen image and
// reuses it until Invalidate or the screen changes size, for a board that rarely changes; [TelemetryRenderer] prints the
// frame and tick rates and the entity count, and under them the lines of any [Reporter] it is
// built With — a plugin's own, such as the sky's time of day or collision's contacts a second.
//
// # Atlas and AtlasSource
//
// An [Atlas] is a sprite sheet built lazily: Add records a [SpriteDrawer] in a [Sprited]'s slot
// — a kind's handle, a board's cover, a bare SpriteID — at a texture size of its own, and Close is
// when the sheet is laid out and baked — so a slot issued late is as welcome as an early one, as
// long as it comes before Close. The drawn size is the entity's box; the texture size is
// resolution. Add hands the sprite back as a [Slot]: chain [Slot.Under] for the look drawn in
// its place while an effect holds ([Dresser], which rule/effect's Effect is through its Look).
// [Solid], [Border], [Diamond], [Cross], [Hexagon], [Dot] and [Arrow] are ready-made drawers. [AtlasSource]
// is what a Frame draws from: the sheet, each [SpriteID]'s UV rectangle and a white texel for plain
// colours, which Close bakes in.
//
// # Composer, Frame and Source
//
// A [Composer] is the WorldRenderer of a scene's world: one picture per viewport from several
// [Source]s — the board's, the world's, sight's, the selection's, the routes' renderers. Each
// source hands its pieces to a [Frame] in screen pixels, each with a [Tier] and a depth: a sprite
// ([Frame.Sprite], or [Frame.SpriteRect] over a world box, split at a wrap seam), a line with soft
// sides ([Frame.Line]), a fan ([Frame.Fan]) or a quad fading towards chosen sides ([Frame.Soft]).
// A sprite is drawn in a [Shade], a [Light] — red, green, blue — at each corner, and
// [Frame.Fog] has the sprite just added turn to the Fog's colour as much as each corner says.
// Tiers are drawn in
// order — [Backdrop], [Ground], [Objects], [Overlays], [Marks], with room between for a
// game's own — and when the camera's projection sorts, everything below Marks is drawn back to
// front by depth, ties by tier, so a mountain hides the route and the cone behind it while the
// selection stays on top; otherwise the tier alone decides. Every piece is drawn with one shader, sampling a
// sheet — a colour its white texel — so a run of pieces on one sheet is one call. Ties keep the
// order pieces came in. [Frame.SpriteBlend] draws a sprite only where a weight
// blended between its corners is over a half: one ground running into another along a line, not
// along the edges of a quad. A [Direct] source draws a part of the picture itself, with a shader
// of its own — the ground as a mesh of its heights, the sky, the rain — before the first piece of
// its tier or over and after all before it, handed where to draw ([Target]: the screen and the
// frame's shared [Depth], reversed, 1 nearest, cleared as the frame begins) and the frame's
// [Uniforms]; [NewMeshShaderWith] builds such a shader on the composer's library and materials, its
// own vertex stage drawing [Image.DrawMesh] with depth, instances ([Shader.Instanced]) and the
// frame's depth to read ([DrawMeshOptions.ReadDepth]: what lies under a decal). A nil layer handed
// to the composer is left out. [Frame.Branch] and [Frame.Append] let a source gather pieces on
// several goroutines and take them back in order.
//
// # Still and Sprites
//
// A [Still] is a frame composed once in world units — through a camera drawing a world unit a
// pixel — and kept on the GPU run by run, drawn every frame scaled and moved as a camera from
// above shows the world and in a light of its own: a flat board's tiles, composed anew only when
// the board changes. [Sprites] is a Direct source's sprites drawn as instances, one call a run
// sharing an atlas, piece for piece what Frame.SpriteRectUV would lay.
//
// What is worked out per pixel beyond that — water, the clouds' shadows — is a material a plugin
// or a game (examples/material-demo)
// brings in WGSL of its own and registers ([RegisterMaterials]); the composer's one shader is its
// own part and every material registered, put together and compiled once ([Compile],
// [ShaderSource]). [Frame.Overlay] lays over the sprite just added a quad for a material to work out,
// where its corners lie in the world ([World], [Box]) and what the material reads at each —
// [Frame.OverlayOn] over one added earlier ([Frame.Last]), over all drawn on it since;
// [Frame.Material] lays a material's quad on its own. A material declares its own uniforms beside
// its WGSL and a source sets them ([Frame.Uniform]); the composer's own are Toward, the way towards
// the eye, Clock, the frame's time ([Frame.Time]), Pixel, the world units a pixel spans, and Fog,
// the colour what lies far off turns to as much as a plain sprite asks ([Frame.Fog]). What the
// light, the weather or the ground are is no business of the frame's: the sun and the air are
// plugins/atmosphere's materials, water the topography's. [Frame.SpritePart] draws any part of a
// sheet, and [Paint] paints a frame's sprites once into an image of one's own, through the same
// shader: a sheet painted once and drawn from every frame. [ProjectCorners] projects a world box
// at a height through a camera; [VisitWrapImages] visits each image of a box on a wrapping world.
//
// # Appearance and Rules
//
// [Appearance] is the sprite an entity is drawn from and how it sways. A [Rule] says, every frame,
// how the entities a renderer draws are drawn, each reading one component T of the entity: [Over]
// lays a sprite on top of one carrying T, [As] draws it as another, [Swap] as the twin of its
// sprite from a table — a kind's own look under a state, the table a sprite a kind — [With]
// reworks its sprite through a function of T, [Show] leaves out those it does not hold for; each
// takes conditions of T (a tag's In for a tag carried, an effect's Mark().In for a state). Being
// no part of the game, they are written in Go — the one place a rule is. A renderer runs them
// through [Rules]: Bind adds what they read to its query, [Own] shares a column it reads itself,
// Run settles each chunk. The world's renderer takes them (world.Plugin.Draw), the views of
// vision too.
package render
