// Package aloft draws the clouds seen from above: the layer the sky shows from below, laid over
// the world as thin slices, the higher ones only where a cloud is thick, so it stands as a body
// lit from above, half see-through over its shadow.
//
// # Renderer
//
// A [Renderer] ([New]) is a render.Direct among what is seen through, at the clouds' height,
// drawn on the GPU while shown ([Renderer.Show]); an eye under the clouds sees them on the sky
// instead. The atmosphere plugin draws it, shown or not as its scene says.
package aloft
