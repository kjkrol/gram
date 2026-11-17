// The sea's surface: every lattice corner of the cells whose ground goes under the sea's level,
// 0, a vertex at that level with how deep the ground lies under it, two triangles a cell
// (terrain.Renderer lays them); drawn among what is seen through, over the ground it does not
// hide, the shallower the water the more of the ground showing through it.

struct Surface {
    @builtin(position) clip: vec4<f32>,
    @location(0) world: vec3<f32>,
    @location(1) deep: f32, // world units the ground lies under the level, less than 0 over it
    @location(2) cloud: f32, // how much of the sun the clouds take
}

// vs_main lays the lattice corner vid, counted row by row, at the sea's level, sunk by U.Bend as
// the ground's corner is (mesh.wgsl), under as much of the clouds' shadow as falls on it.
@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> Surface {
    let cols = u32(U.Corners.x);
    let i = vec2<f32>(f32(vid % cols), f32(vid / cols));
    let p = vec3<f32>(i * U.Cell, 0.0);
    let off = p.xy - U.Eye.xy;
    var s = Surface(U.ViewProj * vec4<f32>(p.xy, -U.Bend * dot(off, off), 1.0), p, -corner(i), 0.0);
    if LEVEL <= 4u && U.Cover > 0.0 && U.CloudsShadeGround > 0.5 {
        s.cloud = cloudsShadeAt(p, U.Cell);
    }
    return s;
}

// the share of U.SeaClear over which the water's edge comes in from nothing
const seaEdge: f32 = 0.05;

// fs_main is the sea's colour at its point, premultiplied, drawn at the level LEVEL as the
// ground's water is (mesh.wgsl): its glint, waves and shore over the water's own colour, the
// board painted, turned to U.SeaShallows over shallow ground (all but 1/e of it gone U.SeaShoal
// deep, none where that is 0), which hides the ground under it the more, the longer the eye's
// way through the water (U.SeaClear world units hiding all but 1/e); in the sun but for the
// clouds' shadow, the shade baked lying on the ground under it.
@fragment
fn fs_main(s: Surface) -> @location(0) vec4<f32> {
    let p = s.world;
    let moveX = dpdx(p);
    let moveY = dpdy(p);
    let span = U.Cell / max(min(length(moveX.xy), length(moveY.xy)), 1e-6);
    let pixel = max(length(moveX.xy), length(moveY.xy));
    var toward = U.Toward;
    if U.Perspective > 0.5 {
        toward = normalize(U.Eye - p);
    }
    let still = waterAt(p.xy, vec2<f32>(1.0, 0.0));
    let shown = still.b * smoothstep(0.0, seaEdge * U.SeaClear, s.deep);
    if shown <= 0.004 {
        discard;
    }
    let sun = lightWay();
    var taken = f32(takesRelief | takesBlocks | takesClouds | takesLamps);
    if LEVEL <= 3u {
        taken = takesOf(flagsAt(p.xy));
    }
    var lit = 0.0;
    if sun.z > 0.0 {
        lit = 1.0;
    }
    var cover = s.cloud;
    if !takes(taken, takesClouds) {
        cover = 0.0;
    }
    let wet = lit * (1.0 - cover);
    var shore = vec4<f32>(0.0);
    if span >= shoreFrom {
        shore = shoreAt(p.xy);
    }
    let sea = SeaGlintAt(p.xy, still.g / still.b, wet, shore, pixel, toward);
    var water = levelAlbedo(p.xy);
    if U.SeaShoal > 0.0 {
        water = mix(water, U.SeaShallows, exp(-s.deep / U.SeaShoal));
    }
    let body = water * lightOn(vec3<f32>(0.0, 0.0, 1.0), sun, p, lit, taken);
    let through = exp(-s.deep / max(toward.z, 0.05) / U.SeaClear);
    var c = vec4<f32>(sea.rgb + body * (1.0 - through) * (1.0 - sea.a),
        1.0 - through * (1.0 - sea.a)) * shown;
    if cover > 0.0 {
        c = vec4<f32>(c.rgb * (1.0 - cloudShade(cover)), c.a);
    }
    if U.Visibility > 0.0 {
        c = vec4<f32>(mix(c.rgb, U.Fog * c.a, 1.0 - exp(-hazeDepth(p) / U.Visibility)), c.a);
    }
    if U.Sight.w > 0.0 {
        let fog = smoothstep(U.Sight.z, U.Sight.w, distance(p.xy, U.Sight.xy));
        c = vec4<f32>(mix(c.rgb, U.Fog * c.a, fog), c.a);
    }
    return c;
}
