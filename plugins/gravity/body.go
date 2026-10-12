package gravity

// Pull is the gravity of a world as a knob on the plugin's own entity, in world units a second
// squared: −Z in a world with heights, any way across a flat one (a platformer's down the
// screen, Y). The plugin only reads it, so an effect's Alter turns it — gravity reversed, a moon's
// — and its end gives the pull back.
type Pull struct{ X, Y, Z float64 }

// Body makes an entity's kind fall under the Pull, stand on what bears it and leap: Leap is how
// high it leaps, in world units — 0, not at all. Whatever carries a Body has its drift
// (world.Drift) and, in a world with heights, its altitude written by the gravity alone.
type Body struct{ Leap float64 }
