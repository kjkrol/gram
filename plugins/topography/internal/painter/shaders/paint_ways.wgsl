// The ways across the board's tiles (painter's bake_gpu.go), an instance a cell a way runs across
// (its column and row), each laid in pieces as wayAnew lays them: bands from the cell's middle
// out to the neighbours its Links name, curving round the middle, a way out to one neighbour
// ending square, to more running as a stem through the two widest with the rest joining it
// halfway, a way out to a cell lying under the rest running on under it, as wide and faded as the
// way and its partner across the side say, masked by the land round it on a square board. A
// world unit lies U.PaintScale pixels apart in a target U.PaintTarget large. A square board's
// cells are U.PaintCell across; a hex board's (U.PaintHex) pointy-top, U.PaintCell from centre to
// corner. The ground is U.PaintLow plus U.PaintSpan at its corners (cells.go); U.PaintFine has
// the bands in their fine steps, U.PaintDetail scales the water's shine, U.PaintHeights says
// whether the water shines at all.

struct Lane {
    @builtin(position) clip: vec4<f32>,
    @location(0) world: vec2<f32>,
    @location(1) uv: vec2<f32>,
    @location(2) weight: f32,
    @location(3) mix: f32,
    @location(4) flow: vec2<f32>,
    // the way's sprite, its mix's, its detail and its mix's... as ids; shine, the mix's shine,
    // how soft it blends and its bits
    @location(5) @interpolate(flat) look: vec4<u32>,
    @location(6) @interpolate(flat) params: vec4<f32>,
}

// a piece's bits
const laneFaded: u32 = 1u;
const laneMixes: u32 = 2u;
const laneRuns: u32 = 4u;
const laneGlints: u32 = 8u;

// rise over run under which water lies level (way.go)
const stillFall: f32 = 0.01;

// Way is what runs across a cell (cells.go's lane).
struct Way {
    runs: bool,
    sprite: u32,
    links: u32,
    detail: u32,
    width: f32,
    fade: f32,
    mix: f32,
    shine: f32,
    flow: f32,
    mixShine: f32,
    mixSprite: u32,
    mixes: bool,
}

fn wayOf(b: vec2<f32>) -> Way {
    let t0 = texelOf(b, 0.0, 0.0);
    let t2 = texelOf(b, 2.0, 0.0);
    let t3 = texelOf(b, 3.0, 0.0);
    let u0 = texelOf(b, 0.0, 1.0);
    let u1 = texelOf(b, 1.0, 1.0);
    let flags = byteIn(t0, 2);
    var w: Way;
    w.runs = (flags & flagRuns) != 0u;
    w.mixes = (flags & flagMixes) != 0u;
    w.sprite = byteIn(t2, 0) | (byteIn(t2, 1) << 8u);
    w.links = byteIn(t2, 2);
    w.detail = byteIn(t2, 3);
    w.width = f32(byteIn(t3, 0) | (byteIn(t3, 1) << 8u)) / 64.0;
    w.fade = f32(byteIn(t3, 2)) / 255.0;
    w.mix = f32(byteIn(t3, 3)) / 255.0;
    w.shine = f32(byteIn(u0, 0)) / 255.0;
    w.flow = f32(byteIn(u0, 1) | (byteIn(u0, 2) << 8u)) / 16.0;
    w.mixShine = f32(byteIn(u0, 3)) / 255.0;
    w.mixSprite = byteIn(u1, 0) | (byteIn(u1, 1) << 8u);
    return w;
}

// directions is how many neighbours a cell has: a square board's eight, a hex board's six.
fn directions() -> i32 {
    return select(8, 6, U.PaintHex > 0.5);
}

// offsetOf is the cells across and down to the neighbour in the grid's direction i: a square
// board's north, south, west, east, then north-west, north-east, south-west, south-east; a hex
// board's axial east, north-east, north-west, west, south-west, south-east.
fn offsetOf(i: i32) -> vec2<f32> {
    if U.PaintHex > 0.5 {
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
        return d;
    }
    var d = vec2<f32>(select(-1.0, 1.0, i == 5 || i == 7), select(-1.0, 1.0, i >= 6));
    if i < 2 {
        d = vec2<f32>(0.0, select(-1.0, 1.0, i == 1));
    } else if i < 4 {
        d = vec2<f32>(select(-1.0, 1.0, i == 3), 0.0);
    }
    return d;
}

// centreOf is the middle of the cell c, as far along as c is.
fn centreOf(c: vec2<f32>) -> vec2<f32> {
    let s = U.PaintCell;
    if U.PaintHex > 0.5 {
        return vec2<f32>(s * (sqrt(3.0) * c.x + sqrt(3.0) / 2.0 * c.y) + s, s * 1.5 * c.y + s);
    }
    return (c + 0.5) * s;
}

// cellAt is the cell the point p lies in, as far along as p is.
fn cellAt(p: vec2<f32>) -> vec2<f32> {
    let s = U.PaintCell;
    if U.PaintHex > 0.5 {
        let x = p.x - s;
        let y = p.y - s;
        let c = vec2<f32>((sqrt(3.0) / 3.0 * x - y / 3.0) / s, (2.0 / 3.0 * y) / s);
        let t = -c.x - c.y;
        var q = round(c.x);
        var r = round(c.y);
        let tr = round(t);
        let dq = abs(q - c.x);
        let dr = abs(r - c.y);
        let dt = abs(tr - t);
        if dq > dr && dq > dt {
            q = -r - tr;
        } else if dr > dt {
            r = -q - tr;
        }
        return vec2<f32>(q, r);
    }
    return floor(p / s);
}

// groundOf is the ground at the cell b's corners: top-left, top-right, bottom-left,
// bottom-right.
fn groundOf(b: vec2<f32>) -> vec4<f32> {
    let a = texelOf(b, 2.0, 1.0);
    let c = texelOf(b, 3.0, 1.0);
    let v = vec4<f32>(f32(byteIn(a, 0) | (byteIn(a, 1) << 8u)),
        f32(byteIn(a, 2) | (byteIn(a, 3) << 8u)), f32(byteIn(c, 0) | (byteIn(c, 1) << 8u)),
        f32(byteIn(c, 2) | (byteIn(c, 3) << 8u)));
    return U.PaintLow + v / 65535.0 * U.PaintSpan;
}

// groundAt is the ground at p as the relief has it (relief.GroundAt): over a square cell its two
// triangles split along the diagonal whose corners stand nearer in height; a hex cell's level; 0
// off the board.
fn groundAt(p: vec2<f32>) -> f32 {
    let c = cellAt(p);
    let b = folded(c);
    if b.z < 0.5 {
        return 0.0;
    }
    let h = groundOf(b.xy);
    if U.PaintHex > 0.5 {
        return h.x;
    }
    let f = p / U.PaintCell - c;
    let u = f.x;
    let v = f.y;
    if abs(h.x - h.w) < abs(h.y - h.z) {
        if u >= v {
            return h.x + u * (h.y - h.x) + v * (h.w - h.y);
        }
        return h.x + v * (h.z - h.x) + u * (h.w - h.z);
    }
    if u + v <= 1.0 {
        return h.x + u * (h.y - h.x) + v * (h.z - h.x);
    }
    return h.w + (1.0 - u) * (h.z - h.w) + (1.0 - v) * (h.y - h.w);
}

// landAt is how much of a square board's ground at p is land: the share of the cells round the
// point's quarter that lie not under the rest, blended across the quarter; 1 on a hex board and
// off the board.
fn landAt(p: vec2<f32>) -> f32 {
    if U.PaintHex > 0.5 {
        return 1.0;
    }
    let f = p / U.PaintCell;
    let c = floor(f);
    let b = folded(c);
    if b.z < 0.5 {
        return 1.0;
    }
    let mine = topOf(b.xy);
    var dry: array<f32, 9>;
    for (var k = 0; k < 9; k++) {
        let n = folded(b.xy + vec2<f32>(f32(k % 3) - 1.0, f32(k / 3) - 1.0));
        var t = mine;
        if n.z > 0.5 {
            t = topOf(n.xy);
        }
        dry[k] = select(0.0, 1.0, (t.flags & flagUnder) == 0u);
    }
    let g = 2.0 * (f - c);
    let q = min(floor(g), vec2<f32>(1.0));
    let s = g - q;
    var w: vec4<f32>;
    for (var corner = 0; corner < 4; corner++) {
        let point = vec2<i32>(i32(q.x) + corner % 2, i32(q.y) + corner / 2);
        let lo = vec2<i32>(select(1, 0, point.x == 0), select(1, 0, point.y == 0));
        let hi = vec2<i32>(select(1, 2, point.x == 2), select(1, 2, point.y == 2));
        var n = 0.0;
        var all = 0.0;
        for (var y = lo.y; y <= hi.y; y++) {
            for (var x = lo.x; x <= hi.x; x++) {
                n += dry[y * 3 + x];
                all += 1.0;
            }
        }
        w[corner] = n / all;
    }
    let top = w.x + (w.y - w.x) * s.x;
    let bottom = w.z + (w.w - w.z) * s.x;
    return top + (bottom - top) * s.y;
}

// Out is where a band starts or ends: its middle, half its width, how far faded, how far turned
// into the mix, how wide the partner across the side is and whether it runs to a corner.
struct Out {
    at: vec2<f32>,
    half: f32,
    fade: f32,
    mix: f32,
    wide: f32,
}

// Piece is a piece of a band: its corners (top-left, top-right, bottom-left, bottom-right: the
// left side at its start and end, then the right side), how much of the way shows and how far it
// has turned at its start and its end, how fast its water runs along it there, and whether it
// fades.
struct Piece {
    a: vec2<f32>,
    b: vec2<f32>,
    c: vec2<f32>,
    d: vec2<f32>,
    shows: vec2<f32>,
    mix: vec2<f32>,
    flow0: vec2<f32>,
    flow1: vec2<f32>,
    faded: bool,
    on: bool,
}

// bandAt is the point u along the band from a to b round m: where it lies, its way along, its
// half width, how much of it shows and how far it has turned.
struct BandAt {
    at: vec2<f32>,
    along: vec2<f32>,
    half: f32,
    shows: f32,
    mix: f32,
}

fn bandAt(a: Out, b: Out, m: vec2<f32>, w: Way, u: f32) -> BandAt {
    let v = 1.0 - u;
    var out: BandAt;
    out.at = v * v * a.at + 2.0 * u * v * m + u * u * b.at;
    var t = 2.0 * v * (m - a.at) + 2.0 * u * (b.at - m);
    let l = length(t);
    if l > 0.0 {
        t = t / l;
    }
    out.along = t;
    out.half = v * v * a.half + 2.0 * u * v * (w.width / 2.0) + u * u * b.half;
    out.shows = 1.0 - (v * v * a.fade + 2.0 * u * v * w.fade + u * u * b.fade);
    out.mix = v * v * a.mix + 2.0 * u * v * w.mix + u * u * b.mix;
    return out;
}

// band is the k-th of steps pieces of the band from a to b round m, the way w's
fn band(a: Out, b: Out, m: vec2<f32>, w: Way, steps: i32, k: i32) -> Piece {
    let span = distance(a.at, m) + distance(b.at, m);
    var speed = 0.0;
    if span > 0.0 && w.flow > 0.0 {
        var fall = (groundAt(a.at) - groundAt(b.at)) / span;
        // level water runs on the way the way fades: out into the sea
        if abs(fall) < stillFall && a.fade != b.fade {
            fall = select(stillFall, -stillFall, b.fade < a.fade);
        }
        if fall != 0.0 {
            speed = w.flow * sqrt(abs(fall)) * sign(fall);
        }
    }
    let s0 = bandAt(a, b, m, w, f32(k - 1) / f32(steps));
    let s1 = bandAt(a, b, m, w, f32(k) / f32(steps));
    var p: Piece;
    let n0 = vec2<f32>(-s0.along.y, s0.along.x) * s0.half;
    let n1 = vec2<f32>(-s1.along.y, s1.along.x) * s1.half;
    p.a = s0.at + n0;
    p.b = s1.at + n1;
    p.c = s0.at - n0;
    p.d = s1.at - n1;
    p.flow0 = s0.along * speed;
    p.flow1 = s1.along * speed;
    p.shows = vec2<f32>(s0.shows, s1.shows);
    p.mix = vec2<f32>(s0.mix, s1.mix);
    p.faded = s0.shows < 1.0 || s1.shows < 1.0;
    p.on = true;
    return p;
}

// stepsOf is n steps in the fine bands, a third of them, at least one, in the coarse.
fn stepsOf(n: i32) -> i32 {
    if U.PaintFine > 0.5 {
        return n;
    }
    return max(n / 3, 1);
}

// pieceOf is the p-th piece of the way across the cell c, b the board's own c stands for: off
// where it has fewer.
fn pieceOf(c: vec2<f32>, b: vec2<f32>, w: Way, p: i32) -> Piece {
    var none: Piece;
    let mid = centreOf(c);
    let h = w.width / 2.0;
    var outs: array<Out, 8>;
    var into: array<Out, 8>;
    var under: array<bool, 8>;
    var n = 0;
    for (var i = 0; i < directions(); i++) {
        if ((w.links >> u32(i)) & 1u) == 0u {
            continue;
        }
        let nb = folded(b + offsetOf(i));
        if nb.z < 0.5 {
            continue;
        }
        let e = (centreOf(c + offsetOf(i)) - mid) / 2.0;
        var o: Out;
        o.at = mid + e;
        o.half = h;
        o.fade = w.fade / 2.0;
        o.mix = w.mix;
        o.wide = 0.0;
        var below = false;
        let other = wayOf(nb.xy);
        if other.runs {
            o.half = (w.width + other.width) / 4.0;
            o.wide = other.width;
            o.fade = (w.fade + other.fade) / 2.0;
            o.mix = (w.mix + other.mix) / 2.0;
        } else if (topOf(nb.xy).flags & flagUnder) != 0u {
            // out into water: wider than any way in, so always the stem
            o.fade = w.fade;
            o.wide = 2.0 * w.width;
            below = true;
            var deep: Out;
            deep.at = mid + 2.0 * e;
            deep.half = h;
            deep.fade = w.fade;
            deep.mix = 1.0;
            into[n] = deep;
        }
        outs[n] = o;
        under[n] = below;
        n += 1;
    }
    var core: Out;
    core.at = mid;
    core.half = h;
    core.fade = w.fade;
    core.mix = w.mix;
    var at = p;
    if n == 0 {
        if at != 0 {
            return none;
        }
        var q: Piece;
        q.a = mid + vec2<f32>(-h, -h);
        q.b = mid + vec2<f32>(h, -h);
        q.c = mid + vec2<f32>(-h, h);
        q.d = mid + vec2<f32>(h, h);
        q.shows = vec2<f32>(1.0 - w.fade);
        q.mix = vec2<f32>(w.mix);
        q.faded = w.fade > 0.0;
        q.on = true;
        return q;
    }
    if n == 1 {
        let o = outs[0];
        let ax = o.at - mid;
        var back = core;
        back.at = mid - ax / length(ax) * h;
        if w.fade > 0.0 {
            back.fade = 1.0; // a way fading out ends in nothing
        }
        if at == 0 {
            return band(back, core, (back.at + core.at) / 2.0, w, 1, 1);
        }
        at -= 1;
        let s = stepsOf(4);
        if at < s {
            return band(core, o, (core.at + o.at) / 2.0, w, s, at + 1);
        }
        at -= s;
    } else {
        // the stem is the ways out to the two widest neighbours; any other joins it halfway
        var a = 0;
        var bi = 1;
        if outs[1].wide > outs[0].wide {
            a = 1;
            bi = 0;
        }
        for (var i = 2; i < n; i++) {
            if outs[i].wide > outs[a].wide {
                bi = a;
                a = i;
            } else if outs[i].wide > outs[bi].wide {
                bi = i;
            }
        }
        let s6 = stepsOf(6);
        if at < s6 {
            return band(outs[a], outs[bi], mid, w, s6, at + 1);
        }
        at -= s6;
        var joint = core;
        joint.at = 0.25 * outs[a].at + 0.5 * mid + 0.25 * outs[bi].at;
        let s4 = stepsOf(4);
        for (var i = 0; i < n; i++) {
            if i == a || i == bi {
                continue;
            }
            if at < s4 {
                var end = joint;
                end.half = min(outs[i].half, h);
                return band(outs[i], end, mid, w, s4, at + 1);
            }
            at -= s4;
        }
    }
    // a way running out into water runs on under it
    let s2 = stepsOf(2);
    for (var i = 0; i < n; i++) {
        if !under[i] {
            continue;
        }
        if at < s2 {
            return band(outs[i], into[i], (outs[i].at + into[i].at) / 2.0, w, s2, at + 1);
        }
        at -= s2;
    }
    return none;
}

// the largest number of pieces a way lays: a stem through eight, six joining it, all running
// into water
const lanePieces: i32 = 46;

@vertex
fn vs_main(@builtin(vertex_index) vid: u32, @location(0) at: vec4<f32>) -> Lane {
    var out: Lane;
    out.clip = vec4<f32>(2.0, 2.0, 0.0, 1.0); // nowhere, unless a piece is laid
    let c = at.xy;
    let b = folded(c);
    let w = wayOf(b.xy);
    var p = pieceOf(c, b.xy, w, i32(vid / 6u));
    if !p.on || !w.runs {
        return out;
    }
    // the land round it masks it, over where it fades
    var weight = vec4<f32>(p.shows.x, p.shows.y, p.shows.x, p.shows.y);
    var soft = 0.0;
    let land = vec4<f32>(landAt(p.a), landAt(p.b), landAt(p.c), landAt(p.d));
    if min(min(land.x, land.y), min(land.z, land.w)) < 1.0 {
        if !p.faded {
            weight = vec4<f32>(1.0);
        }
        weight = min(weight, land);
        p.faded = true;
        soft = topOf(b.xy).spread;
        if soft <= 0.0 {
            soft = 0.25;
        }
    }
    if soft <= 0.0 {
        soft = 0.5;
    }
    // two triangles: top-left, top-right, bottom-left, then top-right, bottom-left, bottom-right
    var k = vid % 6u;
    if k >= 3u {
        k -= 2u;
    }
    var corner = p.a;
    var flow = p.flow0;
    if k == 1u {
        corner = p.b;
        flow = p.flow1;
    } else if k == 2u {
        corner = p.c;
    } else if k == 3u {
        corner = p.d;
        flow = p.flow1;
    }
    var mix = vec2<f32>(0.0);
    if w.mixes {
        mix = clamp(p.mix, vec2<f32>(0.0), vec2<f32>(1.0));
    }
    let shine = select(0.0, w.shine, U.PaintHeights > 0.5) * U.PaintDetail;
    let mixShine = select(0.0, w.mixShine, U.PaintHeights > 0.5) * U.PaintDetail;
    var bits = 0u;
    if p.faded {
        bits |= laneFaded;
    }
    if w.mixes && max(mix.x, mix.y) > 0.0 {
        bits |= laneMixes;
    }
    if shine > 0.0 && (shine * (1.0 - mix.x) > 0.0 || shine * (1.0 - mix.y) > 0.0) {
        bits |= laneRuns;
    }
    if !p.faded && mixShine * max(mix.x, mix.y) > 0.0 {
        bits |= laneGlints;
    }
    let px = corner * U.PaintScale;
    out.clip = vec4<f32>(px.x / U.PaintTarget.x * 2.0 - 1.0, 1.0 - px.y / U.PaintTarget.y * 2.0,
        0.0, 1.0);
    out.world = corner;
    out.uv = vec2<f32>(f32(k % 2u), f32(k / 2u));
    out.weight = weight[k];
    out.mix = mix[k % 2u];
    out.flow = flow;
    out.look = vec4<u32>(w.sprite, w.mixSprite, w.detail, 0u);
    out.params = vec4<f32>(shine, mixShine, soft, f32(bits));
    return out;
}

// masked reports whether the pixel at frag lies outside the cells painted anew.
fn masked(frag: vec2<f32>) -> bool {
    if U.PaintMasked < 0.5 {
        return false;
    }
    let c = floor(frag / U.PaintPx);
    return imageSrc3At(imageSrc3Origin() + c + 0.5).r < 0.5;
}
