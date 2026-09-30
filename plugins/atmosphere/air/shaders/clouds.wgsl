// The clouds on the sky, as the sky draws them: every pixel looks along its own line of sight
// (U.EyeAt, U.LookDir + U.LookDX·x + U.LookDY·y, camera.RayField) up to the layer U.CloudHeight
// up and takes the clouds' noise there; U.Visibility is how far one sees through the air, 0
// without end.

// cloudsLook is how the clouds along the line of sight of the screen pixel px look, as skyClouds
// reads it: how hazed the layer is there, how square their undersides face the eye, how near the
// sun it looks and how much of the layer shows, none along the horizon; all 0 where it looks under
// the layer. It changes slowly: a few pixels work it out and the ones between have it smoothed.
fn cloudsLook(px: vec2<f32>) -> vec4<f32> {
    let d = U.LookDir + U.LookDX * px.x + U.LookDY * px.y;
    if d.z <= 0.0 {
        return vec4<f32>(0.0);
    }
    let t = (U.CloudHeight - U.EyeAt.z) / d.z;
    if t <= 0.0 {
        return vec4<f32>(0.0);
    }
    let view = d / length(d);
    var haze = 0.0;
    if U.Visibility > 0.0 {
        haze = 1.0 - exp(-t * length(d) / U.Visibility);
    }
    let toward = max(dot(view, sunWay()), 0.0);
    let t2 = toward * toward;
    return vec4<f32>(haze, smoothstep(0.1, 0.6, view.z), t2 * t2 * t2, smoothstep(skyLift, 3.0 * skyLift, view.z));
}

// skyClouds is the clouds of noise field on the sky, premultiplied, their edge softened soft
// either way, looking as look says (cloudsLook), over the sky behind them: lit by the sky and by
// the sun or the moon, as much as they shine, greyer underneath the more of the sky they cover,
// their thick middles darker seen from below and their thin edges shining towards the light, the
// more so the more heaped they are, turning to the sky behind as far off as the layer lies in the
// air; whole, hiding the stars, the moon and the sun behind them.
fn skyClouds(field: f32, soft: f32, look: vec4<f32>, behind: vec3<f32>) -> vec4<f32> {
    let cover = cloudCoverSoft(field, soft);
    if cover <= 0.0 {
        return vec4<f32>(0.0);
    }
    let lit = clamp(U.SunStrength * 1.4, 0.0, 1.0);
    let light = U.Ambience * cloudsSkyLit + U.SunColor * U.SunStrength * cloudsSunLit;
    let under = mix(1.0, 0.72, clamp(U.Cover, 0.0, 1.0)); // the undersides greyer under a heavier sky
    let heaped = clamp(U.Billow, 0.0, 1.0);
    let thick = cloudThick(field);
    let base = 1.0 - thick * thick * (0.1 + 0.25 * heaped) * mix(0.6, 1.0, look.y);
    let rim = look.z * (1.0 - thick) * lit * (0.3 + 0.5 * heaped);
    let rgb = mix(light * under * base + U.SunColor * rim, behind, look.x);
    let a = cover * look.w;
    return vec4<f32>(rgb * a, a);
}

// How much of the sky's light and of the sun's (or the moon's) a cloud gives back: all but white
// under the noon sun.
const cloudsSkyLit: f32 = 0.6;
const cloudsSunLit: f32 = 1.1;

// skyLift is how far up a line of sight must look to see the cloud layer at all, and three times
// as far to see all of it: along the horizon the air alone shows.
const skyLift: f32 = 0.02;

// overcastSky is the sky's colour under the clouds covering it, as air.Overcast has it.
fn overcastSky() -> vec3<f32> {
    let grey = dot(U.SkyColor, vec3<f32>(0.3, 0.5, 0.2)) * 0.85;
    return mix(U.SkyColor, vec3<f32>(grey), clamp(U.Cover, 0.0, 1.0) * 0.8);
}
