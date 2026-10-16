// Package atmosphere is the sky over a world on the world's clock: the calendar, the light of the
// day, the climate and its weather, what falls, and what the weather does to the ground.
//
// # Plugin
//
// [NewPlugin] takes a world with heights and a [Config]: the calendar's
// (plugins/atmosphere/calendar), the sky's (plugins/atmosphere/sky), the climate's
// (plugins/atmosphere/climate) and which of its workings go on ([Running]; [Plugin.SetRunning]
// switches them as the game goes). The light runs once a tick, in the tactical pause too; the
// weather in every step of the simulation, carried over the world by the wind: a new one is born
// at the horizon it blows from and comes in over the world, and rain and snow fall only under
// their clouds.
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
// one, either over the world within seconds; [Clouds] has the clouds drawn in the next
// [CloudLook] (Shift+V).
//
// # Drawing
//
// The plugin draws the sky behind the world (plugins/atmosphere/backdrop), which hands every frame
// the sun and the weather; the clouds in two layers, low heaps and high shreds, flat or as bodies
// as a scene shows them ([Plugin.ShowClouds]): heaped on the sky from under them, half see-through
// over the world from above (plugins/atmosphere/aloft); and the rain and the snow before it
// (plugins/atmosphere/precipitation); whoever draws the ground shadows it under the clouds, each
// layer's along the way to the sun.
// [Plugin.Reporter] adds the time, the moon, the light, the weather and how the clouds look to
// the telemetry panel, [Plugin.HUD] the calendar to a scene (ui.Drawer).
//
// # Sky
//
// The sky is the Earth's unless a game gives its own: a [Sky], in a file of its own (sky.go),
// noted in the Stage's Figures link ([Plugin.Skies]). Its Look sets the parts of the dome
// (plugins/atmosphere/backdrop) it draws its own way: the vault, the sun, the moon, the stars.
//
// # Vocabulary
//
// Moments, for a role's rules, hosted by its parts: sky.Moonrise and climate.Weathering (the
// atmosphere's own entity, once a step). Knobs, for an effect's Alter: sky.Moon (on the
// atmosphere's own entity). Commands, a player's: [Freeze], [Later], [Earlier], [ChangeWeather],
// [SetWeather], [Clouds].
package atmosphere
