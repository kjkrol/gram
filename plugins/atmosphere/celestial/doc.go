// Package celestial is the celestial sphere over a world: where the sun, the moon and the stars
// stand as the world turns, and the real stars and the moon's face to draw them with.
//
// # Place
//
// A [Place] is where on the world the sky is watched from: its latitude and which [Way] the sun
// stands at noon. [Place.SunPath] is the way to the sun at a time of the year and of the day,
// [Place.MoonAt] the way to the moon and [Phase] how much of its face is lit; [SiderealTime] and
// [Place.Sphere] are how the sphere of the stars stands then. [Place.HeavensAt] gathers them into
// [Heavens] for whoever draws the sky, and [Heavens.OnSky] places a star on it.
//
// # Heights
//
// Of a body whose height's sine is h: [Up] whether it lights the world, [Risen] how far into its
// full light, [Night] how dark the sky is under a sun there and [Gone] how far it has gone for the
// moon to light the night: one place for the thresholds the light and the sky share.
//
// # Stars
//
// [Stars] is the real night sky, the Yale Bright Star Catalogue to magnitude 6; [StarSky] chooses
// it ([RealStars]) or made-up stars ([ScatteredStars]). A [StarField] lays out the stars a camera
// sees and draws them on the GPU. [MoonFace] is the moon's face, from NASA's CGI Moon Kit.
package celestial
