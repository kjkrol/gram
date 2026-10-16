// Package precipitation draws what falls from the world's weather: rain as streaks slanting with
// the wind, snow as flakes drifting down, only under the clouds it falls from and as thick as
// they let fall (air.Weather.FallsAt), in the light of the sun.
//
// # Renderer
//
// A [Renderer] ([New]) is a render.Direct before the whole world and under the marks, the
// atmosphere's, drawn on the GPU.
package precipitation
