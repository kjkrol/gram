// The ground of a hex board in relief (topography's hexes): every cell an instance — its centre
// (x, y), level (z) and the shade it takes (w, render.Taking); the heights of its six corners,
// clockwise from the top; the edges it has on the board's edge (bits, edge k from corner k to
// k+1) and its ordinal — of pointy-top cells U.HexSize from centre to corner, U.Facing.x a row.
// Its top is six triangles from its centre at its level to its corners at theirs, then a wall
// down to U.Base from each edge on the board's edge. Drawn through U.ViewProj (camera.Transform)
// sunk U.Bend·d² under U.Eye's level; coloured from the board from above, the first image,
// U.TopPx pixels a world unit, lit by the sun the way the ground faces, blended from the
// corners' (the second image) and the centre's, shaded by the clouds, outlined in the grid's line
// U.GridLine, premultiplied, U.GridWidth pixels wide (0 none), hazed as much air as lies before
// it (hazeDepth) over U.Visibility.

struct Face {
    @builtin(position) clip: vec4<f32>,
    @location(0) world: vec3<f32>,
    @location(1) sample: vec2<f32>,
    @location(2) normal: vec3<f32>,
    @location(3) edge: f32,  // on a top, 1 at the centre, 0 on the edge; -1 on a wall
    @location(4) cloud: f32, // how much the clouds darken it
    @location(5) @interpolate(flat) takes: f32,
}

// how far in from its edge a wall takes its colour from the top, and how far towards its centre a
// top takes its own: clear of the sprite's rough edge and the neighbour's colour past it
const faceIn: f32 = 0.15;
const topIn: f32 = 0.1;

// corner is where the corner i of the cell of ordinal o lies, clockwise from the top: on the
// lattice of the cells' triangles (topography's relief), the point (i, j) at
// (S + i·S·√3/2, S + i·S/2 + j·S) and the cell (q, r)'s centre at (2q + r, r - q), so the cells
// meeting at a corner lay it alike to the bit.
fn corner(o: u32, i: u32) -> vec2<f32> {
    let cols = u32(U.Facing.x);
    let q = i32(o % cols);
    let r = i32(o / cols);
    var di = array<i32, 6>(0, 1, 1, 0, -1, -1);
    var dj = array<i32, 6>(-1, -1, 0, 1, 1, 0);
    let li = f32(2 * q + r + di[i % 6u]);
    let lj = f32(r - q + dj[i % 6u]);
    let s = U.HexSize;
    return vec2<f32>(s + li * s * 0.8660254, s + li * s * 0.5 + lj * s);
}

// facing is the way the ground faces at the corner i of the cell of ordinal o, from the second
// image: a texel a corner in the relief's order (relief.HexCorners), U.Facing.y a row, (n+1)/2.
fn facing(o: u32, i: u32) -> vec3<f32> {
    let cols = u32(U.Facing.x);
    let top = 2u * ((o / cols) * (cols + 1u) + o % cols) + 1u;
    let row = 2u * (cols + 1u);
    var at = array<u32, 6>(top, top + 1u, top + row, top + row - 1u, top + row - 2u, top - 1u);
    let k = at[i % 6u];
    let wide = u32(U.Facing.y);
    let texel = vec2<f32>(f32(k % wide), f32(k / wide));
    return imageSrc1At(imageSrc1Origin() + texel + 0.5).rgb * 2.0 - 1.0;
}

// centreFacing is the way the ground faces at a cell's centre, its corners h high: the mean of
// its six triangles' slopes.
fn centreFacing(h: array<f32, 6>) -> vec3<f32> {
    let gx = 0.8660254 * (h[1] + h[2] - h[4] - h[5]);
    let gy = -h[0] - 0.5 * h[1] + 0.5 * h[2] + h[3] + 0.5 * h[4] - 0.5 * h[5];
    return normalize(vec3<f32>(-gx, -gy, 3.0 * U.HexSize));
}

@vertex
fn vs_main(
    @builtin(vertex_index) vid: u32,
    @location(0) cell: vec4<f32>,
    @location(1) h03: vec4<f32>,
    @location(2) h45: vec4<f32>,
) -> Face {
    let c = cell.xy;
    var hs = array<f32, 6>(h03.x, h03.y, h03.z, h03.w, h45.x, h45.y);
    let rim = u32(h45.z);
    let o = u32(h45.w);
    var p = vec3<f32>(c, cell.z);
    var f = Face(vec4<f32>(0.0), vec3<f32>(0.0), vec2<f32>(0.0), vec3<f32>(0.0, 0.0, 1.0), 1.0,
        0.0, cell.w);
    if vid < 18u {
        let t = vid / 3u;
        let k = vid % 3u;
        if k == 0u {
            f.normal = centreFacing(hs);
        } else {
            let i = (t + k - 1u) % 6u;
            p = vec3<f32>(corner(o, i), hs[i]);
            f.normal = facing(o, i);
            f.edge = 0.0;
        }
        f.sample = mix(p.xy, c, topIn);
    } else {
        let s = (vid - 18u) / 6u;
        var along = array<u32, 6>(0u, 1u, 0u, 1u, 1u, 0u);
        var down = array<bool, 6>(false, false, true, false, true, true);
        let k = (vid - 18u) % 6u;
        let i = (s + along[k]) % 6u;
        p = vec3<f32>(corner(o, i), hs[i]);
        // within the board the cells' tops meet at their corners: a wall on its edge alone
        if down[k] && ((rim >> s) & 1u) == 1u {
            p.z = U.Base;
        }
        f.sample = mix(p.xy, c, faceIn);
        let a = -1.0471976 + f32(s) * 1.0471976;
        f.normal = vec3<f32>(cos(a), sin(a), 0.0);
        f.edge = -1.0;
    }
    let off = p.xy - U.Eye.xy;
    f.clip = U.ViewProj * vec4<f32>(p.xy, p.z - U.Bend * dot(off, off), 1.0);
    f.world = p;
    if U.Cover > 0.0 && U.CloudsShadeGround > 0.5 && takes(cell.w, takesClouds) {
        f.cloud = cloudShade(cloudsShadeAt(p, U.HexSize)); // the noise as fine as a cell shows it
    }
    return f;
}

@fragment
fn fs_main(f: Face) -> @location(0) vec4<f32> {
    let unit = max(length(dpdx(f.world.xy)), length(dpdy(f.world.xy))); // world units a pixel spans
    let albedo = imageSrc0Linear(imageSrc0Origin() + f.sample * U.TopPx);
    let sun = lightWay();
    let n = normalize(f.normal);
    var light = U.Ambience + U.LightColor * max(dot(n, sun), 0.0) * U.LightStrength;
    if takes(f.takes, takesLamps) {
        light += lampLightAt(f.world);
    }
    var rgb = albedo.rgb * light;
    rgb *= 1.0 - f.cloud;
    if U.GridWidth > 0.0 && f.edge >= 0.0 {
        let d = f.edge * U.HexSize * 0.8660254 / max(unit, 1e-6); // pixels to the cell's edge
        let half = U.GridWidth * 0.5;
        let e = 1.0 - smoothstep(half - 0.5, half + 0.5, d);
        rgb = rgb * (1.0 - U.GridLine.a * e) + U.GridLine.rgb * e;
    }
    if U.Visibility > 0.0 {
        rgb = mix(rgb, U.Fog, 1.0 - exp(-hazeDepth(f.world) / U.Visibility));
    }
    // the fog before the camera's sight ends (U.Sight: from where, where it begins and ends)
    if U.Sight.w > 0.0 {
        rgb = mix(rgb, U.Fog, smoothstep(U.Sight.z, U.Sight.w, distance(f.world.xy, U.Sight.xy)));
    }
    return vec4<f32>(rgb, 1.0);
}

// hazeDepth is how much air lies between the eye and the world point p, world units: its
// distance from U.Eye, or seen from above (U.HazeUp) the air over p up to that height, along the
// lines of sight U.HazeWay.
fn hazeDepth(p: vec3<f32>) -> f32 {
    if U.HazeUp <= 0.0 {
        return distance(U.Eye, p);
    }
    return max(U.HazeUp - p.z, 0.0) / max(-U.HazeWay.z, 0.05);
}
