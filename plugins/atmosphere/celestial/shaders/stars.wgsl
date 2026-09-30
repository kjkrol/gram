// The real stars (celestial.StarField), under the sky drawn over them: an instance a star, a soft
// dot of light round at.xy on the screen, U.StarView pixels, spread at.z pixels, as bright as at.w,
// in its colour.

struct Dot {
    @builtin(position) clip: vec4<f32>,
    @location(0) off: vec2<f32>,
    @location(1) light: vec4<f32>,
}

// vs_main lays the corner vid of the star's square, three spreads and a half pixel from it.
@vertex
fn vs_main(@builtin(vertex_index) vid: u32, @location(0) at: vec4<f32>, @location(1) colour: vec4<f32>) -> Dot {
    var across = array<f32, 6>(-1.0, 1.0, -1.0, 1.0, 1.0, -1.0);
    var down = array<f32, 6>(-1.0, -1.0, 1.0, -1.0, 1.0, 1.0);
    let off = vec2<f32>(across[vid], down[vid]) * (3.0 * at.z + 0.5);
    let px = at.xy + off;
    var d: Dot;
    d.clip = vec4<f32>(px.x / U.StarView.x * 2.0 - 1.0, 1.0 - px.y / U.StarView.y * 2.0, 0.0, 1.0);
    d.off = off / at.z;
    d.light = vec4<f32>(colour.rgb, at.w);
    return d;
}

@fragment
fn fs_main(d: Dot) -> @location(0) vec4<f32> {
    let a = d.light.a * exp(-0.5 * dot(d.off, d.off));
    return vec4<f32>(d.light.rgb * a, a);
}
