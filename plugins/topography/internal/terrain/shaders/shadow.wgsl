// The shadows of what stands, laid on the ground (terrain.Renderer.DrawShadows): an instance a
// patch (sky.Patch) — its middle and the way away from the sun, how far it reaches along and
// across that way, how far in it fades and how dark it is — a grid of shadowGrid by shadowGrid
// pieces over it, each corner at the ground's height, nudged towards the eye to lie over it.

struct Shade {
    @builtin(position) clip: vec4<f32>,
    @location(0) local: vec2<f32>,
    @location(1) @interpolate(flat) size: vec4<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vid: u32, @location(0) at: vec4<f32>, @location(1) size: vec4<f32>) -> Shade {
    var across = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var down = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    let piece = vid / 6u;
    let k = vid % 6u;
    let g = (vec2<f32>(f32(piece % shadowGrid), f32(piece / shadowGrid)) + vec2<f32>(across[k], down[k])) / f32(shadowGrid);
    let local = (g * 2.0 - 1.0) * size.xy;
    let way = at.zw;
    let xy = at.xy + way * local.x + vec2<f32>(-way.y, way.x) * local.y;
    var p = vec3<f32>(xy, height(xy));
    p += nudge(p);
    let off = p.xy - U.Eye.xy;
    var s: Shade;
    s.clip = U.ViewProj * vec4<f32>(p.xy, p.z - U.Bend * dot(off, off), 1.0);
    s.local = local;
    s.size = size;
    return s;
}

// nudge moves the ground point p towards the eye, so what lies on the ground passes the depth
// test over it.
fn nudge(p: vec3<f32>) -> vec3<f32> {
    var toward = -U.Look;
    if U.Perspective > 0.5 {
        toward = U.Eye - p;
    }
    return normalize(toward) * shadowLift;
}

@fragment
fn fs_main(s: Shade) -> @location(0) vec4<f32> {
    let edges = vec4<f32>(s.size.x + s.local.x, s.size.x - s.local.x, s.size.y + s.local.y, s.size.y - s.local.y) / max(s.size.z, 1e-3);
    let f = smoothstep(vec4<f32>(0.0), vec4<f32>(1.0), clamp(edges, vec4<f32>(0.0), vec4<f32>(1.0)));
    return vec4<f32>(0.0, 0.0, 0.0, s.size.w * f.x * f.y * f.z * f.w);
}

// How many pieces a side a patch is laid in, following the ground, and how far towards the eye it
// is nudged, world units.
const shadowGrid: u32 = 6u;
const shadowLift: f32 = 0.75;
