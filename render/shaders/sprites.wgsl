// Sprites drawn as instances (render.Sprites), six vertices each: its rectangle on the screen
// (rect, pixels of a viewport U.ViewSize large), the part of the atlas over it (src, the first
// image's pixels) and the light it is drawn in (light) — as the composer draws a plain sprite.

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
    let px = mix(rect.xy, rect.zw, k);
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
