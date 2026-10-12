// Package site is where a bridge may stand: [Plan] a bridge from one head to another over a
// [Ground] under the [Rules], or say why not ([Refusal]). It runs no game: the bridges plugin
// plans a player's bridge over the board and its heights with it, and cmd/gramap a map's over the
// ground it is making, so both keep one set of rules.
//
// A bridge runs straight along one of the grid's directions from a head to a head — tiles its
// road comes onto from any side, flat at its deck — over Rules.Longest cells at most, its deck
// Rules.Clearance over the ground under every one and no more than Rules.High over it. It crosses
// something: water, or a valley somewhere Rules.Under its deck — the height of what passes under
// it, the game's figures; in a world without heights, water or cells its road may not take. No
// span runs beside another bridge's span or over another bridge, and a road under it runs
// straight across it, Under its deck. A head's corners count at the deck under the spans too, as
// they will lie: in relief a bridge spans two cells at least, one alone lying over nothing. A
// head's ground is moved Rules.Reshape at most to lie flat at the deck, never so a road beside it
// would lie where Transport Tycoon would not lay it (ground.RoadFits).
// A bridge of two spans or more stands on a pier between every two.
package site
