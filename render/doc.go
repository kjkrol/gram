// Package render is the drawing side of gram: pure primitives that know nothing of Stages,
// Scenes or plugins. A Scene's Layers and a plugin's Renderer are made of these.
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
// An [Atlas] is a sprite sheet built lazily: Register (or RegisterAt, into a slot issued
// elsewhere) records a [SpriteDrawer] at a texture size of its own, and Close is when the sheet
// is laid out and baked — so a slot issued late is as welcome as an early one, as long as it
// comes before Close. The drawn size is the entity's box; the texture size is resolution.
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
// sheet — a colour its white texel — so a run of pieces on one sheet is one DrawTrianglesShader
// call. Ties keep the order pieces came in. [Frame.SpriteBlend] draws a sprite only where a weight
// blended between its corners is over a half: one ground running into another along a line, not
// along the edges of a quad.
//
// What is worked out per pixel beyond that — water, the clouds' shadows — is a material a plugin
// brings in Kage of its own and registers ([RegisterMaterials]); the composer's one shader is its
// own part and every material registered, put together and compiled once ([Compile],
// [ShaderSource]). [Frame.Overlay] lays over the sprite just added a quad for a material to work out,
// where its corners lie in the world ([World], [Box]) and what the material reads at each —
// [Frame.OverlayOn] over one added earlier ([Frame.Last]), over all drawn on it since;
// [Frame.Material] lays a material's quad on its own. A material declares its own uniforms in its
// Kage and a source sets them ([Frame.Uniform]); the composer's own are Toward, the way towards
// the eye, Clock, the frame's time ([Frame.Time]), Pixel, the world units a pixel spans, and Fog,
// the colour what lies far off turns to as much as a plain sprite asks ([Frame.Fog]). What the
// light, the weather or the ground are is no business of the frame's: the sun and the air are
// plugins/atmosphere's materials, water the topography's. [Frame.SpritePart] draws any part of a
// sheet, and [Paint] paints a frame's sprites once into an image of one's own, through the same
// shader: a sheet painted once and drawn from every frame. [ProjectCorners] projects a world box
// at a height through a camera; [VisitWrapImages] visits each image of a box on a wrapping world.
package render
