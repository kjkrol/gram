// The board's tiles laid on the GPU (look's tiles), one triangle over the viewport: the pixel
// (x, y) looks from U.RayAt + x·U.RayDX + y·U.RayDY along U.RayDir + x·U.RayDDX + y·U.RayDDY
// (camera.RayField) down to the ground at 0 and draws what lies there as the board's renderer
// lays its tiles. Each cell's sprite is stretched over its box (U.TileBox), every cell whose box
// covers the point laid in the order the cells are, the covers on each over it, faded in over
// its neighbours (U.TileCovers and on in U.TileCoversOn, U.TileCoverCount of them); then the ways, bands from a cell's
// middle out towards the neighbours its links name and a hub round its middle, cut out of the
// way's sprite laid over the cell's box; all of it in U.TileTint and, where U.TileLit, the light
// shade.wgsl lays on the ground; over it the cells drawn by their material. Where U.TileSways,
// what sways leans with the wind (U.Wind) as air.Weather.Sway leans it.
//
// The sprites are the sheet's (image 0), found through their rectangles (image 1, two pixels a
// sprite, 256 a row: left and top, then right and bottom, a coordinate in two bytes) by what each
// cell shows (image 2, a block of 2×2 pixels a cell: see look's tiles.go). A square board's cells
// are U.TileBox across; a hex board's (U.TileHex) are pointy-top, U.TileSide from centre to
// corner, their axial q and r its columns and rows. Along an axis U.TileWrap says wraps the cells
// run on.

@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> @builtin(position) vec4<f32> {
    let uv = vec2<f32>(f32((vid << 1u) & 2u), f32(vid & 2u));
    return vec4<f32>(uv * 2.0 - 1.0, 0.0, 1.0);
}

struct Laid {
    @location(0) colour: vec4<f32>,
    @builtin(frag_depth) depth: f32,
}

// a cell's flags (look's tiles.go)
const flagShaded: u32 = 1u;
const flagWay: u32 = 2u;
const flagSways: u32 = 4u;

// the share of a cover's weight it fades in over (draw.coverSoft)
const coverSoft: f32 = 0.2;

@fragment
fn fs_main(@builtin(position) frag: vec4<f32>) -> Laid {
    let px = frag.xy - U.ViewAt;
    let from = U.RayAt + px.x * U.RayDX + px.y * U.RayDY;
    let way = U.RayDir + px.x * U.RayDDX + px.y * U.RayDDY;
    if way.z >= 0.0 {
        discard; // it never meets the ground
    }
    let p = (from + way * (-from.z / min(way.z, -1e-6))).xy;
    var here = vec2<f32>(0.0);
    if U.TileHex > 0.5 {
        here = hexAt(p);
    } else {
        here = floor(p / U.TileBox);
    }
    let many = U.TileHex > 0.5 || U.TileSways > 0.5;
    var laid = vec4<f32>(0.0);
    for (var k = 0; k < 9; k++) {
        if !many && k != 4 {
            continue;
        }
        let n = near(here, k);
        if n.z > 0.5 {
            laid = over(tileAt(n.xy, p), laid);
        }
    }
    if U.TileWays > 0.5 {
        for (var k = 0; k < 9; k++) {
            let n = near(here, k);
            if n.z > 0.5 {
                laid = waysOf(n.xy, p, laid);
            }
        }
    }
    laid = vec4<f32>(laid.rgb * U.TileTint * lightAt(here, p), laid.a);
    if U.TileShaded > 0.5 {
        for (var k = 0; k < 9; k++) {
            if U.TileHex < 0.5 && k != 4 {
                continue;
            }
            let n = near(here, k);
            if n.z > 0.5 {
                laid = over(materialAt(n.xy, p), laid);
            }
        }
    }
    if laid.a <= 0.0 {
        discard;
    }
    var out: Laid;
    out.colour = laid;
    out.depth = 0.0;
    if U.TileDeep > 0.5 {
        let at = U.TileViewProj * vec4<f32>(p, 0.0, 1.0);
        out.depth = at.z / at.w;
    }
    return out;
}

// near is the k-th cell round c in the order the cells are laid, row by row: of a square board
// the 3×3 round c, of a hex board c and its six neighbours (k under 7); z 0 past them.
fn near(c: vec2<f32>, k: i32) -> vec3<f32> {
    if U.TileHex > 0.5 {
        let order = array<vec2<f32>, 7>(vec2<f32>(0.0, -1.0), vec2<f32>(1.0, -1.0),
            vec2<f32>(-1.0, 0.0), vec2<f32>(0.0, 0.0), vec2<f32>(1.0, 0.0),
            vec2<f32>(-1.0, 1.0), vec2<f32>(0.0, 1.0));
        if k >= 7 {
            return vec3<f32>(0.0);
        }
        return vec3<f32>(c + order[k], 1.0);
    }
    return vec3<f32>(c + vec2<f32>(f32(k % 3) - 1.0, f32(k / 3) - 1.0), 1.0);
}

// over is a laid over b, both premultiplied.
fn over(a: vec4<f32>, b: vec4<f32>) -> vec4<f32> {
    return a + b * (1.0 - a.a);
}

// hexAt is the axial cell of a hex board the point p lies in.
fn hexAt(p: vec2<f32>) -> vec2<f32> {
    let s = U.TileSide;
    let x = p.x - s;
    let y = p.y - s;
    return axialRound(vec2<f32>((sqrt(3.0) / 3.0 * x - y / 3.0) / s, (2.0 / 3.0 * y) / s));
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

// centreOf is the middle of the cell c, a square board's column and row or a hex board's axial q
// and r, as far along as c is, wrapped or not.
fn centreOf(c: vec2<f32>) -> vec2<f32> {
    if U.TileHex > 0.5 {
        let s = U.TileSide;
        return vec2<f32>(s * (sqrt(3.0) * c.x + sqrt(3.0) / 2.0 * c.y) + s, s * 1.5 * c.y + s);
    }
    return (c + 0.5) * U.TileBox;
}

// folded is the board's own cell c stands for, along an axis that wraps; z 0 off the board.
fn folded(c: vec2<f32>) -> vec3<f32> {
    var f = c;
    if U.TileWrap.x > 0.5 {
        f.x = modulo(f.x, U.TileCells.x);
    }
    if U.TileWrap.y > 0.5 {
        f.y = modulo(f.y, U.TileCells.y);
    }
    let on = f.x >= 0.0 && f.y >= 0.0 && f.x < U.TileCells.x && f.y < U.TileCells.y;
    return vec3<f32>(f, select(0.0, 1.0, on));
}

// byteOf is the byte k, 0 to 3, of the pixel (dx, dy) of the board's cell b's block.
fn byteOf(b: vec2<f32>, dx: f32, dy: f32, k: i32) -> u32 {
    let px = imageSrc2At(imageSrc2Origin() + b * 2.0 + vec2<f32>(dx, dy) + 0.5);
    return u32(px[k] * 255.0 + 0.5);
}

// pairOf is the bytes k and k+1 of the pixel (dx, dy) of b's block, low then high.
fn pairOf(b: vec2<f32>, dx: f32, dy: f32, k: i32) -> u32 {
    return byteOf(b, dx, dy, k) | (byteOf(b, dx, dy, k + 1) << 8u);
}

// rectOf is the sheet's rectangle of sprite id: left, top, right, bottom.
fn rectOf(id: u32) -> vec4<f32> {
    let at = imageSrc1Origin() + vec2<f32>(f32((id % 256u) * 2u), f32(id / 256u)) + 0.5;
    let a = imageSrc1At(at) * 255.0 + 0.5;
    let b = imageSrc1At(at + vec2<f32>(1.0, 0.0)) * 255.0 + 0.5;
    return vec4<f32>(floor(a.x) + floor(a.y) * 256.0, floor(a.z) + floor(a.w) * 256.0,
        floor(b.x) + floor(b.y) * 256.0, floor(b.z) + floor(b.w) * 256.0);
}

// spriteAt is sprite id laid over the box from lo to hi, at the point p: the texel a quad laid
// over the box samples there, half a texel in from the rectangle's edges, held to the box.
fn spriteAt(id: u32, lo: vec2<f32>, hi: vec2<f32>, p: vec2<f32>) -> vec4<f32> {
    let r = rectOf(id);
    let t = clamp((p - lo) / (hi - lo), vec2<f32>(0.0), vec2<f32>(1.0));
    let src = r.xy + 0.5 + t * (r.zw - r.xy - 1.0);
    return imageSrc0At(src);
}

// leanOf is how far the tile of the board's cell b, its middle at mid, leans in the wind now.
fn leanOf(b: vec2<f32>, mid: vec2<f32>) -> vec2<f32> {
    if U.TileSways < 0.5 || (byteOf(b, 0.0, 0.0, 3) & flagSways) == 0u {
        return vec2<f32>(0.0);
    }
    let blow = length(U.Wind);
    let amount = f32(byteOf(b, 0.0, 1.0, 3)) / 255.0;
    if blow <= 0.0 || amount <= 0.0 {
        return vec2<f32>(0.0);
    }
    let w = U.Wind / blow;
    let strength = min(blow / 40.0, 1.5);
    let gust = dot(mid, w) / 60.0;
    let rock = sin(U.Clock * 2.2 * (0.7 + 0.3 * strength) - gust);
    let lean = amount * 0.35 * strength * (0.5 + 0.5 * rock);
    let rise = f32(pairOf(b, 1.0, 1.0, 0)) / 64.0;
    return w * lean * rise;
}

// tileAt is what the tile of the cell c lays at p, its covers over its sprite; nothing where its
// box, leant with the wind, does not reach or where its material draws it.
fn tileAt(c: vec2<f32>, p: vec2<f32>) -> vec4<f32> {
    let b = folded(c);
    if b.z < 0.5 {
        return vec4<f32>(0.0);
    }
    let flags = byteOf(b.xy, 0.0, 0.0, 3);
    if (flags & flagShaded) != 0u {
        return vec4<f32>(0.0);
    }
    let base = centreOf(c);
    let mid = base + leanOf(b.xy, base);
    let half = U.TileBox * 0.5;
    let lo = mid - half;
    let hi = mid + half;
    if p.x < lo.x || p.y < lo.y || p.x >= hi.x || p.y >= hi.y {
        return vec4<f32>(0.0);
    }
    var laid = spriteAt(pairOf(b.xy, 0.0, 0.0, 0), lo, hi, p);
    if U.TileCoverCount > 0.5 {
        let t = (p - lo) / (hi - lo);
        for (var i = 0; i < 8; i++) {
            if f32(i) >= U.TileCoverCount {
                break;
            }
            laid = over(coverAt(c, b.xy, u32(i), t, lo, hi, p), laid);
        }
    }
    return laid;
}

// coveredAt reports whether the i-th cover lies on the board's cell b.
fn coveredAt(b: vec2<f32>, i: u32) -> bool {
    return ((byteOf(b, 1.0, 0.0, 3) >> i) & 1u) != 0u;
}

// coverAt is the i-th cover over the tile of the cell c, the board's b, at t across its box lo
// to hi: weighed at its corners, side middles and middle by the cells round meeting there that
// it lies on too (draw.Weigh), shown where the weight over the quarter of the tile t lies in
// comes over a half.
fn coverAt(c: vec2<f32>, b: vec2<f32>, i: u32, t: vec2<f32>, lo: vec2<f32>, hi: vec2<f32>,
    p: vec2<f32>) -> vec4<f32> {
    let mine = coveredAt(b, i);
    var inside: array<f32, 9>;
    var any = false;
    for (var k = 0; k < 9; k++) {
        var on = mine;
        if U.TileHex < 0.5 && k != 4 {
            let n = folded(c + vec2<f32>(f32(k % 3) - 1.0, f32(k / 3) - 1.0));
            if n.z > 0.5 {
                on = coveredAt(n.xy, i);
            }
        }
        inside[k] = select(0.0, 1.0, on);
        any = any || on;
    }
    if !any {
        return vec4<f32>(0.0);
    }
    // the weights at the quarter's corners, of the points 0, 1, 2 across and down the tile
    let q = vec2<f32>(select(0.0, 1.0, t.x >= 0.5), select(0.0, 1.0, t.y >= 0.5));
    let w00 = weightAt(inside, q.x, q.y);
    let w10 = weightAt(inside, q.x + 1.0, q.y);
    let w01 = weightAt(inside, q.x, q.y + 1.0);
    let w11 = weightAt(inside, q.x + 1.0, q.y + 1.0);
    // across the quarter's two triangles, meeting from its top-right to its bottom-left corner
    let uv = (t - q * 0.5) / 0.5;
    var weight = w00 + (w10 - w00) * uv.x + (w01 - w00) * uv.y;
    if uv.x + uv.y > 1.0 {
        weight = w11 + (w01 - w11) * (1.0 - uv.x) + (w10 - w11) * (1.0 - uv.y);
    }
    let a = smoothstep(0.5 - coverSoft, 0.5 + coverSoft, weight);
    var id = 0.0;
    if i < 4u {
        id = U.TileCovers[i];
    } else {
        id = U.TileCoversOn[i - 4u];
    }
    let s = spriteAt(u32(id + 0.5), lo, hi, p);
    return s * a;
}

// weightAt is the share of the cells round a tile a cover lies on at its point (x, y), 0 its left
// or top edge, 1 its middle, 2 its right or bottom edge: of those meeting there.
fn weightAt(inside: array<f32, 9>, x: f32, y: f32) -> f32 {
    let x0 = select(1, 0, x < 0.5);
    let x1 = select(1, 2, x > 1.5);
    let y0 = select(1, 0, y < 0.5);
    let y1 = select(1, 2, y > 1.5);
    var n = 0.0;
    var all = 0.0;
    for (var cy = y0; cy <= y1; cy++) {
        for (var cx = x0; cx <= x1; cx++) {
            n += inside[cy * 3 + cx];
            all += 1.0;
        }
    }
    return n / all;
}

// stepOf is the step from a cell's middle to its neighbour in the grid's direction i, and that
// neighbour's offset: a square board's eight (north, south, west, east, then north-west,
// north-east, south-west, south-east), a hex board's six (east, north-east, north-west, west,
// south-west, south-east); z 0 past them.
fn stepOf(i: i32) -> vec4<f32> {
    if U.TileHex > 0.5 {
        let dirs = array<vec2<f32>, 6>(vec2<f32>(1.0, 0.0), vec2<f32>(1.0, -1.0),
            vec2<f32>(0.0, -1.0), vec2<f32>(-1.0, 0.0), vec2<f32>(-1.0, 1.0),
            vec2<f32>(0.0, 1.0));
        if i >= 6 {
            return vec4<f32>(0.0);
        }
        let d = dirs[i];
        let s = U.TileSide;
        return vec4<f32>(s * (sqrt(3.0) * d.x + sqrt(3.0) / 2.0 * d.y), s * 1.5 * d.y, d);
    }
    let dirs = array<vec2<f32>, 8>(vec2<f32>(0.0, -1.0), vec2<f32>(0.0, 1.0),
        vec2<f32>(-1.0, 0.0), vec2<f32>(1.0, 0.0), vec2<f32>(-1.0, -1.0),
        vec2<f32>(1.0, -1.0), vec2<f32>(-1.0, 1.0), vec2<f32>(1.0, 1.0));
    let d = dirs[i];
    return vec4<f32>(d * U.TileBox, d);
}

// waysOf is laid with the way across the cell c over it at p, if any reaches there: a band out
// towards every neighbour its links name that lies on the board, then the hub, each laid as it
// comes.
fn waysOf(c: vec2<f32>, p: vec2<f32>, laid: vec4<f32>) -> vec4<f32> {
    let b = folded(c);
    if b.z < 0.5 || (byteOf(b.xy, 0.0, 0.0, 3) & flagWay) == 0u {
        return laid;
    }
    let mid = centreOf(c);
    let half = f32(pairOf(b.xy, 0.0, 1.0, 0)) / 64.0 * 0.5;
    let d = p - mid;
    if dot(d, d) > dot(U.TileBox, U.TileBox) + half * half * 4.0 {
        return laid; // too far for any band of the cell
    }
    let keep = 1.0 - f32(byteOf(b.xy, 0.0, 1.0, 2)) / 255.0;
    let links = byteOf(b.xy, 1.0, 0.0, 2);
    let s = spriteAt(pairOf(b.xy, 1.0, 0.0, 0), mid - U.TileBox * 0.5, mid + U.TileBox * 0.5, p) *
        keep;
    var out = laid;
    let dirs = select(8, 6, U.TileHex > 0.5);
    for (var i = 0; i < 8; i++) {
        if i >= dirs || ((links >> u32(i)) & 1u) == 0u {
            continue;
        }
        let st = stepOf(i);
        if folded(c + st.zw).z < 0.5 {
            continue;
        }
        let l = length(st.xy);
        if l <= 0.0 {
            continue;
        }
        let along = dot(d, st.xy) / l;
        let across = abs(dot(d, vec2<f32>(-st.y, st.x)) / l);
        if along >= 0.0 && along <= l * 0.5 && across <= half {
            out = over(s, out);
        }
    }
    // the hub: an octagon of the band's width round the middle, its corners at the eighths
    var inHub = true;
    for (var k = 0; k < 8; k++) {
        let a = (f32(k) + 0.5) * 0.7853982;
        if dot(d, vec2<f32>(cos(a), sin(a))) > half * 0.9238795 {
            inHub = false;
        }
    }
    if inHub {
        out = over(s, out);
    }
    return out;
}

// materialAt is the material of the cell c over its box at p, nothing where c is drawn by its
// sprite or its box does not reach.
fn materialAt(c: vec2<f32>, p: vec2<f32>) -> vec4<f32> {
    let b = folded(c);
    if b.z < 0.5 || (byteOf(b.xy, 0.0, 0.0, 3) & flagShaded) == 0u {
        return vec4<f32>(0.0);
    }
    let mid = centreOf(c);
    let half = U.TileBox * 0.5;
    let lo = mid - half;
    let hi = mid + half;
    if p.x < lo.x || p.y < lo.y || p.x >= hi.x || p.y >= hi.y {
        return vec4<f32>(0.0);
    }
    let k = f32(byteOf(b.xy, 1.0, 1.0, 2));
    return material(k, p, 1.0, 0.0, vec4<f32>(mid, half.x, 0.0));
}

// lightAt is the light on the ground at p in the cell c: the ambient light, the distant light on
// level ground less the shadows the frame holds and the lamps', as the cell takes them; white
// without a light, or where the cell takes none.
fn lightAt(c: vec2<f32>, p: vec2<f32>) -> vec3<f32> {
    if U.TileLit < 0.5 {
        return vec3<f32>(1.0);
    }
    let b = folded(c);
    var taken = f32(takesRelief | takesBlocks | takesClouds | takesLamps);
    if b.z > 0.5 {
        taken = f32(byteOf(b.xy, 0.0, 0.0, 2));
    }
    if u32(taken + 0.5) == 0u {
        return vec3<f32>(1.0);
    }
    let way = lightWay();
    var lit = 1.0;
    if takes(taken, takesRelief) || takes(taken, takesBlocks) {
        lit = groundShadeOf(p).r;
    }
    var light = U.Ambience + U.LightColor * max(way.z, 0.0) * U.LightStrength * lit;
    if takes(taken, takesLamps) {
        light += lampLightAt(vec3<f32>(p, 0.0));
    }
    return min(light, vec3<f32>(1.0)); // as a shade laid over the ground scales it, at most whole
}
