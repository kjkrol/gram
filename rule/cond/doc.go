// Package cond is the When of a branch: who is in what state, every clause saying whom it is
// about.
//
//	When: cond.Other.If(traveller).
//		And(cond.Self.If(cond.Not(talking))).
//		And(cond.Self.If(cond.Not(talked))),
//
// # Clauses
//
// [Self], [Other], [Around] and [On] each take one state: an effect the entity is under, a
// role it plays, a [Word] of the moment, [Chance], [Always], or [Not] of any of them. Clauses
// join with And and Or; [Not] of a whole Cond turns it round. At the top of a rule a role of
// Self or of Other narrows whom the host walks; anywhere else it is asked of the entity as the
// branch runs.
//
// # Words
//
// A moment's or a fact's words are a plugin's, made with [NewWord]: unit.Fallen,
// vision.SeesNobody, bullet.Within. A word says whether it is of the entity or of the other,
// and a clause of the wrong side is refused as the branch is built.
package cond
