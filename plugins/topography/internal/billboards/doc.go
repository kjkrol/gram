// Package billboards is how the world's entities look on a map in relief, drawn on the GPU: in the
// views in relief they stand as billboards upright on their centres at their altitudes, as wide as
// their boxes and as tall as their world.Z says (as the box is long without a height), hidden by
// the ground's depth where it stands before them; from above they lie over their boxes as the
// world's own flat look draws them. Either way they are lit by the [Sky]'s sun on level ground,
// lean with its wind what sways, haze far off, and cast their shadows on the relief away from the
// sun (sky.Sun.ShadowOf) — laid by the terrain's mesh over a square grid, over the frame's depth
// over hex prisms. An eye riding in a unit does not see the unit it rides in.
//
// A [Look] ([New]) is the world's Look (world.DirectLook): picking and the selection's outline
// follow the billboards, since they ask it where an entity is drawn (Drawn, Footprint).
package billboards
