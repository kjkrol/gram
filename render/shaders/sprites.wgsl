// Sprites drawn as instances (render.Sprites), six vertices each: its rectangle on the screen
// (rect, pixels of a viewport U.ViewSize large), the part of the atlas over it (src, the first
// image's pixels), the light it is drawn in (light.rgb) — as the composer draws a plain sprite —
// and the angle it is turned by about the rectangle's middle (light.w, radians, 0 east against
// the clock with y growing down).

struct Piece {
    @builtin(position) clip: vec4<f32>,
    @location(0) src: vec2<f32>,
    @location(1) @interpolate(flat) light: vec4<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vid: u32, @location(0) rect: vec4<f32>, @location(1) src: vec4<f32>, @location(2) light: vec4<f32>) -> Piece {
    var across = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var down = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    let k = vec2<f32>(across[vid], down[vid]);
    let centre = (rect.xy + rect.zw) * 0.5;
    let off = (k - vec2<f32>(0.5, 0.5)) * (rect.zw - rect.xy);
    let c = cos(light.w);
    let s = sin(light.w);
    let px = centre + vec2<f32>(off.x * c + off.y * s, off.y * c - off.x * s);
    var p: Piece;
    p.clip = vec4<f32>(px.x / U.ViewSize.x * 2.0 - 1.0, 1.0 - px.y / U.ViewSize.y * 2.0, 0.0, 1.0);
    p.src = mix(src.xy, src.zw, k);
    p.light = light;
    return p;
}

@fragment
fn fs_main(p: Piece) -> @location(0) vec4<f32> {
    return imageSrc0At(p.src) * vec4<f32>(p.light.rgb, 1.0);
}
