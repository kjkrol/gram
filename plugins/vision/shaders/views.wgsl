// The views laid over what the frame drew (vision's views), an instance an observer: its eye (x,
// y, the eye's level) and reach; its facing, half its angle and its block of the baked sight (the
// third image, U.Spokes by U.Rings texels an observer, sighted.wgsl); the rectangle of the viewport
// its cone may cover, pixels. Every pixel of it finds the ground point drawn there from the depth
// the ground left (depthAt, through U.Unproject, the viewport U.ViewSize pixels from U.ViewAt on
// the target) — the views come after the ground, before what stands on it — or, over level ground
// (U.FlatGround), where its line of sight meets the ground at 0 — and, inside the cone,
// veils it in U.ShadowColor as far as the observer does not see it; the cone's edge is stroked in
// the instance's colour, a pixel wide, lying at the ground's depth.

struct View {
    @builtin(position) clip: vec4<f32>,
    @location(0) @interpolate(flat) at: vec4<f32>,
    @location(1) @interpolate(flat) look: vec4<f32>,
    @location(2) @interpolate(flat) color: vec4<f32>,
    @location(3) @interpolate(flat) field: vec4<f32>, // the vertical field: its tilt, half of it
}

@vertex
fn vs_main(@builtin(vertex_index) vid: u32, @location(0) at: vec4<f32>, @location(1) look: vec4<f32>, @location(2) rect: vec4<f32>, @location(3) color: vec4<f32>, @location(4) field: vec4<f32>) -> View {
    var across = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var down = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    let px = mix(rect.xy, rect.zw, vec2<f32>(across[vid], down[vid]));
    var v: View;
    v.clip = vec4<f32>(px.x / U.ViewSize.x * 2.0 - 1.0, 1.0 - px.y / U.ViewSize.y * 2.0, 0.0, 1.0);
    v.at = at;
    v.look = look;
    v.color = color;
    v.field = field;
    return v;
}

struct Laid {
    @location(0) color: vec4<f32>,
    @builtin(frag_depth) depth: f32,
}

@fragment
fn fs_main(v: View) -> Laid {
    let px = v.clip.xy - U.ViewAt;
    let ndc = vec2<f32>(px.x / U.ViewSize.x * 2.0 - 1.0, 1.0 - px.y / U.ViewSize.y * 2.0);
    var p: vec3<f32>;
    var lies = v.clip.z;
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
        let depth = depthAt(v.clip.xy);
        if depth <= 0.0 {
            discard; // nothing drawn there: the sky
        }
        lies = depth;
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
    // where the ground lies up or down from the eye against its vertical field: past its edge by
    // off radians; read for every pixel, so the derivatives below are taken everywhere
    let elev = atan2(p.z - v.at.z, dist);
    let off = abs(elev - v.field.x) - v.field.y;
    let fieldEdge = abs(off) / max(fwidth(off), 1e-6); // pixels from the field's edge
    let inField = v.field.y <= 0.0 || off <= 0.0;
    // the stroke along the cone's edges and its reach within the field, and along the field's
    // edges within the cone and its reach: one far edge, the nearer
    var edge = 1e9;
    if inField {
        if abs(turn) <= half {
            edge = abs(v.at.w - dist);
        }
        if dist <= v.at.w {
            edge = min(edge, abs(dist * sin(abs(turn) - half)) + select(0.0, 1e9, abs(turn) - half > 1.5707963));
        }
    }
    var stroke = 1.0 - smoothstep(0.5, 1.5, edge / max(pixel, 1e-4));
    if v.field.y > 0.0 && abs(turn) <= half && dist <= v.at.w {
        stroke = max(stroke, 1.0 - smoothstep(0.5, 1.5, fieldEdge));
    }
    var c = vec4<f32>(0.0);
    if abs(turn) <= half && dist <= v.at.w && inField {
        let t = vec2<f32>((turn + half) / (2.0 * half) * U.Spokes, v.look.z * U.Rings + clamp(dist / v.at.w * U.Rings, 0.5, U.Rings - 0.5));
        c = U.ShadowColor * (1.0 - imageSrc2Linear(imageSrc2Origin() + t).r);
    }
    let a = v.color.a * stroke;
    c = vec4<f32>(v.color.rgb * stroke + c.rgb * (1.0 - a), a + c.a * (1.0 - a));
    if c.a < 0.002 {
        discard;
    }
    return Laid(c, lies);
}
