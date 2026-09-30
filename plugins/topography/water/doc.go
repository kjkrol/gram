// Package water is the topography's water as its shaders draw it: the materials of the sea, which
// glints, rolls towards the shore and breaks into foam there ([Glint], the SeaGlint material), and
// of running water, carried down its slope ([Stream], RunningWater), both registered with the
// composer's library for any shader built on it to call (SeaGlintAt, RunningWater); the way to
// the shore a point of water has ([Shore], [Shores]); how fast water runs ([Flow]); and how the
// water is painted in layers for the ground drawn on the GPU ([Layers], [FlowSpan]).
package water
