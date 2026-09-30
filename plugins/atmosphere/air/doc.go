// Package air is the weather over a world as it is felt and drawn: the [Weather] the climate
// makes — the wind, the clouds and how far the wind has carried them, rain, snow, the temperature,
// how far one sees — and what it does to what is drawn.
//
// # Weather
//
// The zero [Weather] is a calm, clear day. [Visibility] is how far one sees through it on a world
// of a scale: [ClearAir] in clear air, less under clouds, far less in rain and snow. A weather
// hands a render.Frame its shader's uniforms — the wind, the clouds' drift, cover and heaps — and the
// colour what lies far off turns to, the sky greyed by the clouds ([Weather.Frame], [Overcast]).
//
// # What the weather does
//
// [Weather.Sway] is how far what sways leans in the wind at a time: with it, the harder the
// further, rocking as gusts roll downwind. [Weather.Cloud] is the clouds' noise over a point,
// drifted with the wind — torn shreds some 400 world units wide gathered, as much as the
// weather's Billow says, into big heaps some 3000 apart, each forming whole as the cover reaches
// its own and growing as it grows on, round lobes on its edge — and [Weather.Shade] how much
// shadow it casts under the cover: on the CPU the very numbers the shaders' cloudField,
// cloudCover and cloudShade work out on the GPU; [CloudShadow] is their material for a piece of a
// frame. The noise is the same again every [CloudTile] (the shreds) and [HeapTile] (the heaps'
// cores), so [BakeTile] bakes a tile of it once — the shreds in red, the heaps' cores in green,
// with levels each half as fine, each pixel averaging the noise it stands for — and the sky and
// the ground look it up per pixel (cloudTileSpot, cloudTileLevel, cloudFromTile), fixed to the
// world as the eye turns, evened out as far off as a pixel sees it. [Weather.Haze] is how much of
// a point the air hides from a camera's eye.
//
// The package is a leaf under plugins/atmosphere: the topography lights and dresses its relief by
// it and the sky's sun, the atmosphere's own sources draw the sky, the rain and a flat world's
// cloud shadows by it.
package air
