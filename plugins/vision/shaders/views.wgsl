// The views laid over what the frame drew (vision's views), an instance an observer: its eye (x,
// y, the eye's level) and reach; its facing, half its angle and its block of the baked sight (the
// third image, U.Spokes by U.Rings texels an observer, sighted.wgsl); the rectangle of the viewport
// its cone may cover, pixels. Every pixel of it finds the ground point drawn there from the depth
// the ground left (depthAt, through U.Unproject, the viewport U.ViewSize pixels from U.ViewAt on
// the target) — the views come after the ground, before what stands on it — or, over level ground
// (U.FlatGround), where its line of sight meets the ground at 0 — and, inside the cone,
// veils it in U.ShadowColor as far as the observer does not see it; the cone's edge is stroked in
// U.ConeColor, a pixel wide.

struct View {
    @builtin(position) clip: vec4<f32>,
    @location(0) @interpolate(flat) at: vec4<f32>,
    @location(1) @interpolate(flat) look: vec4<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vid: u32, @location(0) at: vec4<f32>, @location(1) look: vec4<f32>, @location(2) rect: vec4<f32>) -> View {
    var across = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var down = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    let px = mix(rect.xy, rect.zw, vec2<f32>(across[vid], down[vid]));
    var v: View;
    v.clip = vec4<f32>(px.x / U.ViewSize.x * 2.0 - 1.0, 1.0 - px.y / U.ViewSize.y * 2.0, 0.0, 1.0);
    v.at = at;
    v.look = look;
    return v;
}

@fragment
fn fs_main(@builtin(position) frag: vec4<f32>, v: View) -> @location(0) vec4<f32> {
    let px = frag.xy - U.ViewAt;
    let ndc = vec2<f32>(px.x / U.ViewSize.x * 2.0 - 1.0, 1.0 - px.y / U.ViewSize.y * 2.0);
    var p: vec3<f32>;
    if U.FlatGround > 0.5 {
        // the line of sight through two depths, down to the ground at 0
        let far = U.Unproject * vec4<f32>(ndc, 0.25, 1.0);
        let near = U.Unproject * vec4<f32>(ndc, 1.0, 1.0);
        let a = far.xyz / far.w;
        let b = near.xyz / near.w;
        if a.z >= b.z - 1e-6 {
            discard; // it never comes down to the ground
        }
        p = mix(a, b, a.z / (a.z - b.z));
    } else {
        let depth = depthAt(frag.xy);
        if depth <= 0.0 {
            discard; // nothing drawn there: the sky
        }
        let w = U.Unproject * vec4<f32>(ndc, depth, 1.0);
        p = w.xyz / w.w;
        let off = p.xy - U.Eye.xy;
        p.z += U.Bend * dot(off, off); // as high as it stands, not as far as it is sunk
    }
    let rel = p.xy - v.at.xy;
    let dist = length(rel);
    let half = v.look.y;
    var turn = atan2(rel.y, rel.x) - v.look.x; // from the facing, -π to π
    turn -= 6.2831853 * floor((turn + 3.1415927) / 6.2831853);
    let pixel = select(U.PixelSpan.x, U.PixelSpan.y * distance(U.Eye, p), U.Perspective > 0.5); // world units a pixel spans
    // the stroke along the cone's edges and its reach
    var edge = 1e9;
    if abs(turn) <= half {
        edge = abs(v.at.w - dist);
    }
    if dist <= v.at.w {
        edge = min(edge, abs(dist * sin(abs(turn) - half)) + select(0.0, 1e9, abs(turn) - half > 1.5707963));
    }
    let stroke = 1.0 - smoothstep(0.5, 1.5, edge / max(pixel, 1e-4));
    var c = vec4<f32>(0.0);
    if abs(turn) <= half && dist <= v.at.w {
        let t = vec2<f32>((turn + half) / (2.0 * half) * U.Spokes, v.look.z * U.Rings + clamp(dist / v.at.w * U.Rings, 0.5, U.Rings - 0.5));
        c = U.ShadowColor * (1.0 - imageSrc2Linear(imageSrc2Origin() + t).r);
    }
    let a = U.ConeColor.a * stroke;
    c = vec4<f32>(U.ConeColor.rgb * stroke + c.rgb * (1.0 - a), a + c.a * (1.0 - a));
    if c.a < 0.002 {
        discard;
    }
    return c;
}
