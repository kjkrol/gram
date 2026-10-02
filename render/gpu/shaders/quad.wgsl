// The vertex stage of a program drawing quads: vertices in the target's pixels, handed to its
// Fragment with the source position, the colour and the custom values interpolated.

struct VertexIn {
    @location(0) dst: vec2<f32>,
    @location(1) src: vec2<f32>,
    @location(2) color: vec4<f32>,
    @location(3) custom: vec4<f32>,
}

struct VertexOut {
    @builtin(position) position: vec4<f32>,
    @location(0) src: vec2<f32>,
    @location(1) color: vec4<f32>,
    @location(2) custom: vec4<f32>,
}

@vertex
fn vs_main(v: VertexIn) -> VertexOut {
    var o: VertexOut;
    let dst = v.dst * D.place.xy + D.place.zw;
    o.position = vec4<f32>(dst.x / D.target.x * 2.0 - 1.0, 1.0 - dst.y / D.target.y * 2.0, 0.0, 1.0);
    o.src = v.src;
    o.color = v.color;
    if v.color.a <= 1.5 {
        o.color = vec4<f32>(v.color.rgb * D.tint.rgb, v.color.a); // a plain colour, not an overlay's data
    }
    o.custom = v.custom;
    return o;
}

@fragment
fn fs_main(v: VertexOut) -> @location(0) vec4<f32> {
    return Fragment(v.position, v.src, v.color, v.custom);
}

