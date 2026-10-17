// Package air is the weather over a world as it is felt and drawn: the [Weather] the climate
// makes, and what it does to what is drawn.
//
// # Weather
//
// The zero [Weather] is a calm, clear day. [Weather.Visibility] is how far one sees through the
// air over a point, its [Sight] that of clear air on a world of a scale; [Weather.Frame] hands a
// render.Frame its shaders' uniforms and the sky's colours over the eye ([Under]), one source for
// the sky, the fog and the water: at the horizon the Fog ([Overcast], the sky greyed by the
// clouds), overhead deeper ([Overhead]), skyAt in WGSL between them. [Weather.Haze] is how much
// of a point the air hides from a camera's eye ([Hazer] for a frame's worth) and [Weather.Sway]
// how far what sways leans in the wind.
//
// # Dome
//
// The weather over a point is its air [Mass]: how much of the sky its clouds cover, how heaped they
// are, how much falls from them, how damp it is. A [Dome] holds the masses over a world and its sky
// out to the horizon, drifting with the clouds: level 0 over the world in cells of [BoardCell],
// each level further out in coarser cells, every level a ring of [Patch]es, each an entity of its
// own. [Dome.Carry] brings masses in where the wind blows from, at the horizon, [Dome.Settle] has
// them settle in place, [Dome.Lift] has the [Land] under them work on them: the low clouds held
// back and piling up before ground standing up into them, rain wrung out and damp gathered into low
// clouds where it says ([Mass.WithLow] changes the low ones alone). [Weather.At] is the mass over a
// point (airAt in WGSL, from the frame's image AirMasses a [DomeImage] keeps), [Weather.Falls] its
// rain and snow by the temperature and [Weather.FallsAt] what comes down there under the clouds,
// none from their gaps.
//
// # Clouds
//
// The clouds hang in two layers, each a [Layer]: the heaps [Low], their bodies reaching
// [Weather.Top], and the torn shreds [High], thin sheets each at its own height through a band
// from over the heaps up to the weather's Bases ([Weather.HighHeight]). The heaps hang over the
// ground's relief ([ReliefOf], [BasesOver]; [Bases] over no ground in particular) in three bands,
// under the peaks, covering them and over them, each in places of its own as the weather's
// Ceilings share them, drifting slower than the clouds ([Weather.LowBand], cloudLowBand in WGSL):
// the same show over a relief of any height. A mass holds the heaps the more the sky is covered
// and heaped, the shreds with a few clouds already ([Mass.Layer]), the shreds in patches
// ([Weather.HighAt]). [Weather.Cloud]
// is a layer's noise over a point, drifted with the wind, and [Weather.Shade] how much of the sun
// the clouds take at a point of the ground, each layer's shadow where the way to the sun crosses
// it: on the CPU the numbers the shaders work out on the GPU (cloudsShadeAt), [CloudShadow] their
// material. The noise repeats every [CloudTile] and [HeapTile], so [Tile] is a tile of it, baked
// once, which every shader looks up per pixel as the frame's image CloudTile ([Weather.Frame]
// hands it).
package air
