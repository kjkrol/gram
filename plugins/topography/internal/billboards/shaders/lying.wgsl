// What lies on the ground in relief (topography's sprites): an instance an entity — its corners in
// the world at their heights (top-left, top-right, bottom-left, bottom-right), each with the ground
// under it, its sprite's corners on the atlas, the first image, the light it is drawn in with how
// hazed it is, how much of the light is the sun's, how far into the relief's shadow it fades into
// it and the shade it takes (render.Taking) — two triangles split along the diagonal whose corners
// stand nearer, as the ground's cells are, at the depths of its points nudged towards the eye:
// the ground it lies on passes behind it.

struct Lain {
    @builtin(position) clip: vec4<f32>,
    @location(0) uv: vec2<f32>,
    @location(1) @interpolate(flat) light: vec4<f32>,
    @location(2) world: vec3<f32>,
    @location(3) over: f32,
    @location(4) @interpolate(flat) shade: vec4<f32>,
}

@vertex
fn vs_main(
    @builtin(vertex_index) vid: u32,
    @location(0) tl: vec4<f32>,
    @location(1) tr: vec4<f32>,
    @location(2) bl: vec4<f32>,
    @location(3) br: vec4<f32>,
    @location(4) uv: vec4<f32>,
    @location(5) light: vec4<f32>,
    @location(6) shade: vec4<f32>,
) -> Lain {
    var corners = array<vec3<f32>, 4>(tl.xyz, tr.xyz, bl.xyz, br.xyz);
    var grounds = array<f32, 4>(tl.w, tr.w, bl.w, br.w);
    var uvs = array<vec2<f32>, 4>(uv.xy, uv.zy, uv.xw, uv.zw);
    var split = array<u32, 6>(0u, 1u, 3u, 0u, 3u, 2u);
    if abs(tl.z - br.z) >= abs(tr.z - bl.z) {
        split = array<u32, 6>(0u, 1u, 2u, 1u, 3u, 2u);
    }
    let k = split[vid];
    let p = corners[k];
    let at = project(p + nudge(p));
    var l: Lain;
    l.clip = project(p);
    if at.w > 0.0 {
        l.clip.z = at.z / at.w * l.clip.w;
    }
    l.uv = uvs[k];
    l.light = light;
    l.world = p;
    l.over = p.z - grounds[k];
    l.shade = shade;
    return l;
}

// fs_main is the sprite's texel in its light, turned to the Fog as hazed as it is; what is less
// than half there is left out.
@fragment
fn fs_main(l: Lain) -> @location(0) vec4<f32> {
    let c = imageSrc0At(l.uv);
    if c.a < 0.5 {
        discard;
    }
    // the relief's shadow and the clouds': the blocks' would shade what casts them
    var reached = 1.0;
    if takes(l.shade.z, takesRelief) {
        reached = groundShadeAt(l.world.xy, l.over, l.shade.y);
    }
    var clouds = 1.0;
    if U.Cover > 0.0 && U.CloudsShadeFigures > 0.5 && takes(l.shade.z, takesClouds) {
        clouds = 1.0 - cloudShade(cloudsShadeAt(l.world, 1.0));
    }
    let shade = (1.0 - l.shade.x * (1.0 - reached)) * clouds;
    return vec4<f32>(mix(c.rgb * l.light.rgb * shade, U.Fog * c.a, l.light.a), c.a);
}
