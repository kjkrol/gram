// Package atmosphere is the sky over a world on the world's clock: the calendar, the light of the
// day, the climate and its weather, what falls, and what the weather does to the ground.
//
// # Plugin
//
// [NewPlugin] takes the world and a [Config]: the calendar's (plugins/atmosphere/calendar), the
// sky's (plugins/atmosphere/sky), the climate's (plugins/atmosphere/climate) and which of its
// workings go on ([Running]; [Plugin.SetRunning] switches them as the game goes). The light runs
// once a tick, in the tactical pause too; the weather in every step of the simulation.
// [Plugin.Sun] and [Plugin.Air] are what a board takes to light and shade itself
// (board.Plugin.WithAtmosphere, topography.Plugin.WithAtmosphere).
//
// [Plugin.WithWeathering], as the plugin is made, lays the weather on a board
// (plugins/atmosphere/weathering): snow, ice and what sways, as effects on the cells. The
// atmosphere is an entity of its own: it plays roles, whose rules of climate.Weathering are told
// the weather once a step, and a command may be for it.
//
// # Commands
//
// [Freeze] stops the light of the day (P), [Later] and [Earlier] move a frozen light half an hour
// (Shift+], Shift+[), [ChangeWeather] throws the next weather (Shift+W) and [SetWeather] names
// one.
//
// # Drawing
//
// [Plugin.Renderer] is the sky behind the world (plugins/atmosphere/backdrop),
// [Plugin.Precipitation] the rain and the snow, [Plugin.Clouds] a flat world's cloud shadows
// (plugins/atmosphere/overcast), all render.Sources for a scene's Composer; [Plugin.Reporter]
// adds the time, the light and the weather to the telemetry panel, [Plugin.HUD] the
// calendar to a scene's layers.
//
// # Vocabulary
//
// Moments, for a role's rules, hosted by its parts: sky.Moonrise and climate.Weathering (the
// atmosphere's own entity, once a step). Knobs, for an effect's Alter: sky.Moon (on the
// atmosphere's own entity). Commands, a player's: [Freeze], [Later], [Earlier], [ChangeWeather],
// [SetWeather].
package atmosphere
