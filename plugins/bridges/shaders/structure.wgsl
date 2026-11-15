// A piece of a bridge's structure an instance: a wall standing along a line, from its top down
// as deep as the instance says at either end, or a slab lying between two lines. Drawn through
// U.SpanViewProj as the ground's mesh is (camera.Transform, sunk by U.SpanBend·d² off
// U.SpanEye), in the frame's light and the air's haze.

struct Piece {
    @builtin(position) clip: vec4<f32>,
    @location(0) world: vec3<f32>,
    @location(1) @interpolate(flat) normal: vec3<f32>,
    @location(2) @interpolate(flat) body: vec3<f32>,
    @location(3) lamps: vec3<f32>,
}

@vertex
fn vs_main(
    @builtin(vertex_index) vid: u32,
    @location(0) a: vec4<f32>, // the line from and to
    @location(1) b: vec4<f32>, // its top at either end, how deep it reaches there
    @location(2) c: vec4<f32>, // a wall: the way it faces; a slab: half its width at either end
    @location(3) d: vec4<f32>, // its colour, its kind: a wall, a slab, a slab seen from below
    @location(4) e: vec4<f32>,
) -> Piece {
    var along = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var across = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    let t = along[vid];
    let v = across[vid];
    var p = vec3<f32>(mix(a.xy, a.zw, t), mix(b.x, b.y, t));
    var n = vec3<f32>(c.xy, 0.0);
    if d.w < 0.5 {
        p.z -= v * mix(b.z, b.w, t);
    } else {
        p = vec3<f32>(p.xy + mix(c.xy, c.zw, t) * (2.0 * v - 1.0), p.z);
        n = vec3<f32>(0.0, 0.0, 1.0);
        if d.w > 1.5 {
            n = vec3<f32>(0.0, 0.0, -1.0);
        }
    }
    var out: Piece;
    let from = p.xy - U.SpanEye.xy;
    out.clip = U.SpanViewProj * vec4<f32>(p.xy, p.z - U.SpanBend * dot(from, from), 1.0);
    out.world = p;
    out.normal = n;
    out.body = d.rgb;
    out.lamps = lampLightAt(p);
    return out;
}

// hazeDepth is how much air lies between the eye and the point p, world units, as the ground's
// shader has it.
fn hazeDepth(p: vec3<f32>) -> f32 {
    if U.SpanHaze.y <= 0.0 {
        return distance(U.SpanEye, p);
    }
    return max(U.SpanHaze.y - p.z, 0.0) / max(-U.SpanHazeWay.z, 0.05);
}

@fragment
fn fs_main(in: Piece) -> @location(0) vec4<f32> {
    let sun = lightWay();
    var day = 0.0;
    if sun.z > 0.0 {
        day = 1.0;
    }
    let diffuse = max(dot(in.normal, sun), 0.0) * day;
    // what the lit ground throws back up a wall and under a deck
    let bounce = 0.45 * (1.0 - max(in.normal.z, 0.0)) * max(sun.z, 0.0);
    let light = U.Ambience + U.LightColor * (diffuse + bounce) * U.LightStrength + in.lamps;
    var rgb = in.body * light;
    if U.SpanHaze.x > 0.0 {
        rgb = mix(rgb, U.Fog, 1.0 - exp(-hazeDepth(in.world) / U.SpanHaze.x));
    }
    if U.SpanSight.w > 0.0 {
        let far = distance(in.world.xy, U.SpanSight.xy);
        rgb = mix(rgb, U.Fog, smoothstep(U.SpanSight.z, U.SpanSight.w, far));
    }
    return vec4<f32>(rgb, 1.0);
}
