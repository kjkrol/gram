// The sun over the world, in the composer's shader: U.Sun is the way towards it, U.SunStrength how
// much it lights a surface facing it square on, U.SunColor and U.SkyColor the colours of its light
// and of the sky, U.Ambience the light every surface gets from the sky. sky.Sun.Frame sets them.

// sunWay is the way towards the sun, of length 1.
fn sunWay() -> vec3<f32> {
    return U.Sun / max(length(U.Sun), 1e-6);
}
