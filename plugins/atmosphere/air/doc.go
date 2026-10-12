// Package air is the weather over a world as it is felt and drawn: the [Weather] the climate
// makes, and what it does to what is drawn.
//
// # Weather
//
// The zero [Weather] is a calm, clear day. [Visibility] is how far one sees through it on a world
// of a scale, [Weather.Frame] hands a render.Frame its shaders' uniforms and the fog colour
// ([Overcast], the sky greyed by the clouds), [Weather.Haze] is how much of a point the air hides
// from a camera's eye and [Weather.Sway] how far what sways leans in the wind.
//
// # Clouds
//
// [Weather.Cloud] is the clouds' noise over a point, drifted with the wind, and [Weather.Shade]
// how much shadow it casts under the cover: on the CPU the very numbers the shaders work out on
// the GPU, [CloudShadow] their material. The noise repeats every [CloudTile] and [HeapTile], so
// [BakeTile] bakes a tile of it once for the sky and the ground to look up per pixel.
package air
