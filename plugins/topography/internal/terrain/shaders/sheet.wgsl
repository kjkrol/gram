// What the sheet of what stands (terrain's photo.go, photo.wgsl) is drawn with: the light on a
// point facing n, a colour hazed and fogged as the ground is, and a corner's top.

// sheetLight is the light on a point facing n: the ambience in the shade, the sun as it meets n,
// in bands where the ground is drawn so, warmed as the hour warms it.
fn sheetLight(n: vec3<f32>) -> vec3<f32> {
    let sun = lightWay();
    var lit = 0.0;
    if sun.z > 0.0 {
        lit = 1.0;
    }
    var diffuse = max(dot(n, sun), 0.0);
    if U.Bands > 0.5 {
        diffuse = banded(diffuse, U.Bands);
    }
    var sunColor = U.LightColor;
    if U.Warmth > 0.0 {
        sunColor = mix(U.LightColor, U.LightColor * vec3<f32>(1.08, 0.92, 0.7), U.Warmth);
    }
    let shade = 1.0 - diffuse * lit;
    var light = U.Ambience * mix(vec3<f32>(1.0), U.ShadowTint, shade);
    return light + sunColor * diffuse * lit * U.LightStrength;
}

// sheetSeen is the colour rgb, premultiplied by a, at the world point p shaded by the clouds as
// cloud says, turned into the haze and the fog before the camera's sight as the ground is.
fn sheetSeen(rgb: vec3<f32>, a: f32, p: vec3<f32>, cloud: f32) -> vec4<f32> {
    var c = rgb;
    if cloud > 0.0 {
        c *= 1.0 - cloudShade(cloud);
    }
    // the fog over a colour premultiplied
    let fog = U.Fog * a;
    if U.Visibility > 0.0 {
        c = mix(c, fog, 1.0 - exp(-hazeDepth(p) / U.Visibility));
    }
    if U.Sight.w > 0.0 {
        c = mix(c, fog, smoothstep(U.Sight.z, U.Sight.w, distance(p.xy, U.Sight.xy)));
    }
    return vec4<f32>(c, a);
}

// banded is the light d quantised into bands steps, each edge softened a little, as the ground's.
fn banded(d: f32, bands: f32) -> f32 {
    let b = d * bands;
    let k = floor(b);
    return (k + smoothstep(0.42, 0.58, b - k)) / bands;
}

// hazeDepth is how much air lies between the eye and p, as the ground's.
fn hazeDepth(p: vec3<f32>) -> f32 {
    if U.HazeUp <= 0.0 {
        return distance(U.Eye, p);
    }
    return max(U.HazeUp - p.z, 0.0) / max(-U.HazeWay.z, 0.05);
}

// sheetTop is the top the corner at among the tops (the fourth image) stands at, and 1, or 0
// twice where nothing stands near it.
fn sheetTop(at: vec2<f32>) -> vec2<f32> {
    let t = imageSrc3At(imageSrc3Origin() + at + 0.5);
    if t.b < 0.5 {
        return vec2<f32>(0.0);
    }
    let bits = u32(t.r * 255.0 + 0.5) | (u32(t.g * 255.0 + 0.5) << 8u);
    return vec2<f32>(unpack2x16float(bits).x, 1.0);
}
