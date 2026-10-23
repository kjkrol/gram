// The board painted flat (painter's bake_gpu.go), an instance a cell to paint anew (its column and
// row), U.PaintPx pixels a cell from the target's top-left, the target U.PaintTarget large: the
// sprite its top is drawn in (base) over the whole cell, then the grounds round it running in
// (blendsAnew): over each quarter of the cell a ground's whole sprite, shown where the share of
// the cells meeting at the quarter's corners it lies on comes over a half, fading in over its
// spread.

struct Cell {
    @builtin(position) clip: vec4<f32>,
    @location(0) @interpolate(flat) at: vec2<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vid: u32, @location(0) at: vec4<f32>) -> Cell {
    // two triangles: top-left, top-right, bottom-left, then top-right, bottom-left, bottom-right
    var k = vid % 6u;
    if k >= 3u {
        k -= 2u;
    }
    let p = (at.xy + vec2<f32>(f32(k % 2u), f32(k / 2u))) * U.PaintPx;
    var out: Cell;
    out.clip = vec4<f32>(p.x / U.PaintTarget.x * 2.0 - 1.0, 1.0 - p.y / U.PaintTarget.y * 2.0,
        0.0, 1.0);
    out.at = at.xy;
    return out;
}

@fragment
fn fs_main(in: Cell) -> @location(0) vec4<f32> {
    let uv = (in.clip.xy - in.at * U.PaintPx) / U.PaintPx;
    var near: array<Top, 9>;
    let mine = topOf(in.at);
    for (var k = 0; k < 9; k++) {
        let c = folded(in.at + vec2<f32>(f32(k % 3) - 1.0, f32(k / 3) - 1.0));
        near[k] = mine;
        if c.z > 0.5 {
            near[k] = topOf(c.xy);
        }
    }
    // the sprite the top is drawn in: its own, or that of the first neighbour it runs into that
    // lies under it
    var base = mine;
    var based = false;
    if land(mine) {
        for (var i = 0; i < 8; i++) {
            let n = folded(in.at + stepOf(i));
            if n.z > 0.5 && !based {
                let t = topOf(n.xy);
                if (t.flags & flagUnder) != 0u {
                    base = t;
                    based = true;
                }
            }
        }
    }
    var laid = sampled(base.sprite, uv);
    if (mine.flags & flagRaised) != 0u || ((mine.flags & flagUnder) == 0u && mine.spread <= 0.0) {
        return laid;
    }
    // the ground on top: its own, or the land round it most of the cells hold, row by row the
    // first to come to that many
    var top = 4;
    if (mine.flags & flagUnder) != 0u {
        top = -1;
        var most = 0;
        var reached = 9;
        for (var k = 0; k < 9; k++) {
            if !land(near[k]) || earlier(near, k) {
                continue;
            }
            var n = 0;
            var last = k;
            for (var j = k; j < 9; j++) {
                if land(near[j]) && near[j].sprite == near[k].sprite {
                    n += 1;
                    last = j;
                }
            }
            if n > most || (n == most && last < reached) {
                top = k;
                most = n;
                reached = last;
            }
        }
        if top < 0 {
            return laid;
        }
    }
    let q = min(floor(uv * 2.0), vec2<f32>(1.0));
    let s = uv * 2.0 - q;
    let t = near[top];
    if (mine.flags & flagUnder) != 0u || based {
        laid = over(quarterOf(t.sprite, weighed(near, q, t.sprite, true), s, t.spread), laid);
    }
    for (var k = 0; k < 9; k++) {
        let o = near[k];
        if !land(o) || o.sprite == t.sprite {
            continue;
        }
        if earlier(near, k) {
            continue;
        }
        let w = weighed(near, q, o.sprite, false);
        laid = over(quarterOf(o.sprite, w, s, (t.spread + o.spread) / 2.0), laid);
    }
    return laid;
}

// earlier reports whether a cell of land before the k-th round, row by row, is of its sprite.
fn earlier(near: array<Top, 9>, k: i32) -> bool {
    for (var j = 0; j < k; j++) {
        if land(near[j]) && near[j].sprite == near[k].sprite {
            return true;
        }
    }
    return false;
}

// quarterOf is sprite id over a quarter, at s across it, weighed w at its corners and fading in over
// soft: nothing where no corner comes within soft of a half, as no piece is laid there.
fn quarterOf(id: u32, w: vec4<f32>, s: vec2<f32>, soft: f32) -> vec4<f32> {
    if max(max(w.x, w.y), max(w.z, w.w)) <= 0.5 - soft {
        return vec4<f32>(0.0);
    }
    return sampled(id, s) * shows(blendAt(w, s), soft);
}

// weighed is the share of the 3×3 cells round the corners of the quarter q lying not under the
// rest (dry) or of the sprite id: at each corner the cells meeting there (weigh).
fn weighed(near: array<Top, 9>, q: vec2<f32>, id: u32, dry: bool) -> vec4<f32> {
    var w: vec4<f32>;
    for (var corner = 0; corner < 4; corner++) {
        let point = vec2<i32>(i32(q.x) + corner % 2, i32(q.y) + corner / 2);
        // the cells a point spans: the edge's two, the middle's one
        let lo = vec2<i32>(select(1, 0, point.x == 0), select(1, 0, point.y == 0));
        let hi = vec2<i32>(select(1, 2, point.x == 2), select(1, 2, point.y == 2));
        var n = 0.0;
        var all = 0.0;
        for (var y = lo.y; y <= hi.y; y++) {
            for (var x = lo.x; x <= hi.x; x++) {
                let c = near[y * 3 + x];
                var on = c.sprite == id;
                if dry {
                    on = (c.flags & flagUnder) == 0u;
                }
                n += select(0.0, 1.0, on);
                all += 1.0;
            }
        }
        w[corner] = n / all;
    }
    return w;
}

// stepOf is the step to the neighbour in a square grid's direction i: north, south, west, east,
// then north-west, north-east, south-west, south-east.
fn stepOf(i: i32) -> vec2<f32> {
    var d = vec2<f32>(select(-1.0, 1.0, i == 5 || i == 7), select(-1.0, 1.0, i >= 6));
    if i < 2 {
        d = vec2<f32>(0.0, select(-1.0, 1.0, i == 1));
    } else if i < 4 {
        d = vec2<f32>(select(-1.0, 1.0, i == 3), 0.0);
    }
    return d;
}
