// The board painted flat (painter's bake_gpu.go), an instance a cell to paint anew (its column and
// row), U.PaintPx pixels a cell from U.PaintOrigin in a target U.PaintTarget large, the layer
// U.PaintLayer says: the sprite the cell's top is drawn in (its own or that of a cell under it
// it runs into) over the whole cell, then the grounds round it running in — over each quarter
// of the cell a ground's whole sprite, shown where the share of the cells meeting at the
// quarter's corners it lies on comes over a half, fading in over its spread. The kinds' layer
// lays each one's detail instead, edged hard; the flow's and the shine's lay the water of a top
// that shines, running or still, and the grounds running in darken it. What holds for the whole
// cell is worked out at its corners, its pixels left the quarters' weights and the sprites.

struct Cell {
    @builtin(position) clip: vec4<f32>,
    @location(0) @interpolate(flat) at: vec2<f32>,
    // the 3×3 cells round it, row by row, as their blocks' words hold them (wordsOf); a cell off
    // the board as the cell itself
    @location(1) @interpolate(flat) near0: vec4<u32>,
    @location(2) @interpolate(flat) near1: vec4<u32>,
    @location(3) @interpolate(flat) near2: vec4<u32>,
    @location(4) @interpolate(flat) near3: vec4<u32>,
    @location(5) @interpolate(flat) near4: vec2<u32>,
    // the round cell the top is drawn in, the ground on top (none past 8), the round cells running
    // in (a bit each) and whether the base lies alone (1) and the top over it (2)
    @location(6) @interpolate(flat) plan: vec4<u32>,
    // the water's flow at the cell's corners across and down, where the layer is the flow's
    @location(7) @interpolate(flat) fx: vec4<f32>,
    @location(8) @interpolate(flat) fy: vec4<f32>,
}

const planAlone: u32 = 1u;
const planTop: u32 = 2u;

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
    var words: array<vec2<u32>, 9>;
    let own = wordsOf(at.xy);
    for (var j = 0; j < 9; j++) {
        let c = folded(at.xy + vec2<f32>(f32(j % 3) - 1.0, f32(j / 3) - 1.0));
        words[j] = own;
        if c.z > 0.5 {
            words[j] = wordsOf(c.xy);
        }
    }
    out.near0 = vec4<u32>(words[0], words[1]);
    out.near1 = vec4<u32>(words[2], words[3]);
    out.near2 = vec4<u32>(words[4], words[5]);
    out.near3 = vec4<u32>(words[6], words[7]);
    out.near4 = words[8];
    var near: array<Top, 9>;
    for (var j = 0; j < 9; j++) {
        near[j] = topIn(words[j]);
    }
    let mine = near[4];
    // the sprite the top is drawn in: its own, or that of the first neighbour it runs into that
    // lies under it (one off the board is the cell itself, which does not)
    var base = 4;
    if land(mine) {
        for (var i = 0; i < 8; i++) {
            let d = stepOf(i);
            let n = i32(d.y + 1.0) * 3 + i32(d.x + 1.0);
            if base == 4 && (near[n].flags & flagUnder) != 0u {
                base = n;
            }
        }
    }
    out.plan = vec4<u32>(u32(base), 9u, 0u, 0u);
    if layer() == layerFlow && near[base].shine > 0.0 && near[base].flow > 0.0 {
        for (var corner = 0; corner < 4; corner++) {
            let d = at.xy + vec2<f32>(f32(corner % 2), f32(corner / 2));
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
            out.fx[corner] = flowOf(sum.x);
            out.fy[corner] = flowOf(sum.y);
        }
    }
    if (mine.flags & flagRaised) != 0u || ((mine.flags & flagUnder) == 0u && mine.spread <= 0.0) {
        out.plan.w = planAlone;
        return out;
    }
    // the ground on top: its own, or the land round it most of the cells hold, row by row the
    // first to come to that many
    var top = 4;
    if (mine.flags & flagUnder) != 0u {
        top = 9;
        var most = 0;
        var reached = 9;
        for (var j = 0; j < 9; j++) {
            if !land(near[j]) || earlier(near, j) {
                continue;
            }
            var n = 0;
            var last = j;
            for (var i = j; i < 9; i++) {
                if land(near[i]) && near[i].sprite == near[j].sprite {
                    n += 1;
                    last = i;
                }
            }
            if n > most || (n == most && last < reached) {
                top = j;
                most = n;
                reached = last;
            }
        }
        if top == 9 {
            return out;
        }
    }
    out.plan.y = u32(top);
    if (mine.flags & flagUnder) != 0u || base != 4 {
        out.plan.w = planTop;
    }
    let t = near[top];
    var running = 0u;
    for (var j = 0; j < 9; j++) {
        let o = near[j];
        if land(o) && o.sprite != t.sprite && !earlier(near, j) {
            running = running | (1u << u32(j));
        }
    }
    out.plan.z = running;
    return out;
}

@fragment
fn fs_main(in: Cell) -> @location(0) vec4<f32> {
    let uv = (in.clip.xy - U.PaintOrigin - in.at * U.PaintPx) / U.PaintPx;
    var words: array<vec2<u32>, 9>;
    words[0] = in.near0.xy;
    words[1] = in.near0.zw;
    words[2] = in.near1.xy;
    words[3] = in.near1.zw;
    words[4] = in.near2.xy;
    words[5] = in.near2.zw;
    words[6] = in.near3.xy;
    words[7] = in.near3.zw;
    words[8] = in.near4;
    var near: array<Top, 9>;
    for (var j = 0; j < 9; j++) {
        near[j] = topIn(words[j]);
    }
    var laid = baseOf(near[in.plan.x], uv, in.fx, in.fy);
    if (in.plan.w & planAlone) != 0u || in.plan.y > 8u {
        return laid;
    }
    let q = min(floor(uv * 2.0), vec2<f32>(1.0));
    let s = uv * 2.0 - q;
    let t = near[in.plan.y];
    if (in.plan.w & planTop) != 0u {
        laid = over(quarterOf(t, weighed(near, q, t.sprite, true), s, t.spread), laid);
    }
    for (var j = 0; j < 9; j++) {
        if ((in.plan.z >> u32(j)) & 1u) == 0u {
            continue;
        }
        let o = near[j];
        let w = weighed(near, q, o.sprite, false);
        laid = over(quarterOf(o, w, s, (t.spread + o.spread) / 2.0), laid);
    }
    return laid;
}

// baseOf is the layer's own of a cell whose top is drawn in base, at uv across it: the sprite,
// its detail, or the water lying on it where base shines — running where its kind flows at fx,
// fy at the cell's corners (vs_main: as fast as the slopes of the cells meeting there run down),
// else still.
fn baseOf(base: Top, uv: vec2<f32>, fx: vec4<f32>, fy: vec4<f32>) -> vec4<f32> {
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
