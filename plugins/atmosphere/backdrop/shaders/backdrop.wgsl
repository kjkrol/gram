// The sky through a perspective: one triangle over the whole viewport, U.SkyView pixels, or with
// the clouds on, U.SkyPieces pieces across it skyPiece pixels wide, how the clouds look worked out
// at their corners (cloudsLook) and smoothed between; every pixel looking along its own line of
// sight (U.LookDir + U.LookDX·x + U.LookDY·y): paler at the horizon (U.SkyHorizon), deeper
// overhead (U.SkyOverhead), the stars as bright as U.StarsOn — the scattered ones here where
// U.ScatteredOn, the real ones drawn before and shining through it where U.OverStars — the moon's
// disc round U.MoonAt, U.MoonRadius wide, lit from U.MoonLight, the sun's glow (U.SunHalo) and disc
// (U.SunDisc) round U.SunAt, U.SunRadius wide, and the clouds over them where U.CloudsOn, their
// noise looked up in the tile of it baked into image0 (air.BakeTile).

struct Sky {
    @builtin(position) clip: vec4<f32>,
    @location(0) px: vec2<f32>,
    @location(1) look: vec4<f32>,
}

const skyPiece: f32 = 16.0;

// vs_main lays the corner vid of a triangle reaching past the viewport's every side, or of a piece.
@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> Sky {
    var s: Sky;
    if U.SkyPieces < 0.5 {
        let c = vec2<f32>(f32((vid << 1u) & 2u), f32(vid & 2u)); // (0, 0), (2, 0), (0, 2)
        s.clip = vec4<f32>(c.x * 2.0 - 1.0, 1.0 - c.y * 2.0, 0.0, 1.0);
        s.px = c * U.SkyView;
        return s;
    }
    var across = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var down = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    let n = vid / 6u;
    let pieces = u32(U.SkyPieces);
    let corner = vec2<f32>(f32(n % pieces) + across[vid % 6u], f32(n / pieces) + down[vid % 6u]);
    s.px = min(corner * skyPiece, U.SkyView);
    s.clip = vec4<f32>(s.px.x / U.SkyView.x * 2.0 - 1.0, 1.0 - s.px.y / U.SkyView.y * 2.0, 0.0, 1.0);
    s.look = cloudsLook(s.px);
    return s;
}

// fs_main is the sky at the pixel: its colour for how far up it looks, the stars, the moon and the
// sun over it and the clouds over all, as a premultiplied sun and clouds are laid over what is
// under them.
@fragment
fn fs_main(s: Sky) -> @location(0) vec4<f32> {
    let d = U.LookDir + U.LookDX * s.px.x + U.LookDY * s.px.y;
    let way = d / max(length(d), 1e-6);
    let pixel = max(length(dpdx(way)), length(dpdy(way))); // radians a pixel spans here
    let up = clamp(way.z, 0.0, 1.0);
    let air = mix(U.SkyHorizon, U.SkyOverhead, sqrt(up));
    // what shines together: the sky's own light, the scattered stars, the moon's glow, the sun's
    // halo; and what hides whatever lies behind it, the real stars drawn before among it,
    // premultiplied: the moon's disc, the sun's, the clouds
    var add = air;
    var front = vec4<f32>(0.0);
    if U.ScatteredOn > 0.5 && U.StarsOn > 0.0 && way.z > 0.0 {
        add += starsAt(way, pixel) * U.StarsOn * smoothstep(0.0, 0.12, way.z);
    }
    if U.MoonRadius > 0.0 {
        let q = (s.px - U.MoonAt) / U.MoonRadius;
        let r = length(q);
        add += U.MoonFace.rgb * U.MoonFace.a * moonGlow * exp(-max(r - 1.0, 0.0) * moonGlowFall);
        if r < 1.0 {
            // the face a half ball towards the eye, lit as the sun stands to it, a little light on
            // its dark side, which shows the sky's colour otherwise
            let n = vec3<f32>(q, sqrt(1.0 - r * r));
            let lit = smoothstep(-0.03, 0.08, dot(n, U.MoonLight));
            let face = U.MoonFace.rgb * moonAlbedo(q) * (earthshine + (1.0 - earthshine) * lit);
            let a = (1.0 - smoothstep(1.0 - 1.5 / U.MoonRadius, 1.0, r)) * U.MoonFace.a;
            front = vec4<f32>(mix(air, face, max(lit, earthshine * U.StarsOn)) * a, a);
        }
    }
    if U.SunRadius > 0.0 {
        // out from the disc's edge, in its radii: the light scattered round it in the sun's colour,
        // a bright halo close and a faint one wide, then the disc itself, white hot, reddened low
        let d = distance(s.px, U.SunAt);
        let out = max(d / U.SunRadius - 1.0, 0.0);
        add += U.SunHalo * (sunHalo * exp(-out * sunHaloFall) + sunGlare * exp(-out * sunGlareFall)) * U.SunDisc.a;
        let a = (1.0 - smoothstep(U.SunRadius - 1.5, U.SunRadius, d)) * U.SunDisc.a;
        front = vec4<f32>(U.SunDisc.rgb * a, a) + front * (1.0 - a);
    }
    if U.CloudsOn > 0.5 {
        // where the line of sight meets the layer, and how much of it a pixel spans there
        let t = (U.CloudHeight - U.EyeAt.z) / max(d.z, 1e-3);
        let hit = U.EyeAt.xy + d.xy * t;
        let span = max(length(dpdx(hit)), length(dpdy(hit)));
        if d.z > skyLift && t > 0.0 {
            let lod = cloudTileLevel(span, cloudTile * cloudSize);
            let torn = skyTile(hit, lod, cloudTile * cloudSize).r;
            var core = 0.0;
            if U.Billow > 0.0 {
                core = skyTile(hit, cloudTileLevel(span, heapTile * heapSize), heapTile * heapSize).g;
            }
            let c = skyClouds(cloudFromTile(torn, core), cloudTileSoft * lod, s.look, air);
            front = c + front * (1.0 - c.a);
        }
    }
    // half a step of the screen's colours either way, so a dark gradient shows no bands; over the
    // stars, as much of them hidden as the front covers
    let dither = fract(52.9829189 * fract(dot(s.px, vec2<f32>(0.06711056, 0.00583715))));
    let rgb = front.rgb + (1.0 - front.a) * add + (dither - 0.5) / 255.0;
    if U.OverStars > 0.5 {
        return vec4<f32>(rgb, front.a);
    }
    return vec4<f32>(rgb, 1.0);
}

// starsAt is the light of the stars along the unit way w, a pixel spanning pixel radians there:
// the way turned into the stars' own (U.StarX, U.StarY, U.StarZ, round the pole), a star in one
// cell of starCells in a few, a pixel or two wide, most faint, a few bright, twinkling.
fn starsAt(w: vec3<f32>, pixel: f32) -> vec3<f32> {
    let s = vec3<f32>(dot(U.StarX, w), dot(U.StarY, w), dot(U.StarZ, w));
    let cell = floor(s * starCells);
    let h = starHash(cell);
    if h.w < 1.0 - starShare {
        return vec3<f32>(0.0);
    }
    let bright = pow((h.w - (1.0 - starShare)) / starShare, 4.0);
    let at = normalize((cell + 0.25 + 0.5 * h.xyz) / starCells);
    let off = length(at - s) / max(pixel, 1e-6); // pixels from it
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

// The stars: how many cells a unit of their way is cut into, and in what share of them a star
// stands.
const starCells: f32 = 110.0;
const starShare: f32 = 0.035;

// The moon: its glow round it, how bright and how fast it falls off, out in radii; how much light
// its dark side takes; how bright its face is to its picture's grey.
const moonGlow: f32 = 0.08;
const moonGlowFall: f32 = 1.2;
const earthshine: f32 = 0.08;
const moonBright: f32 = 1.7;

// moonAlbedo is how bright the moon's face is at q, in its radii from the middle of its disc on
// the screen (y down): its picture in image1 (atmosphere's moonImage), north turned to
// U.MoonNorth, four of its pixels blended where the disc is drawn smaller than the picture.
fn moonAlbedo(q: vec2<f32>) -> f32 {
    let north = U.MoonNorth;
    let east = vec2<f32>(-north.y, north.x);
    let uv = vec2<f32>(0.5 + 0.5 * dot(q, east), 0.5 - 0.5 * dot(q, north));
    let size = f32(textureDimensions(image1).x);
    let o = max(0.5 * size / U.MoonRadius - 1.0, 0.0) * 0.25 / size; // how far apart, when smaller
    var a = textureSampleLevel(image1, linear, uv, 0.0).r;
    if o > 0.0 {
        a = 0.25 * (textureSampleLevel(image1, linear, uv + vec2<f32>(o, o), 0.0).r + textureSampleLevel(image1, linear, uv + vec2<f32>(-o, o), 0.0).r +
            textureSampleLevel(image1, linear, uv + vec2<f32>(o, -o), 0.0).r + textureSampleLevel(image1, linear, uv + vec2<f32>(-o, -o), 0.0).r);
    }
    return a * moonBright;
}
// The sun's halo: how bright close by and how fast it falls off, out in radii of the disc; its glare
// over the sky round it, the same.
const sunHalo: f32 = 0.55;
const sunHaloFall: f32 = 1.4;
const sunGlare: f32 = 0.16;
const sunGlareFall: f32 = 0.22;

// skyTile is the clouds' tile in image0 at p on level lod, its period period: the two levels round
// it blended.
fn skyTile(p: vec2<f32>, lod: f32, period: f32) -> vec2<f32> {
    let low = floor(lod);
    let dims = vec2<f32>(textureDimensions(image0));
    var n = textureSampleLevel(image0, linear, cloudTileSpot(p, low, period) / dims, 0.0).rg;
    if lod > low {
        n = mix(n, textureSampleLevel(image0, linear, cloudTileSpot(p, low + 1.0, period) / dims, 0.0).rg, lod - low);
    }
    return n;
}
