// The ground of a hex board in relief (topography's hexes): every cell a prism, an instance —
// its centre (x, y) and top (z), the tops of the neighbours across its six edges (low) — of
// pointy-top cells U.HexSize from centre to corner; its top a fan of six triangles, then a face
// down each edge to the neighbour's top, none where that is as high. Drawn through U.ViewProj
// (camera.Transform) sunk U.Bend·d² under U.Eye's level; coloured from the board from above, the
// first image, U.TopPx pixels a world unit, lit by the sun on its faces, shaded by the clouds,
// outlined where U.GridOn, hazed as far off as it lies over U.Visibility.

struct Face {
    @builtin(position) clip: vec4<f32>,
    @location(0) world: vec3<f32>,
    @location(1) sample: vec2<f32>,
    @location(2) @interpolate(flat) normal: vec3<f32>,
    @location(3) edge: f32,  // on a top, 1 at the centre, 0 on the edge; -1 on a face
    @location(4) cloud: f32, // how much the clouds darken it
}

// how far in from its edge a face takes its colour from the top, and how far towards its centre a
// top takes its own: clear of the sprite's rough edge and the neighbour's colour past it
const faceIn: f32 = 0.15;
const topIn: f32 = 0.1;

// corner is the i-th corner of the cell centred at c, clockwise from the top.
fn corner(c: vec2<f32>, i: u32) -> vec2<f32> {
    let a = -1.5707963 + f32(i % 6u) * 1.0471976;
    return c + U.HexSize * vec2<f32>(cos(a), sin(a));
}

@vertex
fn vs_main(@builtin(vertex_index) vid: u32, @location(0) cell: vec4<f32>, @location(1) low03: vec4<f32>, @location(2) low45: vec4<f32>) -> Face {
    let c = cell.xy;
    let top = cell.z;
    var p: vec3<f32>;
    var f: Face;
    if vid < 18u {
        let t = vid / 3u;
        let k = vid % 3u;
        p = vec3<f32>(c, top);
        f.edge = 1.0;
        if k == 1u {
            p = vec3<f32>(corner(c, t), top);
            f.edge = 0.0;
        } else if k == 2u {
            p = vec3<f32>(corner(c, t + 1u), top);
            f.edge = 0.0;
        }
        f.sample = mix(p.xy, c, topIn);
        f.normal = vec3<f32>(0.0, 0.0, 1.0);
    } else {
        let s = (vid - 18u) / 6u;
        var lows = array<f32, 6>(low03.x, low03.y, low03.z, low03.w, low45.x, low45.y);
        let low = min(lows[s], top);
        var along = array<u32, 6>(0u, 1u, 0u, 1u, 1u, 0u);
        var down = array<bool, 6>(false, false, true, false, true, true);
        let k = (vid - 18u) % 6u;
        let q = corner(c, s + along[k]);
        p = vec3<f32>(q, select(top, low, down[k]));
        f.sample = mix(q, c, faceIn);
        let a = -1.0471976 + f32(s) * 1.0471976;
        f.normal = vec3<f32>(cos(a), sin(a), 0.0);
        f.edge = -1.0;
    }
    let off = p.xy - U.Eye.xy;
    f.clip = U.ViewProj * vec4<f32>(p.xy, p.z - U.Bend * dot(off, off), 1.0);
    f.world = p;
    f.cloud = 0.0;
    if U.Cover > 0.0 {
        f.cloud = cloudShade(cloudCover(cloudField(p.xy)));
    }
    return f;
}

@fragment
fn fs_main(f: Face) -> @location(0) vec4<f32> {
    let unit = max(length(dpdx(f.world.xy)), length(dpdy(f.world.xy))); // world units a pixel spans
    let albedo = imageSrc0Linear(imageSrc0Origin() + f.sample * U.TopPx);
    let sun = sunWay();
    var rgb = albedo.rgb * (U.Ambience + U.SunColor * max(dot(f.normal, sun), 0.0) * U.SunStrength);
    rgb *= 1.0 - f.cloud;
    if U.GridOn > 0.5 && f.edge >= 0.0 {
        let d = f.edge * U.HexSize * 0.8660254 / max(unit, 1e-6); // pixels to the cell's edge
        rgb *= 1.0 - gridDark * clamp(1.0 - d, 0.0, 1.0);
    }
    if U.Visibility > 0.0 {
        rgb = mix(rgb, U.Fog, 1.0 - exp(-distance(U.Eye, f.world) / U.Visibility));
    }
    return vec4<f32>(rgb, 1.0);
}

// gridDark is how much the grid darkens a top along its edge, as the two outlines laid over it.
const gridDark: f32 = 0.72;
