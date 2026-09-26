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
// tick rate, the entity count and collisions a second from a running total, and under them the
// lines of any [Reporter] it is built With — a plugin's own, such as the sky's time of day.
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
// A sprite is drawn in a [Shade], a [Light] — red, green, blue — at each corner. Tiers are drawn in
// order — [Backdrop], [Ground], [Objects], [Overlays], [Marks], with room between for a
// game's own — and when the camera's projection sorts, everything below Marks is drawn back to
// front by depth, ties by tier, so a mountain hides the route and the cone behind it while the
// selection stays on top; otherwise the tier alone decides. Every piece is drawn with one shader, sampling a
// sheet — a colour its white texel — so a run of pieces on one sheet is one DrawTrianglesShader
// call. Ties keep the order pieces came in. [Frame.Glint] lays over the sprite just added the
// frame's sun ([Frame.Daylight]) thrown back at the eye off small waves the shader runs across it,
// turned to face a [Shore] near one, and the sky reflected the flatter the eye looks: water.
// [Frame.Overcast] lays the clouds' shadows of the frame's weather ([Frame.Weather]) over the
// ground, and [Sway] is how far what sways in the frame's wind leans at its time ([Frame.Time]). [ProjectCorners] projects a world box at a height
// through a camera; [VisitWrapImages] visits each image of a box on a wrapping world.
package render
