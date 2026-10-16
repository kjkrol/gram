// The clouds' shadow on the ground: each layer's where the way from the ground to the sun crosses
// it (U.CloudBases how high each hangs), with U.CloudsOneLayer straight under the one, the high
// one's only with U.CloudsHighShade, as air.Weather.Shade has it.

// CloudShadow is the clouds' shadow over a sprite lying on the ground at the pixel's point of the
// world, how faint the sprite under it is in red and how it fades or blends in custom; it darkens
// the sprite as much as the clouds over it take of the sun.
fn CloudShadow(p: vec2<f32>, red: f32, frac: f32, custom: vec4<f32>) -> vec4<f32> {
    var weight = red * faded(custom);
    if custom.w > 50000.0 {
        weight = blended(red, custom.w);
    }
    return vec4<f32>(0.0, 0.0, 0.0, cloudShade(cloudsShadeAt(vec3<f32>(p, 0.0), U.Pixel)) * weight);
}

// CloudOver is a layer of clouds over a point of the ground: its noise and how soft its edge is
// seen there (cloudLowAt), and its air mass (cloudLowMass); none, its cover 0.
struct CloudOver {
    field: vec2<f32>,
    mass: vec4<f32>,
}

// cloudsShadeAt is how much of the sun the clouds take at the ground point p, a pixel pixel world
// units wide there: 0 in the clear up to 1 under the thickest of both layers.
fn cloudsShadeAt(p: vec3<f32>, pixel: f32) -> f32 {
    let low = cloudLowOver(p, pixel);
    let high = cloudHighOver(p, pixel);
    return 1.0 - (1.0 - cloudOverCover(low)) * (1.0 - cloudOverCover(high));
}

// cloudLowOver is the low clouds where the way from the ground point p to the sun crosses them,
// none over ground higher than they hang; cloudHighOver the high clouds, none without
// U.CloudsHighShade.
fn cloudLowOver(p: vec3<f32>, pixel: f32) -> CloudOver {
    var c = CloudOver(vec2<f32>(0.0), vec4<f32>(0.0));
    let h = U.CloudBases.x - p.z;
    if h <= 0.0 {
        return c;
    }
    let at = p.xy + cloudSunSlant() * h;
    c.mass = cloudLowMass(airAt(at));
    if c.mass.x > 0.0 {
        c.field = cloudLowAt(at, pixel, c.mass.y);
    }
    return c;
}

fn cloudHighOver(p: vec3<f32>, pixel: f32) -> CloudOver {
    var c = CloudOver(vec2<f32>(0.0), vec4<f32>(0.0));
    let h = cloudHighHeight(p.xy) - p.z; // the sheet's over the point, near enough
    if h <= 0.0 || U.CloudsHighShade < 0.5 {
        return c;
    }
    let at = p.xy + cloudSunSlant() * h;
    c.mass = cloudHighHere(cloudHighMass(airAt(at)), at);
    if c.mass.x > 0.0 {
        c.field = cloudHighAt(at, pixel);
    }
    return c;
}

// cloudOverCover is how thick the layer c is.
fn cloudOverCover(c: CloudOver) -> f32 {
    if c.mass.x <= 0.0 {
        return 0.0;
    }
    return cloudCoverSoft(c.field.x, c.field.y, c.mass);
}

// cloudSunSlant is how far across the way to the sun goes for every world unit up, as far as at
// cloudLowSun for a sun lower than that; with U.CloudsOneLayer none, the shadows straight under.
fn cloudSunSlant() -> vec2<f32> {
    if U.CloudsOneLayer > 0.5 {
        return vec2<f32>(0.0);
    }
    let sun = sunWay();
    return sun.xy / max(sun.z, cloudLowSun);
}

const cloudLowSun: f32 = 0.25;

// cloudShade is how much the clouds darken the ground under a cover of cover: as much as they take
// of the sun, more under a strong one.
fn cloudShade(cover: f32) -> f32 {
    return cloudDark * cover * clamp(U.SunStrength * 1.4, 0.0, 1.0);
}

// cloudDark is how much of the sun's light the thickest cloud takes.
const cloudDark: f32 = 0.5;
