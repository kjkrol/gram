// The sky through a perspective: one triangle over the whole viewport, U.SkyView pixels, or with
// the clouds on, U.SkyPieces pieces across it skyPiece pixels wide, how the clouds look worked out
// at their corners (cloudsLook) and smoothed between; every pixel looking along its own line of
// sight (U.LookDir + U.LookDX·x + U.LookDY·y) at the parts of the sky (shaders/sky.wgsl): the
// vault, the stars added to it as bright as U.StarsOn (the real ones drawn before and shining
// through it where U.OverStars), the moon round U.MoonAt and the sun round U.SunAt, each where its
// radius is not 0, and the clouds over them where U.CloudsOn, the high ones and the low before
// them: a layer U.CloudsFlat says flat, its noise looked up in the frame's tile of it (CloudTile,
// air.Tile), the others with U.CloudBodies the heaps image2 holds (shaders/cumulus.wgsl).

struct Sky {
    @builtin(position) clip: vec4<f32>,
    @location(0) px: vec2<f32>,
    @location(1) look: vec4<f32>, // the low clouds', then the high clouds' (cloudsLook)
    @location(2) high: vec4<f32>,
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
    s.look = cloudsLook(s.px, U.CloudBases.x);
    s.high = cloudsLook(s.px, 0.5 * (cloudHighFloor() + U.CloudBases.y));
    return s;
}

// fs_main is the sky at the pixel: the vault, the stars and the glows added, the moon's disc, the
// sun's and the clouds laid over them, premultiplied.
@fragment
fn fs_main(v: Sky) -> @location(0) vec4<f32> {
    let d = U.LookDir + U.LookDX * v.px.x + U.LookDY * v.px.y;
    let way = d / max(length(d), 1e-6);
    var s: SkyAt;
    s.px = v.px;
    s.way = way;
    s.pixel = max(length(dpdx(way)), length(dpdy(way)));
    s.starWay = vec3<f32>(dot(U.StarX, way), dot(U.StarY, way), dot(U.StarZ, way));
    s.horizon = U.Fog;
    s.overhead = U.SkyOverhead;
    s.sun = U.SunWay;
    s.sunColor = U.SunTint;
    s.stars = U.StarsOn;
    s.air = skyAt(way.z);
    let vault = SkyVault(s);
    s.air = vault;
    var add = vault;
    var front = vec4<f32>(0.0);
    if U.StarsOn > 0.0 {
        add += SkyStars(s);
    }
    if U.MoonRadius > 0.0 {
        var disc: SkyDisc;
        disc.at = U.MoonAt;
        disc.radius = U.MoonRadius;
        disc.color = U.MoonFace.rgb;
        disc.shown = U.MoonFace.a;
        disc.glow = U.MoonFace.rgb;
        disc.lit = U.MoonLight;
        disc.north = U.MoonNorth;
        let moon = SkyMoon(s, disc);
        add += moon.glow;
        front = moon.front + front * (1.0 - moon.front.a);
    }
    if U.SunRadius > 0.0 {
        var disc: SkyDisc;
        disc.at = U.SunAt;
        disc.radius = U.SunRadius;
        disc.color = U.SunDisc.rgb;
        disc.shown = U.SunDisc.a;
        disc.glow = U.SunHalo;
        disc.lit = vec3<f32>(0.0, 0.0, 1.0);
        disc.north = vec2<f32>(0.0, -1.0);
        let sun = SkySun(s, disc);
        add += sun.glow;
        front = sun.front + front * (1.0 - sun.front.a);
    }
    if U.CloudsOn > 0.5 {
        if U.CloudsFlat.y > 0.5 {
            let high = skyLayer(d, U.CloudBases.y, false, v.high, vault);
            front = high + front * (1.0 - high.a);
        }
        if U.CloudBodies > 0.5 {
            let heaps = skyHeaps(v.px);
            front = heaps + front * (1.0 - heaps.a);
        }
        if U.CloudsFlat.x > 0.5 {
            let low = skyLayer(d, U.CloudBases.x, true, v.look, vault);
            front = low + front * (1.0 - low.a);
        }
    }
    // half a step of the screen's colours either way, so a dark gradient shows no bands; over the
    // stars, as much of them hidden as the front covers
    let dither = fract(52.9829189 * fract(dot(v.px, vec2<f32>(0.06711056, 0.00583715))));
    let rgb = front.rgb + (1.0 - front.a) * add + (dither - 0.5) / 255.0;
    if U.OverStars > 0.5 {
        return vec4<f32>(rgb, front.a);
    }
    return vec4<f32>(rgb, 1.0);
}

// skyLayer is a layer of clouds seen flat along d from the eye: the heaps if low, height up, else
// the sheets of shreds, each at its own height (cloudHighCross); looking as look says (cloudsLook)
// over the sky behind: premultiplied, none where d does not meet the layer.
fn skyLayer(d: vec3<f32>, height: f32, low: bool, look: vec4<f32>, behind: vec3<f32>) -> vec4<f32> {
    // where the line of sight meets the layer, and how much of it a pixel spans there
    var t = (height - U.EyeAt.z) / max(d.z, 1e-3);
    if !low {
        let band = cloudBodySpan(false);
        t = cloudHighCross(U.EyeAt, d, max((band.x - U.EyeAt.z) / max(d.z, 1e-3), 0.0),
            (band.y - U.EyeAt.z) / max(d.z, 1e-3));
    }
    let hit = U.EyeAt.xy + d.xy * t;
    let span = max(length(dpdx(hit)), length(dpdy(hit)));
    if d.z <= skyLift || t <= 0.0 {
        return vec4<f32>(0.0);
    }
    let air = airAt(hit);
    var m = cloudHighHere(cloudHighMass(air), hit);
    if low {
        m = cloudLowMass(air);
    }
    if m.x <= 0.0 {
        return vec4<f32>(0.0);
    }
    var c: vec2<f32>;
    if low {
        c = cloudLowAt(hit, span, m.y);
    } else {
        c = cloudHighAt(hit, span);
    }
    if low {
        return skyClouds(c.x, c.y, look, behind, m);
    }
    // the shreds thin wisps, thicker only in the middle
    return skyClouds(c.x, c.y, look, behind, m) * (skyThin * (0.5 + 0.5 * cloudThick(c.x, m)));
}

// How much of what lies behind the thickest shreds hide.
const skyThin: f32 = 0.55;

// skyHeaps is the heaps image2 holds at the view's pixel px, a coarse pixel's worth blurred: their
// steps put off at random a pixel showed as grain.
fn skyHeaps(px: vec2<f32>) -> vec4<f32> {
    let at = px / U.SkyView;
    let off = 0.5 / vec2<f32>(textureDimensions(image2));
    return 0.25 * (textureSampleLevel(image2, linear, at + vec2<f32>(off.x, off.y), 0.0) +
        textureSampleLevel(image2, linear, at + vec2<f32>(-off.x, off.y), 0.0) +
        textureSampleLevel(image2, linear, at + vec2<f32>(off.x, -off.y), 0.0) +
        textureSampleLevel(image2, linear, at + vec2<f32>(-off.x, -off.y), 0.0));
}
