// Package precipitation draws what falls from the world's weather: rain and snow, drop by drop
// between the low clouds and the ground, only under the clouds it falls from and as thick as they
// let fall (air.Weather.FallsAt), in the light of the sun.
//
// # Drops
//
// [Drops] ([NewDrops]) is a render.Direct among what is seen through, the atmosphere's, drawn on
// the GPU: every drop a point of the world falling and leaning with the wind, seen through the
// camera as it would be — a dot from above, a short line from the side, nothing too far off to
// make out — hidden behind what is drawn nearer.
package precipitation
