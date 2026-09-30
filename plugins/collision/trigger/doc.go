// Package trigger holds ready-made reactions to contacts, each a node for the body of a
// act.Trigger of a collision.Meeting or Struck: CountContacts for telemetry, ShowHits with
// HitOverlay to keep a hit visible, LogContacts to write a line per contact.
//
// # CountContacts
//
// [CountContacts] adds every Meeting it is handed to a [ContactStats] the game owns. The total
// only grows; whoever shows a rate works it out from it, as render.TelemetryRenderer does.
//
// # ShowHits and HitOverlay
//
// [Hit] defines the hit ([Hits]): the effect of having struck something, lasting a while of game
// time, and the marker ([States]) it has on while it lasts. [ShowHits] is a node for a trigger of a
// Struck casting it on an entity that struck something, and [HitOverlay] a Drawing trigger for the
// world plugin: an overlay sprite on top of the entity while its marker is on.
//
// # LogContacts
//
// [LogContacts] writes a line per contact to stdout in the default format; [LogTo] and [LogAs]
// change where and how.
package trigger
