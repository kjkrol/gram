// The sun on the ground, baked: every pixel of an image U.ShadePx pixels a cell holds how much of
// the sun reaches the ground at its point (sunlit), for the mesh to read between them.

// Fragment is how much of the sun reaches the ground at the world point custom.xy.
fn Fragment(dst: vec4<f32>, src: vec2<f32>, color: vec4<f32>, custom: vec4<f32>) -> vec4<f32> {
    let p = custom.xy;
    let lit = sunlit(vec3<f32>(p, height(p)), sunWay());
    return vec4<f32>(lit, lit, lit, 1.0);
}
