// Package calendar turns the tactical clock's game time into days, seasons and the moon.
//
// # Calendar
//
// A [Calendar] ([New]) is the clock at a fixed scale: a day every [Config.DayRealTime] of game
// time from the moment a fresh game begins at. It keeps no state of its own, so a loaded game's
// clock brings its date back. [Calendar.Now] is the [Moment] the clock stands at, with its Date,
// Time, [Moment.Season] and [Moment.Moon]; [Calendar.At] the moment of any game time. A [Year] is
// a [GameYear] of eight days or an [EarthYear].
//
// For a rule of the clock's Moment, [Calendar.Daily], [Calendar.Yearly] and [Calendar.Seasonal]
// give clock.Every its period and offset. [Calendar.Reporter] is two telemetry lines,
// [Calendar.HUD] a screen renderer.
package calendar
