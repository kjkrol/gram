// Package atmosphere is the sky over a world on the world's clock: the calendar, the light of the
// day, the climate and its weather, what falls, and what the weather does to the ground.
//
// [NewPlugin] takes the world and the [Config]: the calendar's (plugins/atmosphere/calendar — how
// long a day is, when a fresh game begins, a GameYear of eight days or an EarthYear), the sky's
// (plugins/atmosphere/sky — where the sun stands at noon, the light going on or in steps, a frozen light) and
// the climate's (plugins/atmosphere/climate — the zone, the weathers, the seed). The calendar is
// the clock at a fixed scale, so it is saved with the clock and hurries with its tempo; the sun
// goes at the climate's zone's latitude.
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
// few pixels. The sun's maths and shaders are plugins/atmosphere/sky's, the
// weather's plugins/atmosphere/air's; [Backdrop] is the sky behind the world, a render.Direct
// drawn on the GPU (shaders/backdrop.wgsl), and plugins/atmosphere/precipitation what falls.
//
// [Plugin.WithWeathering] lays the weather on a board (plugins/atmosphere/weathering): snow
// lying, ice on the water, what sways swaying, as effects on the cells from the world's schedule,
// once a second of game time. [Plugin.RegisterBehavior] hosts a climate.Every told the weather
// every step.
//
// The plugin is a plugin.CommandHandler, its keys the players carry: P freezes the light and lets
// it go, Shift+] and Shift+[ move a frozen light half an hour on and back, Shift+W changes the
// weather. [Plugin.Renderer] is the sky behind the world — through a perspective the sky of the
// day from the horizon up, the sun in it and the clouds on it, the same clouds whose shadows lie
// on the ground — [Plugin.Precipitation] the rain and the snow, [Plugin.Clouds] the flat world's
// cloud shadows, all render.Sources for a scene's Composer;
// [Plugin.Reporter] adds the time of day, the date, the light and the weather to a scene's
// render.TelemetryRenderer, [Plugin.HUD] the calendar to its layers.
package atmosphere
