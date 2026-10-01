// Package hooks holds ready-made rules of contacts, for the collision plugin's Hook — and one for
// the world's: CountContacts for telemetry, ShowHits with HitOverlay to keep a hit visible,
// LogContacts to write a line per contact. A game wanting something else writes its own rule of a
// collision.Meeting or Struck (rule.On).
//
// # CountContacts
//
// [CountContacts] adds every Meeting to a [ContactStats] the game owns. The total only grows;
// whoever shows a rate works it out from it, as render.TelemetryRenderer does.
//
// # ShowHits and HitOverlay
//
// [Hit] defines the hit: the effect of having struck something, lasting a while of game time, with
// its marker on while it lasts. [ShowHits] casts it at every Struck — hook it on collision — and
// [HitOverlay] draws an overlay sprite on top of the entity while the marker is on — hook it on
// the world.
//
// # LogContacts
//
// [LogContacts] writes a line per contact to stdout in the default format; [LogTo] and [LogAs]
// change where and how.
package hooks
