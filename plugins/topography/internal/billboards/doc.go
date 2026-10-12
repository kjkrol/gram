// Package billboards is how the world's entities look on a map in relief, drawn on the GPU: in
// the views in relief they stand as billboards upright on their centres at their altitudes,
// hidden by the ground where it stands before them; from above they lie over their boxes as the
// world's own flat look draws them. Either way they are lit by the [Sky]'s sun, lean with its
// wind what sways, haze far off and cast their shadows on the relief. What the world's atlas
// says lies (a bridge's deck, a ford) lies flat at its altitude; a sprite standing Upright (a
// pier) stands on the four sides of its box.
//
// A [Look] ([New]) is the world's Look (world.DirectLook, world.LyingLook): picking and the
// selection's outline follow the billboards.
package billboards
