// The ground as triangles, an instance each: its three corners in the world, a channel of its
// colour riding with each, and the way up off the ground at each. Drawn through U.MosaicViewProj as the relief's mesh is
// (camera.Transform, sunk by U.MosaicBend·d² off U.MosaicEye), flat in the frame's light.

struct Piece {
    @builtin(position) clip: vec4<f32>,
    @location(0) world: vec3<f32>,
    @location(1) normal: vec3<f32>,
    @location(2) @interpolate(flat) paint: vec3<f32>,
}

@vertex
fn vs_main(
    @builtin(vertex_index) vid: u32,
    @location(0) a: vec4<f32>,
    @location(1) b: vec4<f32>,
    @location(2) c: vec4<f32>,
    @location(3) na: vec4<f32>,
    @location(4) nb: vec4<f32>,
    @location(5) nc: vec4<f32>,
) -> Piece {
    var p = a.xyz;
    var n = na.xyz;
    if vid == 1u {
        p = b.xyz;
        n = nb.xyz;
    } else if vid == 2u {
        p = c.xyz;
        n = nc.xyz;
    }
    var s: Piece;
    let from = p.xy - U.MosaicEye.xy;
    s.clip = U.MosaicViewProj * vec4<f32>(p.xy, p.z - U.MosaicBend * dot(from, from), 1.0);
    s.world = p;
    s.normal = normalize(n + vec3<f32>(0.0, 0.0, 1e-6));
    s.paint = vec3<f32>(a.w, b.w, c.w);
    return s;
}

@fragment
fn fs_main(s: Piece) -> @location(0) vec4<f32> {
    let sun = lightWay();
    var day = 0.0;
    if sun.z > 0.0 {
        day = 1.0;
    }
    let diffuse = max(dot(normalize(s.normal), sun), 0.0) * day;
    let light = U.Ambience + U.LightColor * diffuse * U.LightStrength + lampLightAt(s.world);
    return vec4<f32>(s.paint * light, 1.0);
}
