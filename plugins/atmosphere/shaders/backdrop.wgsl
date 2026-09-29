// The sky through a perspective: one triangle over the whole viewport, U.SkyView pixels, every
// pixel looking along its own line of sight (U.LookDir + U.LookDX·x + U.LookDY·y): paler at the
// horizon (U.SkyHorizon), deeper overhead (U.SkyOverhead), the sun's glow and disc round U.SunAt,
// U.SunRadius wide, and the clouds (Clouds) over them where U.CloudsOn.

struct Sky {
    @builtin(position) clip: vec4<f32>,
    @location(0) px: vec2<f32>,
}

// vs_main lays the corner vid of a triangle reaching past the viewport's every side.
@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> Sky {
    let c = vec2<f32>(f32((vid << 1u) & 2u), f32(vid & 2u)); // (0, 0), (2, 0), (0, 2)
    var s: Sky;
    s.clip = vec4<f32>(c.x * 2.0 - 1.0, 1.0 - c.y * 2.0, 0.0, 1.0);
    s.px = c * U.SkyView;
    return s;
}

// fs_main is the sky at the pixel: its colour for how far up it looks, the sun over it and the
// clouds over both, as a premultiplied sun and clouds are laid over what is under them.
@fragment
fn fs_main(s: Sky) -> @location(0) vec4<f32> {
    let d = U.LookDir + U.LookDX * s.px.x + U.LookDY * s.px.y;
    let up = clamp(d.z / max(length(d), 1e-6), 0.0, 1.0);
    var rgb = mix(U.SkyHorizon, U.SkyOverhead, sqrt(up));
    if U.SunRadius > 0.0 {
        let r = distance(s.px, U.SunAt);
        let glow = 1.0 - smoothstep(U.SunRadius * sunGlow - 1.0, U.SunRadius * sunGlow, r);
        let disc = 1.0 - smoothstep(U.SunRadius - 1.0, U.SunRadius, r);
        rgb = U.SunDisc.rgb * glow + rgb * (1.0 - 0.25 * U.SunDisc.a * glow);
        rgb = U.SunDisc.rgb * disc + rgb * (1.0 - U.SunDisc.a * disc);
    }
    if U.CloudsOn > 0.5 {
        let c = Clouds(vec2<f32>(0.0), 1.0, 0.0, vec4<f32>(s.px, 0.0, 0.0));
        rgb = c.rgb + rgb * (1.0 - c.a);
    }
    return vec4<f32>(rgb, 1.0);
}

// sunGlow is how many times wider than the disc the sun's glow is (atmosphere.sunGlow).
const sunGlow: f32 = 3.0;
