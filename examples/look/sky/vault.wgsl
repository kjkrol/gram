// The valley's vault: the air turned turquoise, deepening into U.VaultNight as the sun sets, a band
// shimmering along the stars' equator at night; at the horizon the air's own, where the fog meets
// it.
fn SkyVault(s: SkyAt) -> vec3<f32> {
    let turquoise = vec3<f32>(0.35, 0.85, 0.82);
    let band = vec3<f32>(0.2, 0.12, 0.34);
    let night = 1.0 - smoothstep(-0.3, 0.05, s.sun.z);
    let up = smoothstep(0.0, 0.35, s.way.z);
    let light = dot(s.air, vec3<f32>(0.3, 0.59, 0.11));
    let day = mix(s.air, turquoise * light / 0.7, up * 0.85);
    var c = mix(day, U.VaultNight * (0.55 + 0.45 * up), night * up);
    let w = s.starWay;
    let along = atan2(w.y, w.x);
    let shimmer = 0.55 + 0.45 * sin(along * 9.0 + U.Clock * 0.8) *
        sin(along * 4.0 - U.Clock * 0.5 + w.z * 20.0);
    c += band * exp(-w.z * w.z * 10.0) * shimmer * night * smoothstep(0.0, 0.06, s.way.z);
    return c;
}
