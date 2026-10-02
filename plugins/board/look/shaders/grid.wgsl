// The board's grid over its tiles seen from above (board's gridLines), one triangle over the
// viewport: the pixel (x, y) of it looks from U.RayAt + x·U.RayDX + y·U.RayDY along U.RayDir +
// x·U.RayDDX + y·U.RayDDY (camera.RayField) down to the ground at 0. A square grid of U.GridCells
// cells U.GridCell across darkens a tile along its edges as an outlined tile is (outlineDark, a
// pixel in); a hex grid of pointy-top cells U.GridCell from centre to corner draws their edges a
// pixel wide in U.GridLine, premultiplied, as dark as by both cells' outlines. U.GridWrap says which axes wrap: there the cells run
// on; elsewhere only the board's own are drawn.

@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> @builtin(position) vec4<f32> {
    let uv = vec2<f32>(f32((vid << 1u) & 2u), f32(vid & 2u));
    return vec4<f32>(uv * 2.0 - 1.0, 0.0, 1.0);
}

@fragment
fn fs_main(@builtin(position) frag: vec4<f32>) -> @location(0) vec4<f32> {
    let px = frag.xy - U.ViewAt;
    let from = U.RayAt + px.x * U.RayDX + px.y * U.RayDY;
    let way = U.RayDir + px.x * U.RayDDX + px.y * U.RayDDY;
    let p = (from + way * (-from.z / min(way.z, -1e-6))).xy;
    let unit = max(length(dpdx(p)), length(dpdy(p))); // world units a pixel spans
    if way.z >= 0.0 {
        discard; // it never meets the ground
    }
    if U.GridHex > 0.5 {
        return hexEdge(p, unit);
    }
    return squareEdge(p, unit);
}

// squareEdge darkens the tile at p as far as it lies within a pixel of its edges.
fn squareEdge(p: vec2<f32>, unit: f32) -> vec4<f32> {
    let g = p / U.GridCell;
    let cell = floor(g);
    if !onBoard(cell) {
        discard;
    }
    let f = fract(g);
    let d = min(f, 1.0 - f) * U.GridCell / unit; // pixels to the nearer edge on each axis
    let near = 1.0 - smoothstep(vec2<f32>(0.0), vec2<f32>(1.0), clamp(d, vec2<f32>(0.0), vec2<f32>(1.0)));
    return vec4<f32>(0.0, 0.0, 0.0, outlineDark * max(near.x, near.y));
}

// hexEdge is the hex grid's line at p, where the cell p lies in, or the one across the edge nearest
// it, is the board's.
fn hexEdge(p: vec2<f32>, unit: f32) -> vec4<f32> {
    let s = U.GridCell;
    let x = p.x - s;
    let y = p.y - s;
    let cell = axialRound(vec2<f32>((sqrt(3.0) / 3.0 * x - y / 3.0) / s, (2.0 / 3.0 * y) / s));
    let centre = vec2<f32>(s * (sqrt(3.0) * cell.x + sqrt(3.0) / 2.0 * cell.y) + s, s * 1.5 * cell.y + s);
    let v = p - centre;
    // the edges face these ways, each towards a neighbour: (1, 0), (0, 1), (-1, 1) in axial steps
    let k = vec3<f32>(v.x, 0.5 * v.x + sqrt(3.0) / 2.0 * v.y, -0.5 * v.x + sqrt(3.0) / 2.0 * v.y);
    let a = abs(k);
    var step = vec2<f32>(sign(k.x), 0.0);
    if a.y >= a.x && a.y >= a.z {
        step = vec2<f32>(0.0, sign(k.y));
    } else if a.z >= a.x && a.z >= a.y {
        step = vec2<f32>(-sign(k.z), sign(k.z));
    }
    let here = onBoard(cell);
    let there = onBoard(cell + step);
    if !here && !there {
        discard;
    }
    let d = (s * sqrt(3.0) / 2.0 - max(a.x, max(a.y, a.z))) / unit; // pixels to the nearest edge
    let line = U.GridLine * clamp(1.0 - d, 0.0, 1.0);
    if here && there {
        return line * (2.0 - line.a); // outlined by the cells either side, as twice over
    }
    return line;
}

// axialRound is the axial cell the fractional axial point c lies in.
fn axialRound(c: vec2<f32>) -> vec2<f32> {
    let s = -c.x - c.y;
    var q = round(c.x);
    var r = round(c.y);
    let t = round(s);
    let dq = abs(q - c.x);
    let dr = abs(r - c.y);
    let ds = abs(t - s);
    if dq > dr && dq > ds {
        q = -r - t;
    } else if dr > ds {
        r = -q - t;
    }
    return vec2<f32>(q, r);
}

// onBoard reports whether the cell at column or axial cell c is the board's: any along an axis
// that wraps.
fn onBoard(c: vec2<f32>) -> bool {
    let inX = U.GridWrap.x > 0.5 || (c.x >= 0.0 && c.x < U.GridCells.x);
    let inY = U.GridWrap.y > 0.5 || (c.y >= 0.0 && c.y < U.GridCells.y);
    return inX && inY;
}
