// Package backdrop is the sky behind the world, drawn on the GPU before anything else: through a
// perspective the sky of the day with the sun, the moon, the stars and the clouds on it; through
// any other camera the viewport filled in the sky's colour wherever the ground does not cover it.
//
// # Renderer
//
// A [Renderer] ([New]) is a render.Direct behind the whole world, the atmosphere's; composing, it
// hands the frame the sun and the weather (light.Distant.Frame, air.Weather.Frame). It reads them as
// they stand, where the sun, the moon and the stars stand
// ([Renderer.WithHeavens]), whether the stars and the moon show ([Renderer.WithShown]) and how
// the clouds show ([Renderer.WithClouds], [Clouds]): each layer flat or as heaps, bodies seen
// from under them, worked out coarser than the screen into image2; the layers under the eye, and
// the heaps before the ground when another draws them, it leaves to plugins/atmosphere/aloft.
//
// # Dome
//
// The sky is drawn in four parts, a [Dome]: the vault (a [Part]), the [Sun], the [Moon] and the
// [Stars], a catalogue drawn behind the sky where each stands or dots its shader adds, or both.
// [Earth] is the Earth's sky, its parts one by one [EarthVault], [EarthSun], [EarthMoon] and
// [EarthStars], [ScatteredStars] made-up dots; a part without WGSL draws nothing.
// [Renderer.Dress] has the sky drawn with a dome; a game's sky is an atmosphere.Sky.
//
// A part is WGSL written to the contract of shaders/sky.wgsl: its entry (SkyVault, SkyStars,
// SkyMoon, SkySun) is handed the sky at a pixel and a disc, and reads uniforms of its own
// ([Part].Reads), which its Feed sets every frame the sky is drawn ([Values]); the moon's face is
// image1, the heaps image2. A part's names stand beside the library's and the materials': prefix
// them (duskBand). A uniform or a name two parts declare, or the library's, panics as the sky is
// dressed.
package backdrop
