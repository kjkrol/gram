// The clouds on the sky, a material: every pixel looks along its own line of sight (U.EyeAt,
// U.LookDir + U.LookDX·x + U.LookDY·y, camera.RayField) up to the layer U.CloudHeight up and takes
// the clouds' noise there; U.Visibility is how far one sees through the air, 0 without end.

// Clouds is the clouds on the sky at the pixel whose screen position custom holds: lit from above
// by the sun and grey underneath the more of the sky they cover, turning to the Fog as far off as
// the layer lies in the air; nothing looking under the layer.
fn Clouds(p: vec2<f32>, red: f32, frac: f32, custom: vec4<f32>) -> vec4<f32> {
    let d = U.LookDir + U.LookDX * custom.x + U.LookDY * custom.y;
    if d.z <= skyLift {
        return vec4<f32>(0.0);
    }
    let t = (U.CloudHeight - U.EyeAt.z) / d.z;
    if t <= 0.0 {
        return vec4<f32>(0.0);
    }
    let cover = cloudCover(cloudField(U.EyeAt.xy + d.xy * t));
    var haze = 0.0;
    if U.Visibility > 0.0 {
        haze = 1.0 - exp(-t * length(d) / U.Visibility);
    }
    let lit = clamp(U.SunStrength * 1.4, 0.0, 1.0);
    let sunlit = mix(vec3<f32>(0.62), vec3<f32>(1.0), lit) * mix(vec3<f32>(1.0), U.SunColor, 0.6);
    let under = mix(1.0, 0.72, clamp(U.Cover, 0.0, 1.0)); // the undersides greyer under a heavier sky
    let rgb = mix(sunlit * under, U.Fog, haze);
    let a = cover * (1.0 - haze * 0.85);
    return vec4<f32>(rgb * a, a);
}

// skyLift is how far up a line of sight must look to reach the cloud layer at all: along the
// horizon the air alone shows.
const skyLift: f32 = 0.02;

// overcastSky is the sky's colour under the clouds covering it, as air.Overcast has it.
fn overcastSky() -> vec3<f32> {
    let grey = dot(U.SkyColor, vec3<f32>(0.3, 0.5, 0.2)) * 0.85;
    return mix(U.SkyColor, vec3<f32>(grey), clamp(U.Cover, 0.0, 1.0) * 0.8);
}
