// Package celestial is the celestial sphere over a world: where the sun, the moon and the stars
// stand as the world turns, and the real stars and the moon's face to draw them with.
//
// A [Place] is where on the world the sky is watched from: its latitude and which [Way] the sun
// stands at noon. [Place.SunPath] is the way to the sun at a time of the year and of the day —
// rising in the east, over NoonWay at noon, setting in the west, higher in summer, up all day or
// none past the polar circle. [SiderealTime] is the sky's turn at that moment: the sun's right
// ascension along the ecliptic and its hour angle, the sphere going round once a day and once more
// a year. [Place.Sphere] is how the sphere stands then, the ways its equatorial axes point, the
// north celestial pole as high over the north as the latitude is. [Place.MoonAt] is the way to the
// moon on its own path: along the ecliptic, leant [moonInclination] off it, as far ahead of the sun
// as it is round — rising and setting with the stars, drifting some 13° a day eastwards among
// them — and [Phase] how much of its face is lit. [Place.HeavensAt] gathers them into [Heavens]
// for whoever draws the sky; [Heavens.OnSky] places a star on it.
//
// [Stars] is the real night sky: the Yale Bright Star Catalogue, 5th revised edition (Hoffleit &
// Warren 1991, CDS catalogue V/50) as bright as magnitude 6.0, 5080 stars in stars.bin, which
// stars_gen.go writes from the catalogue; [StarSky] chooses them ([RealStars]) or made-up ones
// ([ScatteredStars]) for a world of its own. A [StarField] lays out the stars a camera sees and
// draws them on the GPU, each a soft dot in its colour, twinkling low (shaders/stars.wgsl).
// [MoonFace] is the moon's face as it turns to the Earth, from NASA's Scientific Visualization
// Studio's CGI Moon Kit (Lunar Reconnaissance Orbiter LROC data).
//
// The package is a leaf: plugins/atmosphere/sky lights the world by the sun and the moon it
// places, and the atmosphere's backdrop draws the sky of [Heavens].
package celestial
