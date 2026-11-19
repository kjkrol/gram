// The bodies of kinds (world.Body), registered for every shader: for what photographs them
// (body_shot.wgsl) and for a Look standing them in relief. A part of a body, read off its kind's
// row of the bodies' table (image0, world's Body.row), is a card of six vertices across the line
// of sight on the part's front; in the pixel the line of sight is met with the part itself, a
// spheroid (a ball squashed or drawn out along its line) or a cone (cut short), all in the
// body's own frame, x across, y deep and z up off the middle of its foot. A part is painted in
// its paint, dotted with the paint's lit specks and its dots, and lit in tones (shapeTone).

struct ShapeCard {
    @builtin(position) clip: vec4<f32>,
    // where the card lies in the part's own space: a spheroid's, squeezed so the spheroid is a
    // ball of radius 1 about its middle (shapeSqueezed); a cone's, the body's off the cone's From
    @location(0) at: vec3<f32>,
    @location(1) deep: vec2<f32>, // the card's clip z and w
    // the line of sight in that space, its way where the lines all run one way, else the eye;
    // the part's shape
    @location(2) @interpolate(flat) ray: vec4<f32>,
    // a spheroid's line and one over half its length; a cone's line from its From to its To and
    // half its width at its To
    @location(3) @interpolate(flat) axis: vec4<f32>,
    // its paint, and how many of the dots' cells a unit of the part's space holds, 0 undotted
    @location(4) @interpolate(flat) paint: vec4<f32>,
    // its lit dots' colour and its other dots', the red under 0 where it has none, each with one
    // of the two by which the clip's z and w go along the line of sight
    @location(5) @interpolate(flat) lit: vec4<f32>,
    @location(6) @interpolate(flat) dots: vec4<f32>,
    // the light on it, and how far it is turned to the Fog
    @location(7) @interpolate(flat) light: vec4<f32>,
    // where the part's cells begin; how its body is turned about its foot, the cosine and the
    // sine; one over half a spheroid's width, or half a cone's width at its From
    @location(8) @interpolate(flat) more: vec4<f32>,
}

// ShapeMet is where a line of sight meets a part: the way out of the part there, in the body's
// frame, how far along the line, in its way's lengths and under 0 where it meets none, where in
// the part's space, and how far up a cone, 0 at its From to 1 at its To.
struct ShapeMet {
    out: vec3<f32>,
    far: f32,
    at: vec3<f32>,
    up: f32,
}

// a paint's texels in the table and a part's, and those before a row's parts (world's body.go,
// the same); how much wider than the part its card is
const shapePaintTexels: f32 = 3.0;
const shapePartTexels: f32 = 3.0;
const shapeRowTexels: f32 = 13.0;
const shapeCardRoom: f32 = 1.03;

// How dark a part's side turned from the light is, how bright its side turned to it and the cap
// facing it, and how squarely it faces the light where its shade ends and where those two
// begin, each over shapeToneEdge.
const shapeShade: f32 = 0.68;
const shapeLit: f32 = 1.2;
const shapeCap: f32 = 1.4;
const shapeShadeTo: f32 = -0.2;
const shapeLitFrom: f32 = 0.3;
const shapeCapFrom: f32 = 0.8;
const shapeToneEdge: f32 = 0.06;

// how much less a cone faces the light at its end than at its foot: it darkens up to under what
// stands over it
const shapeUnder: f32 = 0.6;

// The dots over a paint: how wide one is, in the body's widths, and the shares of them that are
// its lit dots and its other dots.
const shapeDot: f32 = 0.07;
const shapeLitDots: f32 = 0.14;
const shapeOtherDots: f32 = 0.05;

// how high over the level a card's views look down from, radians: the 2:1 view's (world's
// bodyElevation, the same)
const shapeElevation: f32 = 0.5235988;

// shapeSteepest is the steepest slope a standing card follows the eye to (shapeRow), 70° down:
// past it a card lies (world's steepest, the same).
const shapeSteepest: f32 = 2.7474774;

// shapeMargin is how far a body's card reaches under its foot and over its top: what the
// cards' eye climbs over half the body's width.
fn shapeMargin(wide: f32) -> f32 {
    return 0.5 * wide * tan(shapeElevation);
}

// shapeRow is where a step up the picture of a body's card goes, for an eye looking along d:
// how far back on the level, off the eye, and how far up. Seen no steeper than the cards'
// elevation it goes straight up; steeper, the card leans back by as much, the eye where its
// picture was taken from, and its rows spread as the outline of a ball squashed, wide across
// and reaching half over the height the card leans about, grows on an upright plane (Body.grown,
// the same): on the screen what the card holds there as tall as ever, what stands under it going
// under the ground.
fn shapeRow(d: vec3<f32>, wide: f32, half: f32) -> vec2<f32> {
    let was = tan(shapeElevation);
    let slope = min(abs(d.z) / max(length(d.xy), 1e-3), shapeSteepest);
    if slope <= was {
        return vec2<f32>(0.0, 1.0);
    }
    let across = 0.25 * wide * wide;
    let up = max(half * half, 1e-3);
    let grown = sqrt((up + across * slope * slope) / (up + across * was * was));
    // the eye's elevation e and the cards' own: the card leans back by e less theirs, its
    // rows as long as show grown times as tall on the upright plane
    let e = vec2<f32>(1.0, slope) / sqrt(1.0 + slope * slope);
    let own = vec2<f32>(cos(shapeElevation), sin(shapeElevation));
    let back = e.y * own.x - e.x * own.y;
    let on = e.x * own.x + e.y * own.y;
    return vec2<f32>(back * sign(-d.z), on) * (grown * e.x / own.x);
}

fn shapeTexel(col: f32, row: f32) -> vec4<f32> {
    return imageSrc0At(imageSrc0Origin() + vec2<f32>(col + 0.5, row + 0.5));
}

// shapePoint is where the end t of a part stands in the frame of a body wide and tall.
fn shapePoint(t: vec4<f32>, wide: f32, tall: f32) -> vec3<f32> {
    return vec3<f32>((t.xy * 2.0 - 1.0) * wide, t.z * tall);
}

// shapeTurned is v turned about the upright by the angle whose cosine and sine turn holds.
fn shapeTurned(v: vec3<f32>, turn: vec2<f32>) -> vec3<f32> {
    return vec3<f32>(v.x * turn.x - v.y * turn.y, v.x * turn.y + v.y * turn.x, v.z);
}

// shapeSqueezed is v in a spheroid's own space, where the spheroid is a ball of radius 1: shrunk by
// half its length along its line w and by half its width r across it.
fn shapeSqueezed(v: vec3<f32>, w: vec3<f32>, r: f32, half: f32) -> vec3<f32> {
    let along = dot(v, w);
    return (v - w * along) / r + w * (along / half);
}

// shapeTone is how bright what faces the light as squarely as facing is drawn, of its colour.
fn shapeTone(facing: f32) -> f32 {
    var tone = mix(shapeShade, 1.0, smoothstep(shapeShadeTo - shapeToneEdge, shapeShadeTo, facing));
    tone = mix(tone, shapeLit, smoothstep(shapeLitFrom, shapeLitFrom + shapeToneEdge, facing));
    return mix(tone, shapeCap, smoothstep(shapeCapFrom, shapeCapFrom + shapeToneEdge, facing));
}

// shapeCardOf fills c with what the card of a part hands its pixels and returns the card's
// corner vid, all in the body's frame: the part runs from a to b, half as wide as ra and rb
// there, kind its third texel of the row of a body wide across; seen along way, the lines of
// sight all one way (ortho), or from eye; seed where its dots' cells begin.
fn shapeCardOf(
    c: ptr<function, ShapeCard>,
    vid: u32,
    a: vec3<f32>,
    b: vec3<f32>,
    ra: f32,
    rb: f32,
    kind: vec4<f32>,
    row: f32,
    wide: f32,
    way: vec3<f32>,
    eye: vec3<f32>,
    ortho: bool,
    seed: f32,
) -> vec3<f32> {
    var across = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var along = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    // the card runs along the part as the eye sees it and stands across it and the line of sight
    let line = b - a;
    var run = line - way * dot(line, way);
    if dot(run, run) < 1e-8 {
        run = cross(way, vec3<f32>(0.0, 0.0, 1.0));
    }
    if dot(run, run) < 1e-8 {
        run = vec3<f32>(1.0, 0.0, 0.0);
    }
    run = normalize(run);
    let side = normalize(cross(way, run));
    let r = max(ra, rb) * shapeCardRoom;
    let k = vid % 6u;
    var p = mix(a - run * r, b + run * r, along[k]) + side * ((across[k] * 2.0 - 1.0) * r);
    // on the part's front: nothing of it lies nearer the eye than its card, so a depth written
    // no nearer than the card's is tested before the pixel is worked out
    p += way * (min(dot(a, way), dot(b, way)) - max(ra, rb) - dot(p, way));
    let shape = round(kind.x * 255.0);
    var girth = ra;
    if shape < 1.5 {
        let mid = 0.5 * (a + b);
        let half = max(0.5 * length(line), 1e-4);
        let w = line / (2.0 * half);
        let r1 = max(ra, 1e-4);
        (*c).at = shapeSqueezed(p - mid, w, r1, half);
        var ray = shapeSqueezed(way, w, r1, half);
        if !ortho {
            ray = shapeSqueezed(eye - mid, w, r1, half);
        }
        (*c).ray = vec4<f32>(ray, shape);
        (*c).axis = vec4<f32>(w, 1.0 / half);
        girth = 1.0 / r1;
    } else {
        (*c).at = p - a;
        var ray = way;
        if !ortho {
            ray = eye - a;
        }
        (*c).ray = vec4<f32>(ray, shape);
        (*c).axis = vec4<f32>(line, rb);
    }
    let col = 1.0 + shapePaintTexels * round(kind.y * 255.0);
    let lit = shapeTexel(col + 1.0, row);
    let dots = shapeTexel(col + 2.0, row);
    var cells = 0.0;
    if lit.a > 0.5 || dots.a > 0.5 {
        cells = 1.0 / (shapeDot * wide);
        if shape < 1.5 {
            cells *= max(ra, 1e-4);
        }
    }
    (*c).paint = vec4<f32>(shapeTexel(col, row).rgb, cells);
    (*c).lit = vec4<f32>(-1.0, 0.0, 0.0, 0.0);
    if lit.a > 0.5 {
        (*c).lit = vec4<f32>(lit.rgb, 0.0);
    }
    (*c).dots = vec4<f32>(-1.0, 0.0, 0.0, 0.0);
    if dots.a > 0.5 {
        (*c).dots = vec4<f32>(dots.rgb, 0.0);
    }
    (*c).more = vec4<f32>(seed, 1.0, 0.0, girth);
    return p;
}

// shapeMet is where the line of sight through the card's pixel first meets its part, the lines
// of sight all one way (ortho) or from the eye.
fn shapeMet(c: ShapeCard, ortho: bool) -> ShapeMet {
    var from = c.at;
    var way = c.ray.xyz;
    if !ortho {
        from = c.ray.xyz;
        way = c.at - c.ray.xyz;
    }
    if c.ray.w < 1.5 {
        return shapeSpheroidMet(from, way, c.axis, c.more.w);
    }
    return shapeConeMet(from, way, c.axis.xyz, c.more.w, c.axis.w);
}

// shapeColour is what the part is painted in where met: its paint, or the dot of the cell there.
fn shapeColour(c: ShapeCard, m: ShapeMet) -> vec3<f32> {
    if c.paint.w > 0.0 {
        let throw = shapeThrow(floor(m.at * c.paint.w + c.more.x));
        if throw < shapeLitDots && c.lit.r >= 0.0 {
            return c.lit.rgb;
        }
        if throw > 1.0 - shapeOtherDots && c.dots.r >= 0.0 {
            return c.dots.rgb;
        }
    }
    return c.paint.rgb;
}

// shapeFacing is how squarely the part faces the light sun, in the body's frame, where met: a
// cone the less the further up it.
fn shapeFacing(m: ShapeMet, sun: vec3<f32>) -> f32 {
    return dot(m.out, sun) - shapeUnder * m.up;
}

// shapeThrow is the throw of a cell of a part's surface, 0 to 1: whole numbers, the same every
// time.
fn shapeThrow(cell: vec3<f32>) -> f32 {
    let c = vec3<u32>(vec3<i32>(cell));
    var h = (c.x * 0x9E3779B1u) ^ (c.y * 0x85EBCA77u) ^ (c.z * 0xC2B2AE3Du);
    h ^= h >> 15u;
    h *= 0x2C1B3C6Du;
    h ^= h >> 12u;
    return f32(h >> 16u) / 65535.0;
}

// shapeSpheroidMet is where the line from..from+way, in the spheroid's own space
// (shapeSqueezed), first meets the spheroid, a ball of radius 1 there, its line axis.xyz, one
// over half its length axis.w and one over half its width girth.
fn shapeSpheroidMet(from: vec3<f32>, way: vec3<f32>, axis: vec4<f32>, girth: f32) -> ShapeMet {
    var m: ShapeMet;
    m.far = -1.0;
    let a = dot(way, way);
    let b = dot(from, way);
    let h = b * b - a * (dot(from, from) - 1.0);
    if h < 0.0 {
        return m;
    }
    let s = (-b - sqrt(h)) / a;
    if s < 0.0 {
        return m;
    }
    let q = from + way * s;
    // the way out of it, bent back from the squeeze
    let up = dot(q, axis.xyz);
    m.out = normalize((q - axis.xyz * up) * girth + axis.xyz * (up * axis.w));
    m.far = s;
    m.at = q;
    m.up = 0.0;
    return m;
}

// shapeConeMet is where the line from..from+way, counted off the cone's From, first meets the
// cone, cut short along line, as wide as twice ra at its foot and twice rb at its end,
// its ends closed (after Inigo Quilez's capped cone).
fn shapeConeMet(from: vec3<f32>, way: vec3<f32>, line: vec3<f32>, ra: f32, rb: f32) -> ShapeMet {
    var m: ShapeMet;
    m.far = -1.0;
    let m0 = dot(line, line);
    let m1 = dot(from, line);
    let m2 = dot(way, line);
    let m3 = dot(way, from);
    let m4 = dot(way, way);
    let m5 = dot(from, from);
    let m9 = m1 - m0;
    if m0 < 1e-8 {
        return m;
    }
    // its ends
    if m1 < 0.0 {
        let q = from * m2 - way * m1;
        if dot(q, q) < ra * ra * m2 * m2 {
            m.out = -line * inverseSqrt(m0);
            m.far = -m1 / m2;
            m.at = from + way * m.far;
            m.up = 0.0;
            return m;
        }
    } else if m9 > 0.0 {
        let s = -m9 / m2;
        let q = from - line + way * s;
        if dot(q, q) < rb * rb {
            m.out = line * inverseSqrt(m0);
            m.far = s;
            m.at = from + way * s;
            m.up = 1.0;
            return m;
        }
    }
    // its side
    let rr = ra - rb;
    let hy = m0 + rr * rr;
    let k2 = m0 * m0 * m4 - m2 * m2 * hy;
    let k1 = m0 * m0 * m3 - m1 * m2 * hy + m0 * ra * (rr * m2);
    let k0 = m0 * m0 * m5 - m1 * m1 * hy + m0 * ra * (rr * m1 * 2.0 - m0 * ra);
    let h = k1 * k1 - k2 * k0;
    if h < 0.0 {
        return m;
    }
    let s = (-k1 - sqrt(h)) / k2;
    let y = m1 + s * m2;
    if y < 0.0 || y > m0 || s < 0.0 {
        return m;
    }
    m.at = from + way * s;
    m.out = normalize(m0 * (m0 * m.at + rr * line * ra) - line * hy * y);
    m.far = s;
    m.up = y / m0;
    return m;
}
