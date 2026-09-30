// Water on the board, materials of the composer's shader: the sea's waves, swell and surf
// (sea.wgsl), water running down its slope (stream.wgsl), both laid over their tile here. They
// read the weather's clouds for the sky they reflect; the clouds' shadow on them is in lit already.

// water is what a glint lays over its tile: the sky reflected, the foam as bright as the tile in
// the light, the sun thrown back — of s, the sun, foam and sky a surface gives back, shine as much
// as it shines and lit of the sun reaching it.
fn water(shine: f32, lit: f32, s: vec3<f32>) -> vec4<f32> {
    let mirror = shine * s.z;
    let foam = s.y;
    let light = U.Ambience + U.SunColor * (U.SunStrength * lit * max(sunWay().z, 0.0));
    let rgb = overcastSky() * mirror * (1.0 - foam) + light * foam + U.SunColor * (shine * lit * s.x);
    return vec4<f32>(rgb, 1.0 - (1.0 - mirror) * (1.0 - foam));
}

// glintSharpness is how narrowly a glint gathers round the perfect reflection.
const glintSharpness: f32 = 400.0;
