// The clouds over the ground, baked every frame: every pixel of the baked image's part right of
// the shade, U.CloudPx pixels a cell, holds how thick the clouds stand over its point (cloudCover),
// for the mesh to read between them.

// Fragment is how thick the clouds stand over the world point custom.xy.
fn Fragment(dst: vec4<f32>, src: vec2<f32>, color: vec4<f32>, custom: vec4<f32>) -> vec4<f32> {
    let c = cloudCover(cloudField(custom.xy));
    return vec4<f32>(c, c, c, 1.0);
}
