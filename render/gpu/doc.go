// Package gpu draws for render on WebGPU (gogpu/wgpu): textures images live in, programs drawing
// triangles into them with a fragment written in WGSL, and the frame's commands, gathered and
// submitted at once. It is render's own: nothing else in gram imports it but the engine, which
// hands it the window's device (Use); a test or a tool draws without a window (Headless).
//
// # Images
//
// A [Texture] holds 8-bit RGBA with colours premultiplied by alpha and no sRGB conversion, as
// Ebitengine kept them, so what gram drew looks the same. A texture made before there is a device
// keeps what is written into it and goes to the GPU when first drawn with or into. A draw targets
// a rectangle of a texture, clipping to it; its vertices are in the texture's pixels.
//
// # Programs
//
// A [Program] is a WGSL fragment on a vertex stage of the package's own: the source defines
//
//	fn Fragment(dst: vec4<f32>, src: vec2<f32>, color: vec4<f32>, custom: vec4<f32>) -> vec4<f32>
//
// — dst the pixel's centre in the target, src and color and custom interpolated from the
// vertices — and reads up to four images with imageSrcNAt(p), p in the texture's pixels, which
// is transparent outside the image's rectangle (imageSrcNOrigin, imageSrcNSize), as Kage's are.
// Its uniforms are a struct Uniforms, read as U; the program is told its fields' WGSL and hands
// them to each draw packed by WGSL's layout (Layout).
//
// # Frames
//
// Draws are encoded into one command buffer, their vertices, indices and uniforms gathered and
// written to the GPU as it is submitted: at Submit, before pixels are written into a texture or
// read out of one, and when the gathered data outgrows its buffers. A write of pixels therefore
// lands after every draw made before it and before every one after.
package gpu
