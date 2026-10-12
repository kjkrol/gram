// Shimmer is the demo's water: the still colour with ripples running across it, worked out per
// pixel on the composer's Clock — a cell kind's look as a material instead of a drawn sprite.
// The renderer fills the inputs with the cell's own: its middle in custom.xy, half its width in
// custom.z; p lies in the world, so the ripples run across cell borders unbroken.
fn Shimmer(p: vec2<f32>, red: f32, fraction: f32, custom: vec4<f32>) -> vec4<f32> {
    let base = vec3<f32>(0.196, 0.392, 0.706);
    let run = p * 0.11 + vec2<f32>(U.Clock * 0.9, sin(U.Clock * 0.5) * 0.4);
    let ripple = 0.5 + 0.5 * sin(run.x + run.y * 2.1 + 3.0 * noise(p * 0.07 + vec2<f32>(U.Clock * 0.2, 0.0)));
    let lit = base + vec3<f32>(0.12, 0.12, 0.10) * smoothstep(0.72, 0.95, ripple);
    return vec4<f32>(lit, 1.0); // opaque: the tile is the material, nothing drawn under it
}
