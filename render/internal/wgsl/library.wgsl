// The library of the composer's shader and of every shader built on it (ShaderSourceWith): the
// composer's own uniforms — U.Toward the way towards the eye, U.Clock the time in seconds, U.Pixel
// how many world units a pixel spans, U.Fog the colour what lies far off in the air turns to, the
// ground's heights (U.GroundLattice, U.GroundSpan: render.GroundHeights) — and the helpers the
// materials call. Every other uniform is a material's, declared with its
// source (render.RegisterMaterials) and set by a source with render.Frame.Uniform.

// faded is how much of a sprite shows by the fades its custom asks for: 1 where it asks none.
fn faded(custom: vec4<f32>) -> f32 {
    let fade = smoothstep(vec4<f32>(0.0), vec4<f32>(1.0), clamp(custom - 1.0, vec4<f32>(0.0),
        vec4<f32>(1.0)));
    let f = mix(vec4<f32>(1.0), fade, step(vec4<f32>(0.5), custom));
    return f.x * f.y * f.z * f.w;
}

// blended is how much of a blended sprite shows at weight, its mark 100000 plus how soft it is.
fn blended(weight: f32, mark: f32) -> f32 {
    let soft = mark - 100000.0;
    return smoothstep(0.5 - soft, 0.5 + soft, weight);
}

// seen is how much of a pattern span world units long shows: all of it over a few pixels long,
// none of it where it would flicker, finer than a pixel or two.
fn seen(span: f32) -> f32 {
    return seenAt(span, U.Pixel);
}

// seenAt is seen where a pixel spans pixel world units: for what knows it pixel by pixel, as a
// perspective draws the far off in larger ones.
fn seenAt(span: f32, pixel: f32) -> f32 {
    return smoothstep(finest, 2.0 * finest, span / max(pixel, 1e-3));
}

// finest is how many pixels long the finest pattern drawn is.
const finest: f32 = 2.0;

const pi: f32 = 3.14159265;

// groundHeightAt is the ground's height at the world point p as the frame's GroundHeights hold it
// (render.GroundHeights), blended between the lattice's corners, the edge's past it; groundNone
// in a frame without them.
fn groundHeightAt(p: vec2<f32>) -> f32 {
    if U.GroundLattice.w < 0.5 {
        return groundNone;
    }
    let g = clamp(p / U.GroundLattice.z, vec2<f32>(0.0), U.GroundLattice.xy - 1.0001);
    let i = floor(g);
    let f = g - i;
    let top = mix(groundCorner(i), groundCorner(i + vec2<f32>(1.0, 0.0)), f.x);
    let bottom = mix(groundCorner(i + vec2<f32>(0.0, 1.0)), groundCorner(i + 1.0), f.x);
    return mix(top, bottom, f.y);
}

// groundDrawnAt is the ground's height at p as the ground is drawn: two flat triangles a cell,
// split along the diagonal whose corners stand nearer in height; groundNone in a frame without
// heights.
fn groundDrawnAt(p: vec2<f32>) -> f32 {
    if U.GroundLattice.w < 0.5 {
        return groundNone;
    }
    let g = clamp(p / U.GroundLattice.z, vec2<f32>(0.0), U.GroundLattice.xy - 1.0001);
    let i = floor(g);
    return groundSplit(g - i, groundCorner(i), groundCorner(i + vec2<f32>(1.0, 0.0)),
        groundCorner(i + vec2<f32>(0.0, 1.0)), groundCorner(i + 1.0));
}

// groundSplit is the height at f, 0 to 1 across and down a cell whose corners stand at h0 to h3,
// as the cell is drawn: two flat triangles split along the diagonal whose corners stand nearer.
fn groundSplit(f: vec2<f32>, h0: f32, h1: f32, h2: f32, h3: f32) -> f32 {
    if abs(h0 - h3) < abs(h1 - h2) {
        if f.x >= f.y {
            return h0 + f.x * (h1 - h0) + f.y * (h3 - h1);
        }
        return h0 + f.y * (h2 - h0) + f.x * (h3 - h2);
    }
    if f.x + f.y <= 1.0 {
        return h0 + f.x * (h1 - h0) + f.y * (h2 - h0);
    }
    return h3 + (1.0 - f.x) * (h2 - h3) + (1.0 - f.y) * (h1 - h3);
}

// groundHighest is the highest the frame's GroundHeights reach; groundNone without them.
fn groundHighest() -> f32 {
    if U.GroundLattice.w < 0.5 {
        return groundNone;
    }
    return U.GroundSpan.z + U.GroundSpan.w;
}

fn groundCorner(i: vec2<f32>) -> f32 {
    let c = textureLoad(GroundHeights, vec2<i32>(U.GroundSpan.xy + i), 0);
    return U.GroundSpan.z + (c.r * 255.0 * 256.0 + c.g * 255.0) / 65535.0 * U.GroundSpan.w;
}

const groundNone: f32 = -3.0e38;

// The shade a drawn thing takes, bits of its Taking (render.Taking): the relief's shadow, the
// blocks', the clouds' and the lamps' light; takes says whether t, the bits as a float, holds bit.
const takesRelief: u32 = 1u;
const takesBlocks: u32 = 2u;
const takesClouds: u32 = 4u;
const takesLamps: u32 = 8u;

fn takes(t: f32, bit: u32) -> bool {
    return (u32(t + 0.5) & bit) != 0u;
}

// What gives way to what is uncovered (render.Sprites.GiveWay, Uncover): a source draws what it
// has uncovered in the pass Uncovered, a disc round each thing, into the depth the passes after
// read (render.Target.Uncovered, depthAt), and what gives way is left out at the pixels written
// where it lies nearer. givesWay is the bit past a Taking's that says a thing gives way, in
// the float its Taking rides in.
const givesWay: u32 = 256u;

// How many of its pixels a disc round something uncovered leaves open at the most, how far out
// of its middle, of its reach, it leaves that many, and how much nearer a thing lies than what
// is uncovered before it gives way, of the depth.
const uncoveredMost: f32 = 0.75;
const uncoveredWhole: f32 = 0.5;
const uncoveredGap: f32 = 1e-6;

// uncoveredOpen reports whether the target's pixel px of the disc round something uncovered,
// off its middle by off (1 at its rim), is written: in a pattern the same in every square of
// four pixels, thinning outwards, so whatever gives way opens at the same pixels.
fn uncoveredOpen(off: vec2<f32>, px: vec2<f32>) -> bool {
    let open = uncoveredMost * (1.0 - smoothstep(uncoveredWhole, 1.0, length(off)));
    let x = u32(px.x);
    let y = u32(px.y);
    let turn = 4u * uncoveredTurn(x & 1u, y & 1u) + uncoveredTurn((x >> 1u) & 1u, (y >> 1u) & 1u);
    return (f32(turn) + 0.5) / 16.0 < open;
}

// where a pixel of a square of two comes in the order its pixels open in
fn uncoveredTurn(x: u32, y: u32) -> u32 {
    return ((x ^ y) << 1u) | y;
}

// uncoveredAt is the depth the pass Uncovered writes for what lies at depth: one less, so of
// those over a pixel the farthest stays and a cleared 0 is nothing.
fn uncoveredAt(depth: f32) -> f32 {
    return 1.0 - depth;
}

// givesWayAt reports whether what lies at depth at the target's pixel px gives way: something
// uncovered lies farther there.
fn givesWayAt(px: vec2<f32>, depth: f32) -> bool {
    let bared = depthAt(px);
    return bared > 0.0 && depth > 1.0 - bared + uncoveredGap;
}

// groundShadeOf is the frame's GroundShade at the world point p (render.GroundShade), each band
// faded from the shade before as far as it has come: the sun everywhere, (1, 0, 1, 1), in a frame
// without it.
fn groundShadeOf(p: vec2<f32>) -> vec4<f32> {
    if U.GroundShadeArea.z <= 0.0 {
        return vec4<f32>(1.0, 0.0, 1.0, 1.0);
    }
    let at = (p - U.GroundShadeArea.xy) / U.GroundShadeArea.zw;
    let uv = clamp(at, vec2<f32>(0.0), vec2<f32>(1.0));
    let now = textureSampleLevel(GroundShade, linear, uv, 0.0);
    let was = textureSampleLevel(GroundShadeWas, linear, uv, 0.0);
    let band = min(i32(uv.y / max(U.GroundShadeSpan.y, 1e-6)), 15);
    var mixes = U.GroundShadeMix0;
    if band >= 12 {
        mixes = U.GroundShadeMix3;
    } else if band >= 8 {
        mixes = U.GroundShadeMix2;
    } else if band >= 4 {
        mixes = U.GroundShadeMix1;
    }
    let k = band % 4;
    let pick = vec4<f32>(f32(k == 0), f32(k == 1), f32(k == 2), f32(k == 3));
    return mix(was, now, dot(mixes, pick));
}

// groundShadeAt is how much of the sun reaches the point over world units over the ground at p
// past the relief: none below how high its shadow reaches there, fading in over soft under it.
fn groundShadeAt(p: vec2<f32>, over: f32, soft: f32) -> f32 {
    let g = groundShadeOf(p).g;
    let reach = g * g * U.GroundShadeSpan.x;
    return smoothstep(reach - max(soft, 1e-3), reach, over);
}

// standingShadeAt is how much of the sun reaches the point over world units over the ground at p
// past the blocks standing on it: as much as at the ground below how high their shadows reach
// there, all of it over, fading in over soft under it.
fn standingShadeAt(p: vec2<f32>, over: f32, soft: f32) -> f32 {
    let s = groundShadeOf(p);
    let reach = (1.0 - s.b) * (1.0 - s.b) * U.GroundShadeSpan.x;
    return mix(s.a, 1.0, smoothstep(reach - max(soft, 1e-3), reach, over));
}
