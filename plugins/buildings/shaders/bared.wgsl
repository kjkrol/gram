// A room an uncovered entity stands in, an instance of its inside (interior.wgsl), the first
// three of its faces, into the depth of the uncovered (render.Uncovered): its floor and the two
// walls across it from the eye up to its ceiling, so whatever lies before the room gives way to it
// whole.

@vertex
fn vs_main(
    @builtin(vertex_index) vid: u32,
    @location(0) a: vec4<f32>, // its middle, its floor and its ceiling
    @location(1) b: vec4<f32>, // half its length and its width, the way its length runs
    @location(2) c: vec4<f32>,
    @location(3) d: vec4<f32>,
    @location(4) e: vec4<f32>, // the floor's colour, how thick its walls are
    @location(5) f: vec4<f32>,
) -> @builtin(position) vec4<f32> {
    var across = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var up = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    let along = b.zw;
    let side = vec2<f32>(-along.y, along.x);
    let half = b.xy;
    let u = across[vid % 6u] * 2.0 - 1.0;
    let v = up[vid % 6u];
    // within the walls, as thick as e.w says
    let inner = half - vec2<f32>(e.w);
    var p = vec3<f32>(a.xy + along * u * inner.x + side * (v * 2.0 - 1.0) * inner.y, a.z);
    if vid >= 12u {
        let sign = -facing(a.xy, along * half.x, -side * half.y, a.z, a.w);
        p = vec3<f32>(a.xy + along * inner.x * sign - side * u * inner.y, mix(a.z, a.w, v));
    } else if vid >= 6u {
        let sign = -facing(a.xy, side * half.y, along * half.x, a.z, a.w);
        p = vec3<f32>(a.xy + along * u * inner.x + side * inner.y * sign, mix(a.z, a.w, v));
    }
    var clip = clipOf(p);
    clip.z = uncoveredAt(clip.z / max(clip.w, 1e-6)) * clip.w;
    return clip;
}

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    return vec4<f32>(0.0);
}
