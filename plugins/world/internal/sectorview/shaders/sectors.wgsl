// The world's sectors laid over the ground (the world's sectors shown): every pixel finds the
// ground point drawn there from the depth the ground left (depthAt, through U.Unproject, the
// viewport U.ViewSize pixels from U.ViewAt on the target), round the world where U.World.zw says
// it wraps (its size U.World.xy), nothing past an edge it does not, then the sector it lies in —
// U.Sectors: their side, world units, how many across and down — and that sector's colour in the
// first image, a texel a sector. The sector's edge is U.Line.x pixels wide at U.Line.z of the
// colour, its inside at U.Line.y; it lies at the ground's depth.

struct Out {
    @builtin(position) clip: vec4<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> Out {
    // one triangle over the whole target
    var corners = array<vec2<f32>, 3>(
        vec2<f32>(-1.0, -1.0), vec2<f32>(3.0, -1.0), vec2<f32>(-1.0, 3.0),
    );
    var o: Out;
    o.clip = vec4<f32>(corners[vid], 0.0, 1.0);
    return o;
}

struct Laid {
    @location(0) color: vec4<f32>,
    @builtin(frag_depth) depth: f32,
}

@fragment
fn fs_main(@builtin(position) frag: vec4<f32>) -> Laid {
    let depth = depthAt(frag.xy);
    let px = frag.xy - U.ViewAt;
    let ndc = vec2<f32>(px.x / U.ViewSize.x * 2.0 - 1.0, 1.0 - px.y / U.ViewSize.y * 2.0);
    let w = U.Unproject * vec4<f32>(ndc, depth, 1.0);
    var p = w.xy / w.w;
    let size = U.World.xy;
    var outside = false;
    if U.World.z > 0.5 {
        p.x = p.x - size.x * floor(p.x / size.x);
    } else {
        outside = p.x < 0.0 || p.x > size.x;
    }
    if U.World.w > 0.5 {
        p.y = p.y - size.y * floor(p.y / size.y);
    } else {
        outside = outside || p.y < 0.0 || p.y > size.y;
    }
    let side = U.Sectors.x;
    let cell = clamp(floor(p / side), vec2<f32>(0.0), U.Sectors.yz - 1.0);
    let tint = imageSrc0At(imageSrc0Origin() + cell + 0.5);
    // how far the sector's nearest edge lies, in pixels, as the ground runs under the screen
    let edge = min(p - cell * side, min((cell + 1.0) * side, size) - p);
    let step = max(fwidth(p), vec2<f32>(1e-6));
    let pixels = min(edge.x / step.x, edge.y / step.y);
    // far off, where a pixel spans much of a sector, the edges would fill it: they fade out
    let blur = smoothstep(0.02, 0.1, max(step.x, step.y) / side);
    let width = U.Line.x * 0.5;
    let line = (1.0 - smoothstep(width - 0.5, width + 0.5, pixels)) * (1.0 - blur);
    let a = mix(U.Line.y, U.Line.z, line) * tint.a;
    if depth <= 0.0 || outside || a < 0.002 {
        discard; // nothing drawn there, the sky; past the world's edge; or a sector not shown
    }
    return Laid(vec4<f32>(tint.rgb * a, a), depth);
}
