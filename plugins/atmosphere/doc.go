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
// [Plugin.Over], as the plugin is made, has the low clouds hang over a board's ground, their bands
// fitted to its relief: under the peaks, covering them and over them, whatever height they
// reach. [Plugin.WithWeathering] does that too and lays the weather on the board
// (plugins/atmosphere/weathering): snow, ice and what sways, as effects on the cells; the ground
// standing up into the low clouds holds them back, piling up before it and clearing past it, and
// the rules of its cells let rain fall out of the air over them and gather its damp into clouds.
// The
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
// as a scene shows them ([Plugin.ShowClouds]): heaped on the sky from under them, the low ones
// half see-through over the world from above and before the slopes from under them
// (plugins/atmosphere/aloft), hiding and hidden by what stands nearer — the high ones never over
// the map from above, where they veiled it; and the rain and the snow before it
// (plugins/atmosphere/precipitation); whoever draws the ground shadows it under the clouds, each
// layer's along the way to the sun.
// [Plugin.Reporter] adds the daylight, how the clouds look and share their bands and whose
// shadows they cast to the telemetry panel; a game showing the time, the season, the moon and
// the weather nowhere else adds the calendar's and the climate's reporters too
// ([Plugin.Calendar], [Plugin.Climate]).
//
// # HUD
//
// The atmosphere gives a scene elements to place, each on a see-through panel as the
// telemetry's: [Plugin.TimeOfDay] the hour of the game's day, [Plugin.Date] the season and the
// date with the moon's phase beside them while the moon runs, [Plugin.Weather] the weather, the
// temperature and the wind, left out while none of the climate's workings run, and [Plugin.Bar]
// the three side by side on one panel, for an edge of the screen (ui.TopMiddle,
// ui.BottomMiddle). The weather is
// read through a drawer of its own: it goes in the tree a scene is made with, never one
// ui.Scene.Replace puts in.
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
// atmosphere's own entity, once a step), weathering.Overhead (a board's cell, once a second, with
// WithWeathering). Knobs, for an effect's Alter: sky.Moon and climate.Spread (on the atmosphere's
// own entity), weathering.Wring (on every cell of the board). Commands, a player's: [Freeze],
// [Hour], [ChangeWeather], [SetWeather], [Downpour], [Clouds], [HighShade]. Settings
// ([Settings], saved with the game, in [Resources]): the frozen light and its hour, the clouds'
// look, the high clouds' shadows.
package atmosphere
