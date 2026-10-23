// The covers on a flat board's cells (look's tiles.go), an instance a cell and a cover lying on it
// or a neighbour (its column, its row and which cover): the cover over each quarter of the cell's
// tile, leant with it, weighed at the quarter's corners by the share of the cells meeting there it
// lies on (draw.Weigh; on a hex board the cell's own alone), shown where the weight comes over a
// half, fading in over coverSoft of it (draw.Quarters); laid over the tiles, U.TileShift along,
// through U.TileViewProj; none on a cell of a sector seen but coarsely.

struct Part {
    @builtin(position) clip: vec4<f32>,
    @location(0) world: vec2<f32>,
    @location(1) uv: vec2<f32>,
    @location(2) weight: f32,
    @location(3) @interpolate(flat) look: vec4<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vid: u32, @location(0) at: vec4<f32>) -> Part {
    var out: Part;
    out.clip = away;
    let b = at.xy;
    let i = u32(at.z + 0.5);
    if levelOf(b) != 1u {
        return out;
    }
    // the quarter, and its corner: two triangles of top-left, top-right, bottom-left, then
    // top-right, bottom-left, bottom-right
    let q = vid / 6u;
    var k = vid % 6u;
    if k >= 3u {
        k -= 2u;
    }
    let point = vec2<f32>(f32(q % 2u + k % 2u), f32(q / 2u + k / 2u)); // 0, 1, 2 across, down
    out.weight = weightAt(b, i, point);
    let base = centreOf(b);
    var mid = base;
    if (byteOf(b, 0.0, 0.0, 3) & flagSways) != 0u {
        mid += leanOf(b, base);
    }
    let lo = mid + U.TileShift - U.TileBox * 0.5;
    out.uv = point * 0.5;
    out.world = lo + out.uv * U.TileBox;
    out.look = rectOf(coverOf(i));
    out.clip = U.TileViewProj * vec4<f32>(out.world, 0.0, 1.0);
    return out;
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

@fragment
fn fs_main(in: Part) -> @location(0) vec4<f32> {
    let a = smoothstep(0.5 - coverSoft, 0.5 + coverSoft, in.weight);
    return litAt(sampled(in.look, in.uv) * a, in.world);
}
