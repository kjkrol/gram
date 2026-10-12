// Package tag is the tags an entity carries: a family's [Tags], a component of up to
// [MaxTagsPerFamily] bits, each a [Tag] a plugin defines by name through the world's kinds
// (world.Kinds.DefineTag), saved by name. Tags are the plugins' technique, not a game's
// vocabulary: a game says roles, names, groups and the plugins' commands. [Any] stands for
// whatever an entity carries, for a host matching pairs.
//
// # Tags and markers
//
// A family's bits serve two ways. Tags are groups: what an entity is (a role, its owners); a
// kind gives them (comp.Tagged) and they seldom change. Markers are states an entity switches on
// and off, often or for a single step: a plugin has every entity carry its family of markers for
// good (comp.Marks) and switches one by writing its bit.
//
// Putting a component on an entity or taking it off moves the entity to another place in
// memory, some 200 ns each time, against a nanosecond or two for a bit. So a state that changes
// often, or lasts a step, is a marker; a state that lasts, on few entities, which a pass wants
// to walk alone, is a component of its own.
package tag
