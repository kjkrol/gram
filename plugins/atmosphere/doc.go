// Package atmosphere is the sky over a world on the world's clock: the calendar, the light of the
// day, the climate and its weather, what falls, and what the weather does to the ground.
//
// [NewPlugin] takes the world and the [Config]: the calendar's (plugins/atmosphere/calendar — how
// long a day is, the hour a fresh game begins at, a GameYear of eight days or an EarthYear), the
// sky's (plugins/atmosphere/sky — where the sun stands at noon, the light going on or in steps, a
// frozen light, the real stars or made-up ones), the climate's (plugins/atmosphere/climate — the
// zone, the weathers, the one to begin in, the seed) and which of its workings go on ([Running]:
// the day, the weather's changes, the wind, the clouds, what falls, the weathering, the stars, the
// moon; all of them without one), to switch as the game goes too ([Plugin.SetRunning]). The
// calendar is the clock at a fixed scale, so it is saved with the clock and hurries with its tempo;
// the sun goes at the climate's zone's latitude, the sphere of the stars and the moon with it
// (plugins/atmosphere/celestial).
//
// In the plan ([Plugin.RunPlan], after the world's) the light runs at once, once a tick — it is a
// look at the world, changing in the tactical pause too — and the weather in every step of the
// simulation, standing in the pause and hurrying with the tempo. The sun and the weather are the
// atmosphere's to give ([Plugin.Sun], [Plugin.Air]; the world knows nothing of them): a board in
// relief takes them through topography.Plugin.WithAtmosphere and shades its terrain by the sun,
// lays the clouds' shadows and hazes the far off, all on the GPU; a flat board and the world's
// sprites take the sun's light on level ground through [Plugin.WithBoard] — tinted by the hour,
// night dark, dawn warm, what sways leaning with the wind — and the clouds' shadows from
// [Plugin.Clouds], drawn on the GPU over the ground under every pixel, their noise worked out every
// few pixels (plugins/atmosphere/overcast). The sun's maths and shaders are
// plugins/atmosphere/sky's, the weather's plugins/atmosphere/air's; plugins/atmosphere/backdrop is
// the sky behind the world, a render.Direct drawn on the GPU, and plugins/atmosphere/precipitation
// what falls. The root keeps no shaders of its own: it composes these.
//
// [Plugin.WithWeathering] lays the weather on a board (plugins/atmosphere/weathering): snow
// lying, ice on the water, what sways swaying, as effects on the cells from a trigger of the
// world's clock, once a second of game time. [Plugin.Hook] hosts triggers told the weather
// (climate.Weathering) every step.
//
// The plugin is a plugin.CommandHandler, its keys the players carry: P freezes the light and lets
// it go, Shift+] and Shift+[ move a frozen light half an hour on and back, Shift+W changes the
// weather. [Plugin.Renderer] is the sky behind the world — through a perspective the sky of the
// day from the horizon up; the sun and the moon (its face celestial.MoonFace, lit as the sun
// stands to it, north towards the pole) in discs as wide on the sky however the camera zooms; the
// real stars at night (celestial.StarField), drawn under the sky; and the clouds on their layer, hiding what lies behind them, the same
// clouds whose shadows lie on the ground, looked up in the tile of their noise (air.BakeTile) —
// [Plugin.Precipitation] the rain and the snow, [Plugin.Clouds] the flat world's cloud shadows,
// all render.Sources for a scene's Composer;
// [Plugin.Reporter] adds the time of day, the date, the light and the weather to a scene's
// render.TelemetryRenderer, [Plugin.HUD] the calendar to its layers.
package atmosphere
