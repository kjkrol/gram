// The Earth's sun: out from the disc's edge, in its radii, the light scattered round it in its
// colour, a bright halo close and a faint one wide; then the disc itself, white hot, reddened low.
fn SkySun(s: SkyAt, d: SkyDisc) -> SkyShine {
    var o = SkyShine();
    let r = distance(s.px, d.at);
    let out = max(r / d.radius - 1.0, 0.0);
    o.glow = d.glow * (sunHalo * exp(-out * sunHaloFall) + sunGlare * exp(-out * sunGlareFall)) *
        d.shown;
    let a = (1.0 - smoothstep(d.radius - 1.5, d.radius, r)) * d.shown;
    o.front = vec4<f32>(d.color * a, a);
    return o;
}

// The sun's halo: how bright close by and how fast it falls off, out in radii of the disc; its
// glare over the sky round it, the same.
const sunHalo: f32 = 0.55;
const sunHaloFall: f32 = 1.4;
const sunGlare: f32 = 0.16;
const sunGlareFall: f32 = 0.22;
