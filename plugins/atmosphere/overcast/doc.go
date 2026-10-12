// Package overcast is the clouds' shadows over a flat world, drawn on the GPU: every pixel of the
// viewport darkened as much as the clouds over the ground there take of the sun.
//
// # Renderer
//
// A [Renderer] ([New]) is a render.Direct over what stands on the ground and under the overlays;
// a world with heights has its terrain lay the same shadows itself.
package overcast
