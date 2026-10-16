// What falls, drop by drop, between the low clouds' base (U.CloudBases.x) and the ground
// (precipitation.Drops), six vertices a drop: U.DropsEach drops in each of three boxes round where
// the eye looks (U.DropBox0 to U.DropBox2: the middle and how wide, none where 0; U.DropTall how
// tall each), every drop a
// point of a lattice fixed in the world that falls along (U.DropLean, -1) U.DropSpeed world units
// a second, kept only where the clouds over it let as much fall; drawn as a streak U.DropLength
// long trailing up behind it, U.DropWidth pixels wide, in U.DropColor, through the scene's
// transform (U.DropViewProj, sunk by U.DropBend round U.DropEye) onto a view U.DropView pixels
// wide: a dot seen from above, a short line from the side, gone as it shrinks under a pixel
// (U.DropPx pixels a world unit spans, at a world unit off through a perspective).

struct Drop {
    @builtin(position) clip: vec4<f32>,
    @location(0) color: vec4<f32>,
    @location(1) across: f32,
}

@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> Drop {
    var d = Drop(vec4<f32>(0.0, 0.0, 0.0, 1.0), vec4<f32>(0.0), 0.0);
    let i = vid / 6u;
    let k = i / u32(U.DropsEach);
    var box = U.DropBox0;
    var tall = U.DropTall.x;
    if k == 1u {
        box = U.DropBox1;
        tall = U.DropTall.y;
    } else if k == 2u {
        box = U.DropBox2;
        tall = U.DropTall.z;
    } else if k > 2u {
        return d;
    }
    if box.w <= 0.0 {
        return d;
    }
    let h = dropHash(i);
    let size = vec3<f32>(box.w, box.w, tall);
    let lo = box.xyz - 0.5 * size;
    let fall = vec3<f32>(U.DropLean, -1.0) * (U.Clock * U.DropSpeed);
    let p = lo + dropWrap(h.xyz * size + fall - lo, size);
    let base = U.CloudBases.x;
    if p.z > base || p.z < max(groundHeightAt(p.xy), 0.0) {
        return d;
    }
    // where it left the clouds, and whether as much falls there
    let src = p.xy - U.DropLean * (base - p.z);
    let m = airAt(src);
    let low = cloudLowMass(m);
    if m.z <= 0.0 || low.x <= 0.0 {
        return d;
    }
    let c = cloudLowAt(src, dropRead, low.y);
    if h.w >= m.z * smoothstep(0.0, dropUnder, cloudCoverSoft(c.x, c.y, low)) {
        return d;
    }
    let a = dropClip(p);
    let b = dropClip(p + normalize(vec3<f32>(-U.DropLean, 1.0)) * U.DropLength);
    if a.w <= 1e-4 || b.w <= 1e-4 {
        return d;
    }
    var long = U.DropLength * U.DropPx;
    if U.DropOrtho < 0.5 {
        long /= a.w;
    }
    let shown = clamp((long - dropLeast) / dropLeast, 0.0, 1.0);
    if shown <= 0.0 {
        return d;
    }
    let half = 0.5 * U.DropView;
    let sa = a.xy / a.w * half;
    var sb = b.xy / b.w * half;
    if distance(sa, sb) < U.DropWidth {
        sb = sa + vec2<f32>(0.0, U.DropWidth); // seen from above: a dot
    }
    let along = normalize(sb - sa);
    let side = vec2<f32>(-along.y, along.x) * (0.5 * U.DropWidth);
    var ends = array<u32, 6>(0u, 0u, 1u, 0u, 1u, 1u);
    var sides = array<f32, 6>(-1.0, 1.0, -1.0, 1.0, 1.0, -1.0);
    let corner = vid % 6u;
    var at = sa;
    var clip = a;
    if ends[corner] == 1u {
        at = sb;
        clip = b;
    }
    let px = at + side * sides[corner];
    d.clip = vec4<f32>(px / half * clip.w, clip.z, clip.w);
    d.color = U.DropColor * (shown * dropShown);
    d.across = sides[corner];
    return d;
}

@fragment
fn fs_main(d: Drop) -> @location(0) vec4<f32> {
    return d.color * (1.0 - smoothstep(0.4, 1.0, abs(d.across)));
}

// dropClip is the world point p through the scene's transform, sunk as the camera sinks the far
// ground.
fn dropClip(p: vec3<f32>) -> vec4<f32> {
    let off = p.xy - U.DropEye.xy;
    return U.DropViewProj * vec4<f32>(p.xy, p.z - U.DropBend * dot(off, off), 1.0);
}

// dropWrap is v within 0 up to n, each way.
fn dropWrap(v: vec3<f32>, n: vec3<f32>) -> vec3<f32> {
    return v - n * floor(v / n);
}

// dropHash is four numbers 0 to 1 fixed for drop i.
fn dropHash(i: u32) -> vec4<f32> {
    var x = i * 747796405u + 2891336453u;
    var out = vec4<f32>(0.0);
    for (var k = 0; k < 4; k++) {
        x = x * 747796405u + 2891336453u;
        var w = ((x >> ((x >> 28u) + 4u)) ^ x) * 277803737u;
        w = (w >> 22u) ^ w;
        out[k] = f32(w & 0xffffffu) / 16777216.0;
    }
    return out;
}

// How wide a pixel the clouds over a drop are read as, world units; how thick a cloud must be to
// let all of it fall (air's fallUnder); how many pixels long a drop must stand to show at all,
// and twice that to show whole; how much of what lies behind a drop it hides.
const dropRead: f32 = 40.0;
const dropUnder: f32 = 0.35;
const dropLeast: f32 = 0.75;
const dropShown: f32 = 0.75;
