// Package sky is a day going by over a world with heights: the time of day and the sun it brings.
//
// [NewPlugin] takes the world and the day's [Config] — how long a whole day takes, when a fresh
// Stage begins, how high the sun stands at noon and which [Way] (the north-west by default), in how
// many steps it moves, the [Calendar] — a [GameYear] of eight days and a four-day moon, or an
// [EarthYear] of 365 days in twelve months and a moon of 29.5 — the season a fresh Stage begins in
// the middle of and how much higher the sun stands at midsummer. The Day keeps the date too, its
// Season ([Day.Season]: a quarter of the year), the moon ([Day.Moon]) and how long it takes; the
// sun stands [Config].Tilt higher at noon in summer and lower in winter, rising earlier and setting
// later in summer. How high it goes and how long the days are is the latitude's: 30° without a
// climate, a climate's zone's with one (Plugin.SetLatitude, which plugins/climate calls); past the
// polar circle the sun stays up all summer and down all winter. plugins/climate reads the day and
// the season from the ECS. The time of day is a
// [Day] on the sky's own entity, made at Setup or found after a load, so it is saved with the game.
// Every tick the day moves on at its Pace, and at every step the world's light is set to
// [Config.LightAt] the hour: the sun — with noon in the south rising in the east and setting in
// the west, the whole path turned round to NoonWay — and once it is well below the horizon the
// moon, going its way behind it, as bright as it is full, in a paler light;
// its strength rising and falling with it, and the colours of the sky and of the sun's light going
// through the day: blue by day, orange at sunrise and sunset, deep blue at night. The board lights and
// shades the ground by that sun and the world lays the units' shadows, so mornings and evenings
// cast long shadows and nights are dark. Stepping keeps the terrain's shadows, worked out anew
// whenever the sun moves, from being worked out every tick.
//
// The plugin is a plugin.CommandHandler: [Pause] stops the day where it is or lets it go on at its
// pace (P); [Forward] (]) doubles the pace while the day goes by and moves it half an hour on while
// it stands; [Back] ([) halves the pace, or moves it half an hour back. [Plugin.Reporter] adds the time of day to a scene's
// render.TelemetryRenderer. [Plugin.Renderer] is the backdrop: behind the world, in the sky's
// colour, drawn only when the ground does not cover the whole screen — beyond the world's edge,
// above a low view. Call [Plugin.RunPlan] every tick, before the world is drawn; a flat world has
// no light to change.
package sky
