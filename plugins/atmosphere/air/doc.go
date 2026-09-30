// Package air is the weather over a world as it is felt and drawn: the [Weather] the climate
// makes — the wind, the clouds and how far the wind has carried them, rain, snow, the temperature,
// how far one sees — and what it does to what is drawn.
//
// # Weather
//
// The zero [Weather] is a calm, clear day. [Visibility] is how far one sees through it on a world
// of a scale: [ClearAir] in clear air, less under clouds, far less in rain and snow. A weather
// hands a render.Frame its shader's uniforms — the wind, the clouds' drift and cover — and the
// colour what lies far off turns to, the sky greyed by the clouds ([Weather.Frame], [Overcast]).
//
// # What the weather does
//
// [Weather.Sway] is how far what sways leans in the wind at a time: with it, the harder the
// further, rocking as gusts roll downwind. [Weather.Cloud] is the clouds' noise over a point,
// drifted with the wind, and [Weather.Shade] how much shadow it casts under the cover — on the
// CPU the very numbers the shaders' cloudField, cloudCover and cloudShade work out on the GPU for
// the ground's shadows and the clouds on the sky; [CloudShadow] is their material for a piece of a
// frame. [Weather.Haze] is how much of a point the air hides from a camera's eye.
//
// The package is a leaf under plugins/atmosphere: the topography lights and dresses its relief by
// it and the sky's sun, the atmosphere's own sources draw the sky, the rain and a flat world's
// cloud shadows by it.
package air
