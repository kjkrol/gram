// Package aloft draws the clouds seen from above: a layer the sky shows from below, the heaps as
// bodies the line of sight passes through, lit from above, the shreds as thin sheets at their
// heights, half see-through over their shadows, thinning away into the ground.
//
// # Renderer
//
// A [Renderer] ([New]) is a layer of clouds (air.Layer), a render.Direct among what is seen
// through at the layer's height, drawn on the GPU while shown ([Renderer.Show]), as bodies or
// flat ([Renderer.Bodies]). An eye under the layer's top sees it on the sky instead, behind the
// hills. The atmosphere plugin draws one a layer, shown or not as its scene says.
package aloft
