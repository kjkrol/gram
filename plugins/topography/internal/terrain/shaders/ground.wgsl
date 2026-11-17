// What the ground's shaders share with the sea's over it (mesh.wgsl, sea.wgsl): the board
// painted flat as a level reads it, the light falling on the ground and the air before it.

// levelAlbedo is albedo at p from the board painted flat as the level reads it, U.AlbedoPx
// pixels a cell: shrunk at the strategic levels, so far off a cell's pattern does not shimmer.
fn levelAlbedo(p: vec2<f32>) -> vec3<f32> {
    return imageSrc1Linear(imageSrc1Origin() + p / U.Cell * U.AlbedoPx).rgb;
}

// lightOn is the light falling at p on the ground facing n, lit of the sun (way sun) reaching
// it, the lamps' too where it takes them (taken, render.Taking): the sky's tinted towards
// U.ShadowTint as much as the sun misses it, and the sun's, warmed by U.Warmth.
fn lightOn(n: vec3<f32>, sun: vec3<f32>, p: vec3<f32>, lit: f32, taken: f32) -> vec3<f32> {
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
    light += sunColor * diffuse * lit * U.LightStrength;
    if LEVEL <= 3u && takes(taken, takesLamps) {
        light += lampLightAt(p);
    }
    return light;
}

// how many pixels a cell spans at least for the shore's swell and surf to be read
const shoreFrom: f32 = 3.0;

// banded is the light d quantised into bands steps, each edge softened a little.
fn banded(d: f32, bands: f32) -> f32 {
    let b = d * bands;
    let k = floor(b);
    return (k + smoothstep(0.42, 0.58, b - k)) / bands;
}

// hazeDepth is how much air lies between the eye and the ground point p, world units: its
// distance from U.Eye, or seen from above (U.HazeUp) the air over p up to that height, along the
// lines of sight U.HazeWay.
fn hazeDepth(p: vec3<f32>) -> f32 {
    if U.HazeUp <= 0.0 {
        return distance(U.Eye, p);
    }
    return max(U.HazeUp - p.z, 0.0) / max(-U.HazeWay.z, 0.05);
}
