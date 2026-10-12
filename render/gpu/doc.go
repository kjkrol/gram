// Package gpu draws for render on WebGPU (gogpu/wgpu): textures images live in, programs drawing
// triangles into them with a fragment written in WGSL, and the frame's commands, gathered and
// submitted at once. The engine hands it the window's device (Use); a test or a tool draws
// without a window (Headless).
//
// # Images
//
// A [Texture] holds 8-bit RGBA with colours premultiplied by alpha and no sRGB conversion. A
// draw targets a rectangle of a texture, clipping to it; its vertices are in the texture's pixels.
//
// # Programs
//
// A [Program] is a WGSL fragment on a vertex stage of the package's own: the source defines
//
//	fn Fragment(dst: vec4<f32>, src: vec2<f32>, color: vec4<f32>, custom: vec4<f32>) -> vec4<f32>
//
// and reads up to four images with imageSrcNAt(p), p in the texture's pixels. Its uniforms are a
// struct Uniforms, read as U, laid out by a [Layout] and handed to each draw packed, at most 4096
// bytes in all. A mesh program ([NewMesh]) brings its own vertex stage and draws against a
// [Depth] buffer ([Mesh]).
//
// # Frames
//
// Draws are gathered into one command buffer and written to the GPU as it is submitted: at
// [Submit], before pixels are written into a texture or read out of one, and when the gathered
// data outgrows its buffers. A write of pixels lands after every draw made before it.
package gpu
