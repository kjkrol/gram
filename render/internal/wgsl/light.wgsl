// The frame's light, in every shader: U.LightWay is the way towards the distant light,
// U.LightStrength how much it lights a surface facing it square on, U.LightColor and
// U.AmbientColor the colours of its light and of the ambient light, U.Ambience the light every
// surface gets anyway; U.Lamp0At .. U.Lamp7At are where each lamp in the picture stands (x, y, z)
// and how far it reaches (w), U.Lamp0Color .. U.Lamp7Color its light at full strength,
// U.LampCount how many shine.

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
