// Package backdrop is the sky behind the world, drawn on the GPU before anything else: through a
// perspective the sky of the day with the sun, the moon, the stars and the clouds on it; through
// any other camera the viewport filled in the sky's colour wherever the ground does not cover it.
//
// # Renderer
//
// A [Renderer] ([New]) is a render.Direct behind the whole world for a scene's Composer. It reads
// the sun and the weather as they stand, where the sun, the moon and the stars stand
// ([Renderer.WithHeavens]) and whether the stars and the moon show ([Renderer.WithShown]).
package backdrop
