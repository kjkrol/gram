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
// drifted with the wind, [Weather.Shade] how much shadow it casts under the cover, and
// [Weather.Overcast], [Weather.OvercastOn] and [Weather.OvercastQuad] lay the clouds' shadows over
// a sprite, over one drawn earlier, or on their own over a piece of the screen — a material of the
// composer's shader (weather.kage, [CloudShadow]) shaded between a piece's corners, laid only where
// a cloud reaches. [Weather.Haze] is how much of a point the air hides from a camera's eye, for
// render.Frame.Fog.
//
// The package is a leaf under plugins/atmosphere: the topography lights and dresses its relief by
// it and the sky's sun, the atmosphere's own sources draw the sky, the rain and a flat world's
// cloud shadows by it.
package air
