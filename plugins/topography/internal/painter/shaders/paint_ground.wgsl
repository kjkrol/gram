// The board painted flat (painter's bake_gpu.go), an instance a cell to paint anew (its column and
// row), U.PaintPx pixels a cell from U.PaintOrigin in a target U.PaintTarget large, the layer
// U.PaintLayer says: the sprite the cell's top is drawn in (its own or that of a cell under it
// it runs into) over the whole cell, then the grounds round it running in — over each quarter
// of the cell a ground's whole sprite, shown where the share of the cells meeting at the
// quarter's corners it lies on comes over a half, fading in over its spread. The kinds' layer
// lays each one's detail instead, edged hard; the flow's and the shine's lay the water of a top
// that shines, running or still, and the grounds running in darken it.

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
    let p = (at.xy + vec2<f32>(f32(k % 2u), f32(k / 2u))) * U.PaintPx + U.PaintOrigin;
    var out: Cell;
    out.clip = vec4<f32>(p.x / U.PaintTarget.x * 2.0 - 1.0, 1.0 - p.y / U.PaintTarget.y * 2.0,
        0.0, 1.0);
    out.at = at.xy;
    return out;
}

@fragment
fn fs_main(in: Cell) -> @location(0) vec4<f32> {
    let uv = (in.clip.xy - U.PaintOrigin - in.at * U.PaintPx) / U.PaintPx;
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
    var laid = baseOf(base, in.at, uv);
    if (mine.flags & flagRaised) != 0u ||
        ((mine.flags & flagUnder) == 0u && mine.spread <= 0.0) {
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
        laid = over(quarterOf(t, weighed(near, q, t.sprite, true), s, t.spread), laid);
    }
    for (var k = 0; k < 9; k++) {
        let o = near[k];
        if !land(o) || o.sprite == t.sprite || earlier(near, k) {
            continue;
        }
        let w = weighed(near, q, o.sprite, false);
        laid = over(quarterOf(o, w, s, (t.spread + o.spread) / 2.0), laid);
    }
    return laid;
}

// baseOf is the layer's own of a cell at at whose top is drawn in base, at uv across it: the
// sprite, its detail, or the water lying on it where base shines — running where its kind flows,
// as fast at the cell's corners as the slopes of the cells meeting there run down, else still.
fn baseOf(base: Top, at: vec2<f32>, uv: vec2<f32>) -> vec4<f32> {
    let l = layer();
    if l == layerAlbedo {
        return sampled(base.sprite, uv);
    }
    if l == layerKinds {
        return vec4<f32>(f32(base.detail) / 255.0, 0.0, 0.0, 1.0);
    }
    if base.shine <= 0.0 {
        return vec4<f32>(0.0);
    }
    if base.flow <= 0.0 {
        if l == layerShine {
            return vec4<f32>(0.0, base.shine, 1.0, 1.0);
        }
        return vec4<f32>(0.0);
    }
    if l == layerShine {
        return vec4<f32>(base.shine, 0.0, 0.0, 1.0);
    }
    var fx: vec4<f32>;
    var fy: vec4<f32>;
    for (var corner = 0; corner < 4; corner++) {
        let d = at + vec2<f32>(f32(corner % 2), f32(corner / 2));
        var sum = vec2<f32>(0.0);
        var n = 0.0;
        for (var o = 0; o < 4; o++) {
            let c = folded(d + vec2<f32>(f32(o % 2) - 1.0, f32(o / 2) - 1.0));
            if c.z < 0.5 {
                continue;
            }
            let t = topOf(c.xy);
            if t.flow <= 0.0 {
                continue;
            }
            sum += runOf(groundOf(c.xy), t.flow);
            n += 1.0;
        }
        if n > 0.0 {
            sum = sum / n;
        }
        fx[corner] = flowOf(sum.x);
        fy[corner] = flowOf(sum.y);
    }
    return vec4<f32>(blendAt(fx, uv), blendAt(fy, uv), 1.0, 1.0);
}

// runOf is how fast water runs down a cell of ground g at its corners, flow fast down a slope
// of 1 in 1: down the slope as the corners tilt it.
fn runOf(g: vec4<f32>, flow: f32) -> vec2<f32> {
    let gx = (g.y - g.x + g.w - g.z) / (2.0 * U.PaintCell);
    let gy = (g.z - g.x + g.w - g.y) / (2.0 * U.PaintCell);
    let slope = length(vec2<f32>(gx, gy));
    if slope == 0.0 {
        return vec2<f32>(0.0);
    }
    let speed = flow * sqrt(slope);
    return -vec2<f32>(gx, gy) / slope * speed;
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

// quarterOf is the layer's own of the ground t over a quarter, at s across it, weighed w at its
// corners and fading in over soft: its sprite, its detail edged hard, or the dark it lays over
// the water; nothing where no corner comes within soft of a half, as no piece is laid there.
fn quarterOf(t: Top, w: vec4<f32>, s: vec2<f32>, soft: f32) -> vec4<f32> {
    if max(max(w.x, w.y), max(w.z, w.w)) <= 0.5 - soft {
        return vec4<f32>(0.0);
    }
    let l = layer();
    if l == layerAlbedo {
        return sampled(t.sprite, s) * shows(blendAt(w, s), soft);
    }
    if l == layerKinds {
        let a = shows(blendAt(w, s), 0.0);
        return vec4<f32>(f32(t.detail) / 255.0 * a, 0.0, 0.0, a);
    }
    return vec4<f32>(0.0, 0.0, 0.0, shows(blendAt(w, s), soft));
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
