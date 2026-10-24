// Package light declares the light over a world: a distant light (the sun) and the lamps, how
// each thing casts its shadow and what shade it takes. It publishes them once a frame
// (render.Lights); render lights the frame by them.
//
// # Plugin
//
// [NewPlugin] makes the light over a world ([Config]: how many lamps nearest a camera cast
// shadows). A game makes it after motion and before the board and the topography, which draw
// by it, and runs it after the world ([Plugin.RunPlan]). From then on the world's entities are
// marked by it as they are published (world.Lighting) and the world's picture is lit by what it
// publishes ([Plugin.Publish]: the sun and the lamps, how it is set, the blocks the ground bakes
// the shadows of, the ground under them; world.Read.Lights for what draws). Its state is read
// through [Plugin.Read], the same on every goroutine: the sources as the clock's Replay left them,
// taken as each Replay ends (clock.Clock.Replayed). It is changed through [Plugin.Write], the
// game's goroutine's.
//
// # Sources
//
// What shines is an entity with a [Source], a knob: a lamp, reaching as far as its Reach from
// where the entity stands, or a [Distant] light, a source of no Reach whose way and strength
// whoever models it writes beside it each tick ([Shining]) — the atmosphere's own entity shines
// the sun of the hour; a scene without a sky has the plugin's own shine one it sets
// ([Plugin.WithDistant], once, or each tick for a sun of the game's own). Two distant lights at once panic. [Read.Distant] is the distant light
// this tick, [Read.Lamps] the lamps; [White] the light of a world without one, everything lit
// as it is.
//
// # Shadows
//
// Every entity casts and takes as its [Shadow] says, a knob its kind declares and an effect may
// turn: a [Patch] or the [Silhouette] of its sprite laid over the ground every frame, a [Baked]
// block whose shadow the ground's shade holds, or [NoShadow]; and which shade it takes
// (render.Taking), all unless it says. A kind declaring none casts a Patch taking all; what the
// board stands is Baked. The shade of the ground — the relief's shadows over a [Relief] the
// topography gives ([Plugin.OverRelief]), level ground without one, and the Baked blocks' on
// either — is baked by render on the GPU as the sun moves, a strip a frame, and handed to every
// frame (render.GroundShade) for whoever draws the ground and what stands on it to read;
// [CoarseShadows] bakes it half as fine. render lays the live shadows over whatever ground the
// frame drew, read off its depth, as a [ShadowPolicy] turns them by the distance from the eye
// ([Plugin.WithShadowPolicy]): the distant light's everywhere, and on level ground the lamps' too,
// from the lamps nearest the camera, as many as the Config says or none ([LampShadows]).
//
// # Light on things
//
// [Distant] is a distant light as the shaders and the CPU take it, render's (render.Distant:
// Light, Shaded, a [Beam] ready to light many, Frame the uniforms, ShadowOf and SilhouetteOf
// where a shadow lies, an [Umbra]); [Lamp] a lamp in the world. The lamps nearest a camera,
// [MaxLamps] at most, light its picture: the entities as render.Frame.Lit says, the ground
// through the shaders' lampLightAt. [Read.Reporter] is the light's line on the telemetry panel,
// as the frame drawn is lit.
//
// # Vocabulary
//
// Knobs, for an effect's Alter: [Source], what an entity shines with; [Shadow], how it casts and
// what it takes. Commands, a player's, of the settings screen alone: [CoarseShadows],
// [LampShadows]. Settings ([Settings], saved with the game, in [Resources]): the shadows coarse,
// the lamps casting.
package light
