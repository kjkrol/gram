// Package precipitation draws what falls from the world's weather: rain as streaks slanting with
// the wind, snow as flakes drifting down, as many as the weather says, in the light of the
// world's sun. [New] takes the world; the [Renderer] is a render.Source for a scene's Composer, on
// render.Air — over the world and under the selection — and keeps nothing between frames: each
// drop is where its number and the frame's time put it.
package precipitation
