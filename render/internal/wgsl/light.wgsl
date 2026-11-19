// The frame's light, in every shader: U.LightWay is the way towards the distant light,
// U.LightStrength how much it lights a surface facing it square on, U.LightColor and
// U.AmbientColor the colours of its light and of the ambient light, U.Ambience the light every
// surface gets anyway; U.Lamp0At .. U.Lamp7At are where each lamp in the picture stands (x, y, z)
// and how far it reaches (w), U.Lamp0Color .. U.Lamp7Color its light at full strength,
// U.LampCount how many shine.

// shadowReach is how many times its caster's height a shadow reaches at most, whatever casts it
// (light.ShadowReach).
const shadowReach: f32 = 6.0;

// lightWay is the way towards the distant light, of length 1.
fn lightWay() -> vec3<f32> {
    return U.LightWay / max(length(U.LightWay), 1e-6);
}

// lampAt is the lamp k's place and reach.
fn lampAt(k: u32) -> vec4<f32> {
    switch k {
        case 0u: { return U.Lamp0At; }
        case 1u: { return U.Lamp1At; }
        case 2u: { return U.Lamp2At; }
        case 3u: { return U.Lamp3At; }
        case 4u: { return U.Lamp4At; }
        case 5u: { return U.Lamp5At; }
        case 6u: { return U.Lamp6At; }
        default: { return U.Lamp7At; }
    }
}

// lampColor is the lamp k's light at full strength.
fn lampColor(k: u32) -> vec3<f32> {
    switch k {
        case 0u: { return U.Lamp0Color; }
        case 1u: { return U.Lamp1Color; }
        case 2u: { return U.Lamp2Color; }
        case 3u: { return U.Lamp3Color; }
        case 4u: { return U.Lamp4Color; }
        case 5u: { return U.Lamp5Color; }
        case 6u: { return U.Lamp6Color; }
        default: { return U.Lamp7Color; }
    }
}

// lampLightAt is the lamps' light at the world point p, each fading by the square of the share of
// its reach left.
fn lampLightAt(p: vec3<f32>) -> vec3<f32> {
    var sum = vec3<f32>(0.0);
    let n = u32(U.LampCount);
    for (var k = 0u; k < 8u; k = k + 1u) {
        if k >= n {
            break;
        }
        let at = lampAt(k);
        let d = distance(p, at.xyz);
        if at.w > 0.0 && d < at.w {
            let left = 1.0 - d / at.w;
            sum = sum + lampColor(k) * left * left;
        }
    }
    return sum;
}

// The roofs over the world (render.Shelter), a row each of the frame's Shelters image, a float32
// a texel in its four bytes: the shelter's floor, its shares of the sky's light and of the sun's
// thrown round, its lamp's light and whether it burns by day, how many openings it has, then each
// opening's line from A to B and its bottom and top over the floor. A thing drawn under one
// carries its row and one more past the bits of its Taking and of givesWay (shelterOf).
const shelterOpenings: u32 = 32u;
const shelterBit: u32 = 512u;

fn shelterFloat(row: u32, k: u32) -> f32 {
    return bitcast<f32>(pack4x8unorm(textureLoad(Shelters, vec2<i32>(i32(k), i32(row)), 0)));
}

// shelterOf is the shelter the float t of a Taking's bits names, its row and one more: 0 in the
// open.
fn shelterOf(t: f32) -> u32 {
    return u32(t + 0.5) / shelterBit;
}

// sunThrough is 1 where the way from p toward the distant light, way, leaves the shelter of row
// by one of its n openings over its floor, else 0: a wall or the roof stops it. A shelter is
// taken for convex: what leaves it by one opening crosses no other wall.
fn sunThrough(p: vec3<f32>, row: u32, floor: f32, n: u32, way: vec3<f32>) -> f32 {
    for (var k = 0u; k < shelterOpenings; k = k + 1u) {
        if k >= n {
            break;
        }
        let at = 8u + 6u * k;
        let a = vec2<f32>(shelterFloat(row, at), shelterFloat(row, at + 1u));
        let run = vec2<f32>(shelterFloat(row, at + 2u), shelterFloat(row, at + 3u)) - a;
        let den = way.x * run.y - way.y * run.x;
        if abs(den) < 1e-6 {
            continue;
        }
        let to = a - p.xy;
        let t = (to.x * run.y - to.y * run.x) / den;
        let on = (to.x * way.y - to.y * way.x) / den;
        if t <= 0.0 || on < 0.0 || on > 1.0 {
            continue;
        }
        let h = p.z + t * way.z - floor;
        if h >= shelterFloat(row, at + 4u) && h <= shelterFloat(row, at + 5u) {
            return 1.0;
        }
    }
    return 0.0;
}

// shelteredLight is the light on a surface facing up at the world point p under the shelter of
// row, lamps the frame's lamps' on it: the distant light's where its way leaves the shelter by
// an opening, the shelter's share of the sky's and of the distant light's thrown round, and its
// own lamp as the sky's light goes (as a lit window, the moon's light as strong as a low sun's),
// or by day too.
fn shelteredLight(p: vec3<f32>, row: u32, lamps: vec3<f32>) -> vec3<f32> {
    let floor = shelterFloat(row, 0u);
    let way = lightWay();
    var sun = vec3<f32>(0.0);
    if way.z > 0.0 {
        let through = sunThrough(p, row, floor, u32(shelterFloat(row, 7u)), way);
        sun = U.LightColor * U.LightStrength * way.z * (through + shelterFloat(row, 2u));
    }
    let lamp = vec3<f32>(shelterFloat(row, 3u), shelterFloat(row, 4u), shelterFloat(row, 5u));
    var burns = 1.0 - smoothstep(0.08, 0.2, dot(U.Ambience, vec3<f32>(0.3, 0.59, 0.11)));
    if shelterFloat(row, 6u) > 0.5 {
        burns = 1.0;
    }
    return lamps + U.Ambience * shelterFloat(row, 1u) + sun + lamp * burns;
}
