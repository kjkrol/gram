// What the passes laying a flat board's tiles on the GPU (look's tiles.go) share. A cell's block
// in the cells' image (image 2, 2×2 pixels a cell) says what it shows: its sprite, its way, the
// covers on it, how it sways, its material and its flags; the sprites are the sheet's (image 0),
// found through their rectangles (image 1, two pixels a sprite, 256 a row: left and top, then
// right and bottom, a coordinate in two bytes). A square board's cells are U.TileBox across; a hex
// board's (U.TileHex) are pointy-top, U.TileSide from centre to corner, their axial q and r its
// columns and rows. Along an axis U.TileWrap says wraps the cells run on. Where U.TileSeen, the
// world's sectors are seen at their levels (image 3, a pixel a sector, red its view.Level;
// U.TileSectors.xy of them across and down, U.TileSectors.zw large). What is drawn is drawn in
// U.TileTint and, where U.TileLit, in the light on level ground as the cell under it takes it.

// a cell's flags (look's tiles.go)
const flagShaded: u32 = 1u;
const flagSways: u32 = 4u;
const flagNearCover: u32 = 8u;
const flagNearSway: u32 = 16u;
const flagOverlaid: u32 = 32u;

// the share of a cover's weight it fades in over (draw.coverSoft)
const coverSoft: f32 = 0.2;

// near is the k-th cell round c in the order the cells are laid, row by row: of a square board
// the 3×3 round c, of a hex board c and its six neighbours (k under 7); z 0 past them.
fn near(c: vec2<f32>, k: i32) -> vec3<f32> {
    if U.TileHex > 0.5 {
        if k >= 7 {
            return vec3<f32>(0.0);
        }
        var d = vec2<f32>(0.0, 1.0);
        if k == 0 {
            d = vec2<f32>(0.0, -1.0);
        } else if k == 1 {
            d = vec2<f32>(1.0, -1.0);
        } else if k == 2 {
            d = vec2<f32>(-1.0, 0.0);
        } else if k == 3 {
            d = vec2<f32>(0.0, 0.0);
        } else if k == 4 {
            d = vec2<f32>(1.0, 0.0);
        } else if k == 5 {
            d = vec2<f32>(-1.0, 1.0);
        }
        return vec3<f32>(c + d, 1.0);
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

// texelOf is the pixel (dx, dy) of the board's cell b's block.
fn texelOf(b: vec2<f32>, dx: f32, dy: f32) -> vec4<f32> {
    return imageSrc2At(imageSrc2Origin() + b * 2.0 + vec2<f32>(dx, dy) + 0.5);
}

// byteIn is the byte k, 0 to 3, of the pixel t.
fn byteIn(t: vec4<f32>, k: i32) -> u32 {
    return u32(t[k] * 255.0 + 0.5);
}

// byteOf is the byte k, 0 to 3, of the pixel (dx, dy) of the board's cell b's block.
fn byteOf(b: vec2<f32>, dx: f32, dy: f32, k: i32) -> u32 {
    return byteIn(texelOf(b, dx, dy), k);
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
    if U.TileSways < 0.5 {
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

// stepOf is the step from a cell's middle to its neighbour in the grid's direction i: a square
// board's eight (north, south, west, east, then north-west, north-east, south-west, south-east), a
// hex board's six (east, north-east, north-west, west, south-west, south-east).
fn stepOf(i: i32) -> vec2<f32> {
    if U.TileHex > 0.5 {
        var d = vec2<f32>(0.0, 1.0);
        if i == 0 {
            d = vec2<f32>(1.0, 0.0);
        } else if i == 1 {
            d = vec2<f32>(1.0, -1.0);
        } else if i == 2 {
            d = vec2<f32>(0.0, -1.0);
        } else if i == 3 {
            d = vec2<f32>(-1.0, 0.0);
        } else if i == 4 {
            d = vec2<f32>(-1.0, 1.0);
        }
        let s = U.TileSide;
        return vec2<f32>(s * (sqrt(3.0) * d.x + sqrt(3.0) / 2.0 * d.y), s * 1.5 * d.y);
    }
    var d = vec2<f32>(select(-1.0, 1.0, i == 5 || i == 7), select(-1.0, 1.0, i >= 6));
    if i < 2 {
        d = vec2<f32>(0.0, select(-1.0, 1.0, i == 1));
    } else if i < 4 {
        d = vec2<f32>(select(-1.0, 1.0, i == 3), 0.0);
    }
    return d * U.TileBox;
}

// levelOf is the level the sector the middle of the board's cell b lies in is seen at
// (view.Level): Exact, 1, without sectors.
fn levelOf(b: vec2<f32>) -> u32 {
    if U.TileSeen < 0.5 {
        return 1u;
    }
    let s = clamp(floor(centreOf(b) / U.TileSectors.zw), vec2<f32>(0.0), U.TileSectors.xy - 1.0);
    return u32(imageSrc3At(imageSrc3Origin() + s + 0.5).r * 255.0 + 0.5);
}

// fogOf is how far into the fog before the sight's end the middle of the board's cell b lies, 0
// short of it to 1 at the end.
fn fogOf(b: vec2<f32>) -> f32 {
    let f = U.TileFogging;
    if U.TileSeen < 0.5 || f.w <= f.z {
        return 0.0;
    }
    return smoothstep(f.z, f.w, distance(centreOf(b), f.xy));
}

// lightAt is the light on the ground at p in a cell taking taken: the ambient light, the distant
// light on level ground less the shadows the frame holds and the lamps', as the cell takes them;
// white without a light, or where the cell takes none.
fn lightAt(taken: f32, p: vec2<f32>) -> vec3<f32> {
    if U.TileLit < 0.5 {
        return vec3<f32>(1.0);
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

// cellAt is the cell of the board the point p lies in, as far along as p is.
fn cellAt(p: vec2<f32>) -> vec2<f32> {
    if U.TileHex > 0.5 {
        return hexAt(p);
    }
    return floor(p / U.TileBox);
}

// takenAt is the shade the cell p lies in takes (render.Taking), every bit off the board.
fn takenAt(p: vec2<f32>) -> f32 {
    let b = folded(cellAt(p));
    if b.z < 0.5 {
        return f32(takesRelief | takesBlocks | takesClouds | takesLamps);
    }
    return f32(byteOf(b.xy, 0.0, 0.0, 2));
}

// litAt is c, premultiplied, at p in U.TileTint and the light on the ground there.
fn litAt(c: vec4<f32>, p: vec2<f32>) -> vec4<f32> {
    return vec4<f32>(c.rgb * U.TileTint * lightAt(takenAt(p), p), c.a);
}

// sampled is the texel of the sheet's rectangle r a quad laid over a sprite samples at uv across
// it: half a texel in from the rectangle's edges.
fn sampled(r: vec4<f32>, uv: vec2<f32>) -> vec4<f32> {
    return imageSrc0At(r.xy + 0.5 + uv * (r.zw - r.xy - 1.0));
}

// plainly reports whether the plain pass lays the cell flagged so: drawn by its sprite, no
// material over it, nothing swaying on or beside it where the tiles sway.
fn plainly(flags: u32) -> bool {
    var mask = flagShaded | flagOverlaid;
    if U.TileSways > 0.5 {
        mask |= flagNearSway;
    }
    return (flags & mask) == 0u;
}

// overlayOf is the material laid over the tile of the board's cell b, flagged flags, its box from
// lo to hi, at p, in U.TileTint (render.Overlay: World the box, Red the block's, Fraction 1);
// nothing where none is or the box does not reach.
fn overlayOf(b: vec2<f32>, flags: u32, lo: vec2<f32>, hi: vec2<f32>, p: vec2<f32>) -> vec4<f32> {
    if (flags & flagOverlaid) == 0u || p.x < lo.x || p.y < lo.y || p.x >= hi.x || p.y >= hi.y {
        return vec4<f32>(0.0);
    }
    let t = texelOf(b, 1.0, 1.0);
    let c = material(f32(byteIn(t, 2)), p, f32(byteIn(t, 3)) / 255.0, 1.0, vec4<f32>(0.0));
    return vec4<f32>(c.rgb * U.TileTint, c.a);
}

// coversAt is what the covers lying on the board's cell b or round it, flagged flags, lay at p
// over its tile, the box from lo U.TileBox large, in the order they come: each over a quarter of
// the box, weighed at the quarter's corners (weightAt) and between them as two triangles from the
// top-left and the bottom-right corner are, as the instanced pass lays them.
fn coversAt(b: vec2<f32>, flags: u32, lo: vec2<f32>, p: vec2<f32>) -> vec4<f32> {
    let uv = (p - lo) / U.TileBox;
    if (flags & flagNearCover) == 0u || uv.x < 0.0 || uv.y < 0.0 || uv.x >= 1.0 || uv.y >= 1.0 {
        return vec4<f32>(0.0);
    }
    var bits = byteOf(b, 1.0, 0.0, 3);
    if U.TileHex < 0.5 {
        for (var k = 0; k < 9; k++) {
            let c = folded(near(b, k).xy);
            if c.z > 0.5 {
                bits |= byteOf(c.xy, 1.0, 0.0, 3);
            }
        }
    }
    let g = uv * 2.0;
    let q = min(floor(g), vec2<f32>(1.0));
    let s = g - q;
    var laid = vec4<f32>(0.0);
    for (var i = 0u; i < 8u; i++) {
        if ((bits >> i) & 1u) == 0u {
            continue;
        }
        var w: f32;
        if s.x + s.y <= 1.0 {
            let tl = weightAt(b, i, q);
            w = tl + s.x * (weightAt(b, i, q + vec2<f32>(1.0, 0.0)) - tl) +
                s.y * (weightAt(b, i, q + vec2<f32>(0.0, 1.0)) - tl);
        } else {
            let br = weightAt(b, i, q + 1.0);
            w = br + (1.0 - s.x) * (weightAt(b, i, q + vec2<f32>(0.0, 1.0)) - br) +
                (1.0 - s.y) * (weightAt(b, i, q + vec2<f32>(1.0, 0.0)) - br);
        }
        let a = smoothstep(0.5 - coverSoft, 0.5 + coverSoft, w);
        laid = over(sampled(rectOf(coverOf(i)), uv) * a, laid);
    }
    return laid;
}

// weightAt is the share of the cells meeting at the point (x, y) of the board's cell b's tile (0
// its left or top edge, 1 its middle, 2 its right or bottom edge) the i-th cover lies on: of a hex
// board's, b's own; past the board's edge, b's own in place of the cell there.
fn weightAt(b: vec2<f32>, i: u32, point: vec2<f32>) -> f32 {
    let mine = select(0.0, 1.0, coveredAt(b, i));
    if U.TileHex > 0.5 {
        return mine;
    }
    let x0 = select(0.0, -1.0, point.x < 0.5);
    let x1 = select(0.0, 1.0, point.x > 1.5);
    let y0 = select(0.0, -1.0, point.y < 0.5);
    let y1 = select(0.0, 1.0, point.y > 1.5);
    var n = 0.0;
    var all = 0.0;
    for (var dy = y0; dy <= y1; dy += 1.0) {
        for (var dx = x0; dx <= x1; dx += 1.0) {
            var on = mine;
            let c = folded(b + vec2<f32>(dx, dy));
            if c.z > 0.5 {
                on = select(0.0, 1.0, coveredAt(c.xy, i));
            }
            n += on;
            all += 1.0;
        }
    }
    return n / all;
}

// coveredAt reports whether the i-th cover lies on the board's cell b.
fn coveredAt(b: vec2<f32>, i: u32) -> bool {
    return ((byteOf(b, 1.0, 0.0, 3) >> i) & 1u) != 0u;
}

// coverOf is the sprite of the i-th cover.
fn coverOf(i: u32) -> u32 {
    var id = U.TileCoversOn[(i + 4u) % 4u];
    if i < 4u {
        id = U.TileCovers[i];
    }
    return u32(id + 0.5);
}

// away is a clip position no triangle laid there is seen at.
const away = vec4<f32>(2.0, 2.0, 0.0, 1.0);
