// Ward is the demo's own material: rings spreading from a centre, shimmering with the library's
// noise, pulsing on the composer's Clock — worked out per pixel where p lies in the world. The
// centre sits in custom.xy, the reach in custom.z (world units); red carries the glow's strength.
fn Ward(p: vec2<f32>, red: f32, fraction: f32, custom: vec4<f32>) -> vec4<f32> {
    let d = distance(p, custom.xy) / custom.z;
    if d >= 1.0 {
        return vec4<f32>(0.0);
    }
    let rings = 0.5 + 0.5 * sin(d * 18.0 - U.Clock * 2.5);
    let shimmer = 0.75 + 0.5 * noise(p * 0.11 + vec2<f32>(U.Clock * 0.4, -U.Clock * 0.3));
    let glow = red * rings * shimmer * (1.0 - d) * (1.0 - d);
    let tint = mix(vec3<f32>(0.25, 0.9, 1.0), vec3<f32>(0.8, 0.4, 1.0), d);
    return vec4<f32>(tint * glow, glow); // premultiplied: the ward glazes what lies under it
}
