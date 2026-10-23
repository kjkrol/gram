// The tiles' passes drawn pixel by pixel (look's tiles.go), one triangle over the viewport: the
// pixel (x, y) looks from U.RayAt + x·U.RayDX + y·U.RayDY along U.RayDir + x·U.RayDDX + y·U.RayDDY
// (camera.RayField) down to the ground at 0, drawn there with the ground's depth where U.TileDeep.

@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> @builtin(position) vec4<f32> {
    let uv = vec2<f32>(f32((vid << 1u) & 2u), f32(vid & 2u));
    return vec4<f32>(uv * 2.0 - 1.0, 0.0, 1.0);
}

struct Laid {
    @location(0) colour: vec4<f32>,
    @builtin(frag_depth) depth: f32,
}

// groundAt is the point on the ground at 0 the pixel at frag looks at, z 0 where it never meets it.
fn groundAt(frag: vec4<f32>) -> vec3<f32> {
    let px = frag.xy - U.ViewAt;
    let from = U.RayAt + px.x * U.RayDX + px.y * U.RayDY;
    let way = U.RayDir + px.x * U.RayDDX + px.y * U.RayDDY;
    if way.z >= 0.0 {
        return vec3<f32>(0.0);
    }
    return vec3<f32>((from + way * (-from.z / min(way.z, -1e-6))).xy, 1.0);
}

// laidAt is laid, the ground's depth at p with it where U.TileDeep.
fn laidAt(laid: vec4<f32>, p: vec2<f32>) -> Laid {
    var out: Laid;
    out.colour = laid;
    out.depth = 0.0;
    if U.TileDeep > 0.5 {
        let at = U.TileViewProj * vec4<f32>(p, 0.0, 1.0);
        out.depth = at.z / at.w;
    }
    return out;
}

// plainly reports whether the plain pass lays the cell flagged so: drawn by its sprite, nothing
// swaying on or beside it where the tiles sway.
fn plainly(flags: u32) -> bool {
    var mask = flagShaded;
    if U.TileSways > 0.5 {
        mask |= flagNearSway;
    }
    return (flags & mask) == 0u;
}
