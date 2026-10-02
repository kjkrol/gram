// Package hooks holds ready-made rules of contacts, for the collision plugin's Hook — and one of
// drawing, for the world's Draw: ShowHits with HitOverlay to keep a hit visible. A game wanting
// something else writes its own rule of a collision.Meeting or Struck (rule.On). How many contacts
// a second and a line per contact are collision's own (collision.Plugin.WithStats, WithLog).
//
// # ShowHits and HitOverlay
//
// [Hit] defines the hit: the effect of having struck something, lasting a while of game time, with
// its marker on while it lasts. [ShowHits] casts it at every Struck — hook it on collision — and
// [HitOverlay] draws an overlay sprite on top of the entity while the marker is on — give it to
// the world's Draw.
package hooks
