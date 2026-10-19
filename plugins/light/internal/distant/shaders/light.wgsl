// The distant light over the world, in the composer's shader: U.LightWay is the way towards it,
// U.LightStrength how much it lights a surface facing it square on, U.LightColor and
// U.AmbientColor the colours of its light and of the ambient light, U.Ambience the light every
// surface gets anyway. light.Distant.Frame sets them.

// lightWay is the way towards the distant light, of length 1.
fn lightWay() -> vec3<f32> {
    return U.LightWay / max(length(U.LightWay), 1e-6);
}
