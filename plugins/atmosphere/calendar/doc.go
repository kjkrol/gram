// Package calendar turns the tactical clock's game time into days, seasons and the moon.
//
// A [Calendar] is the clock at a fixed scale: a day every [Config.Day] of game time, counted from
// the moment a fresh game begins at — [Config.Start], the hour on the clock's face, in the middle of
// [Config.Season]. It keeps no state of its own, so a loaded game's clock brings its date back,
// and it never jumps: the day is hurried by the clock's tempo, stopped by its pause. [Calendar.Now]
// is the [Moment] the clock stands at — its Date, Time, Season and Moon — [Calendar.At] the moment
// of any game time.
//
// A [Year] is [GameYear], eight days to watch go by, or [EarthYear], the 365 days and twelve months
// as they are. For a rule of the clock's Moment, [Calendar.Daily] and [Calendar.Yearly] give
// clock.Every the period and offset of every day at an hour or every year at a time of it, and
// [Calendar.Seasonal] of the start of a season. [Calendar.Reporter] is two telemetry lines,
// [Calendar.HUD] a screen layer.
package calendar
