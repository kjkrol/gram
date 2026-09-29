// The outline laid over a tile after all that lies on it, a material of the composer's shader.

// Outline darkens what lies under it along the edges its custom says, as a sprite's own outline
// does (Fragment): minus 1 minus the distance to each edge in pixels; as faint as red.
fn Outline(p: vec2<f32>, red: f32, frac: f32, custom: vec4<f32>) -> vec4<f32> {
    let outlined = step(custom, vec4<f32>(-0.5));
    let near = outlined * (1.0 - smoothstep(vec4<f32>(0.0), vec4<f32>(1.0), clamp(-custom - 1.0, vec4<f32>(0.0), vec4<f32>(1.0))));
    let edge = max(max(near.x, near.y), max(near.z, near.w));
    return vec4<f32>(0.0, 0.0, 0.0, outlineDark * edge * red);
}
