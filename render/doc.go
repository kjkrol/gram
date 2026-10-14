// Package render is the drawing side of gram: primitives that know nothing of Stages, Scenes or
// plugins. A Scene's Layers and a plugin's Renderer are made of these. Everything is drawn on the
// GPU through WebGPU (render/gpu), in WGSL shaders.
//
// # Layers: Renderer and Picture
//
// A Scene's layers are [Layer]s. A [Renderer] draws on the screen once a frame, in screen pixels;
// a [Picture] is what is in the world before a camera, drawn through the camera it is handed, so
// one picture serves a player's view, the halves of a split screen and a minimap alike.
// [SolidBackground] and [CachedRenderer] are ready-made renderers; a [Reporter] adds lines to
// the telemetry panel (plugins/telemetry).
//
// # Feed and Surface
//
// A [Feed] is the world seen through a camera: a Picture drawn through the camera every frame
// into an image of the size whoever shows it gives it ([Surface]). It turns its pixels into the
// world and back (ToWorld, ToPixels), so whatever shows it knows nothing of its camera.
//
// # Font and DrawText
//
// A [Font] is a TrueType or OpenType face at a size ([NewFont]); [DefaultFont] is Go Regular at
// 14 pixels. [DrawText] draws a string in a colour, [Font.Measure] is how much room it takes.
// [DebugPrint] is the fixed debug text of the telemetry.
//
// # Atlas and AtlasSource
//
// An [Atlas] is a sprite sheet built lazily: Add records a [SpriteDrawer] in a [Sprited]'s slot
// at a texture size of its own, and Close lays the sheet out and bakes it. Add hands the sprite
// back as a [Slot]; [Slot.Under] is the look drawn in its place while a [Dresser]'s state holds.
// [Solid], [Border], [Diamond], [Cross], [Hexagon], [Dot] and [Arrow] are ready-made drawers.
// [AtlasSource] is what a Frame draws from: the sheet, each [SpriteID]'s rectangle and a white
// texel for plain colours.
//
// # Composer, Frame and Source
//
// A [Composer] is the Picture of a scene's world, drawn from several [Source]s. Each source hands
// its pieces to a [Frame] in screen pixels, each with a [Tier] and a depth: sprites, lines, fans,
// soft quads, each in a [Shade] of [Light]. Tiers are drawn in order ([Backdrop], [Ground],
// [Objects], [Overlays], [Air], [Marks], with room between for a game's own), and when the
// camera's projection sorts, everything below Marks goes back to front by depth. Every piece is
// drawn with one shader, so a run of pieces on one sheet is one call. A [Direct] source draws a
// part of the picture itself, with a shader of its own, where its tier comes, handed a [Target]
// and the frame's [Uniforms]; [NewMeshShaderWith] builds such a shader on the composer's library,
// drawn with [Image.DrawMesh] against a [Depth]. [Frame.Branch] and [Frame.Append] let a source
// gather pieces on several goroutines.
//
// # Materials
//
// What is worked out per pixel is a material, WGSL a plugin or a game registers
// ([RegisterMaterials]); the composer's shader is its own part and every material registered,
// compiled once ([Compile], [ShaderSource]). [Frame.Overlay] lays over the sprite just added a
// quad for a material to work out, where its corners lie in the world ([World], [Box]);
// [Frame.Material] lays one on its own. A [Look] is a drawer or a [MaterialID]. A material's
// uniforms are set by a source ([Frame.Uniform]); the composer's own are Toward, Clock
// ([Frame.Time]), Pixel and Fog ([Frame.Fog]).
//
// # Still and Sprites
//
// A [Still] is a frame composed once in world units and kept on the GPU, drawn every frame
// scaled, moved and lit as a camera from above shows it. [Sprites] is a Direct source's sprites
// drawn as instances. [Paint] paints a frame's sprites once into an image of one's own.
//
// # Appearance and Rules
//
// [Appearance] is the sprite an entity is drawn from and how it sways. A [Rule] says, every
// frame, how the entities a renderer draws are drawn, each reading one component of the entity:
// [Over], [As], [Swap], [With] and [Show], each taking conditions of it. A renderer runs them
// through [Rules]: Bind adds what they read to its query, [Own] shares a column it reads itself,
// Run settles each chunk.
package render
