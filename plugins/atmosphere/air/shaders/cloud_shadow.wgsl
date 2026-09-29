// The clouds' shadow on the ground, a material: it falls straight under them, so it does not jump
// as the sun steps on.

// CloudShadow is the clouds' shadow over a sprite: the clouds' noise worked out at the pixel's
// point of the world, how faint the sprite under it is in red and how it fades or blends in
// custom; it darkens the sprite as much as the clouds over it take of the sun.
fn CloudShadow(p: vec2<f32>, red: f32, frac: f32, custom: vec4<f32>) -> vec4<f32> {
    var weight = red * faded(custom);
    if custom.w > 50000.0 {
        weight = blended(red, custom.w);
    }
    return vec4<f32>(0.0, 0.0, 0.0, cloudShade(cloudCover(cloudField(p))) * weight);
}

// cloudShade is how much the clouds darken the ground under a cover of cover: as much as they take
// of the sun, more under a strong one.
fn cloudShade(cover: f32) -> f32 {
    return cloudDark * cover * clamp(U.SunStrength * 1.4, 0.0, 1.0);
}

// cloudDark is how much of the sun's light the thickest cloud takes.
const cloudDark: f32 = 0.5;
