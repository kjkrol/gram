package celestial

// Up reports whether a body whose height's sine is h is up enough to light the world and show its
// disc: a little below the horizon still counts.
func Up(h float32) bool { return h > -0.1 }

// Risen is how far a body whose height's sine is h has risen into its full light: 0 on the
// horizon, 1 a fifth of the way up.
func Risen(h float32) float32 { return smoothstep(0, 0.2, h) }

// Gone is how far the sun, its height's sine h, has gone below the horizon for the moon to light
// the world in its place: 0 while it is up, 1 once the dusk is over.
func Gone(h float32) float32 { return smoothstep(-0.1, -0.25, h) }

// Night is how dark the sky is for the stars with the sun's height's sine h: 0 by day, 1 once the
// twilight is over.
func Night(h float32) float32 { return 1 - smoothstep(-0.2, -0.05, h) }
