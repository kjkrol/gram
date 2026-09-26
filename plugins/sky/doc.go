// Package sky is a day going by over a world with heights: the time of day and the sun it brings.
//
// [NewPlugin] takes the world and the day's [Config] — how long a whole day takes, when a fresh
// Stage begins, how high the sun stands at noon, in how many steps it moves. The time of day is a
// [Day] on the sky's own entity, made at Setup or found after a load, so it is saved with the game.
// Every tick the day moves on at its Pace, and at every step the world's sun is set to [SunAt] the
// hour: rising in the east, over the south at noon, setting in the west, below the horizon at
// night, its strength and the ambient light rising and falling with it. The board lights and
// shades the ground by that sun and the world lays the units' shadows, so mornings and evenings
// cast long shadows and nights are dark. Stepping keeps the terrain's shadows, worked out anew
// whenever the sun moves, from being worked out every tick.
//
// The plugin is a plugin.CommandHandler: [Pause] stops the day where it is or lets it go on at its
// pace (P); [Forward] (]) doubles the pace while the day goes by and moves it half an hour on while
// it stands; [Back] ([) halves the pace, or moves it half an hour back. [Plugin.Reporter] adds the time of day to a scene's
// render.TelemetryRenderer. Call [Plugin.RunPlan] every tick, before the world is drawn; a flat
// world has no light to change.
package sky
