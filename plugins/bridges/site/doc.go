// Package site is where a bridge may stand: [Plan] a bridge from one head to another over a
// [Ground] under the [Rules], or say why not ([Refusal]). It runs no game, so the bridges plugin
// and cmd/gramap keep one set of rules.
//
// A bridge runs straight along one of the grid's directions from a head to a head, tiles flat
// at its deck, over Rules.Longest cells at most, its deck Rules.Clearance over the ground under
// every one and no more than Rules.High over it. It crosses something: water, or a valley
// Rules.Under its deck; over a map without relief, water or cells its road may not take. No span
// runs beside or over another bridge, a road under it runs square across, and a head's ground is
// moved Rules.Reshape at most, never so a road beside it breaks. A bridge of two spans or more
// stands on a pier between every two.
package site
