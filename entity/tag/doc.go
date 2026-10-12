// Package tag is the tags an entity carries: a family's [Tags], a component of up to
// [MaxTagsPerFamily] bits, each a [Tag] a plugin defines by name through the world's kinds
// (world.Kinds.DefineTag), saved by name. Tags are the plugins' technique, not a game's
// vocabulary: a game says roles (world.Roles), names and groups (entity.Named, entity.Group) and
// the plugins' commands (players.Give, selection.Allow). It is a leaf,
// read by the plugins and their hosts alike. [Any] stands for whatever an entity carries, for a
// host matching pairs.
//
// # Tags and markers
//
// A family's bits serve two ways.
//
// Tags are groups: what an entity is — a role, its owners, selectable. A kind gives them
// (comp.Tagged), they seldom change, and a query over the family reaches only the entities that
// carry it.
//
// Markers are states an entity switches on and off, often or for a single step: an effect is on it
// (Effect.Mark), an effect changed its components (effect.Changed), it entered another cell
// (unit.Entered). A plugin names its family of markers States and has every entity carry it
// for good — a kind lists it with comp.Marks, the world's roster gives it to every unit
// (Template.Default), an entity without it gets it the first time it is needed — and switches a marker
// by writing its bit. Putting a component on an entity or taking it off moves the entity to
// another place in memory, its whole row copied: some 200 ns each time, against a nanosecond or two
// for a bit (bench, Benchmark_Marker_*). A state that comes with data does the same: its component
// stays on the entity, empty when off (effect.Active).
//
// So a state that changes often, or lasts a step, is a marker; a state that lasts, on few
// entities, which a pass wants to walk alone — an order, a mind, the facts of a tree, an entity
// Outside the world's open edge — is a component of its own, put on and taken off: finding the
// marked costs a look at every entity of the family, where a component of its own is found by
// its archetype.
package tag
