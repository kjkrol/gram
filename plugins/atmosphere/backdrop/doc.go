// Package backdrop is the sky behind the world, drawn on the GPU before anything else: through a
// perspective the sky of the day from the horizon up, the sun and the moon in discs as wide on the
// sky however the camera zooms, the real stars at night (celestial.StarField) drawn under it, and
// the clouds on their layer, looked up in the tile of their noise (air.BakeTile), hiding what lies
// behind them; through any other camera the viewport filled in the sky's colour wherever the
// ground does not cover it.
//
// A [Renderer] ([New]) is a render.Direct at the Backdrop tier. It reads the sun and the weather
// as they stand, where the sun, the moon and the stars stand ([Renderer.WithHeavens]) and whether
// the stars and the moon show ([Renderer.WithShown]); the atmosphere (plugins/atmosphere) hands
// them its own.
package backdrop
