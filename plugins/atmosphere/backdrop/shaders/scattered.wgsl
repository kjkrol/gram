// The scattered stars: made up, a star in one cell of starCells in a few on the sphere of the
// stars, a pixel or two wide, most faint, a few bright, twinkling, fading towards the horizon.
fn SkyStars(s: SkyAt) -> vec3<f32> {
    if s.way.z <= 0.0 {
        return vec3<f32>(0.0);
    }
    return starsAt(s.starWay, s.pixel) * s.stars * smoothstep(0.0, 0.12, s.way.z);
}

// starsAt is the light of the stars along their own unit way w, a pixel spanning pixel radians.
fn starsAt(w: vec3<f32>, pixel: f32) -> vec3<f32> {
    let cell = floor(w * starCells);
    let h = starHash(cell);
    if h.w < 1.0 - starShare {
        return vec3<f32>(0.0);
    }
    let bright = pow((h.w - (1.0 - starShare)) / starShare, 4.0);
    let at = normalize((cell + 0.25 + 0.5 * h.xyz) / starCells);
    let off = length(at - w) / max(pixel, 1e-6); // pixels from it
    let size = 0.7 + 1.1 * bright;
    let twinkle = 0.8 + 0.2 * sin(U.Clock * (1.5 + 3.0 * h.x) + 6.2831853 * h.y);
    let tint = mix(vec3<f32>(0.78, 0.85, 1.0), vec3<f32>(1.0, 0.9, 0.74), h.z);
    return tint * (0.35 + 0.9 * bright) * twinkle * (1.0 - smoothstep(0.35 * size, size, off));
}

// starHash is four numbers 0 to 1 fixed for the cell c.
fn starHash(c: vec3<f32>) -> vec4<f32> {
    let q = bitcast<vec3<u32>>(vec3<i32>(c));
    var h = q.x * 0x8da6b343u ^ q.y * 0xd8163841u ^ q.z * 0xcb1ab31fu;
    var out: vec4<f32>;
    for (var k = 0; k < 4; k++) {
        h = h * 747796405u + 2891336453u;
        var x = ((h >> ((h >> 28u) + 4u)) ^ h) * 277803737u;
        x = (x >> 22u) ^ x;
        out[k] = f32(x >> 8u) / 16777216.0;
    }
    return out;
}

// How many cells a unit of the stars' way is cut into, and in what share of them a star stands.
const starCells: f32 = 110.0;
const starShare: f32 = 0.035;
