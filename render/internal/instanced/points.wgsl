// Points drawn as instances (render.Sprites.Point), six vertices each: the middle of a box in the
// world and its size (place), the part of the atlas over it (src, the first image's pixels), the
// light it is drawn in and the height it is seen at (light), its fog, the shade it takes, the
// sun's share of its light and the height its depth is taken at (lift). On the screen it is the
// box as large as it shows there, a pixel at least each way, upright to the screen, where the
// camera lays its middle (U.ViewProj, a viewport U.ViewSize large, sunk by U.Bend as the ground
// is); a box showing less than a pixel is drawn as much less opaque, so the many such add up to
// what they cover. Over a world wrapping where U.Wrap says, the copy nearest the camera's window
// (U.Window), and a world back past each seam: six vertices a copy. Deep (U.Deep), it lies at its
// depth's height for the depth test, and what is all but clear of it is left out.

struct Point {
    @builtin(position) clip: vec4<f32>,
    @location(0) src: vec2<f32>,
    @location(1) @interpolate(flat) light: vec4<f32>,
    @location(2) @interpolate(flat) fog: f32,
    @location(3) @interpolate(flat) world: vec2<f32>,
    // the shade it takes, the sun's share of its light, its depth's height and how far up it fades
    @location(4) @interpolate(flat) shade: vec4<f32>,
    // the share of each of its pixels it covers
    @location(5) @interpolate(flat) cover: f32,
}

fn sunk(p: vec3<f32>) -> vec4<f32> {
    let off = p.xy - U.Eye.xy;
    return U.ViewProj * vec4<f32>(p.xy, p.z - U.Bend * dot(off, off), 1.0);
}

fn pixel(c: vec4<f32>) -> vec2<f32> {
    let ndc = c.xy / c.w;
    return vec2<f32>((ndc.x + 1.0) * 0.5 * U.ViewSize.x, (1.0 - ndc.y) * 0.5 * U.ViewSize.y);
}

// where x lies from the window beginning at from and reaching span, round a world size long: the
// nearer of the copies ahead and behind
fn windowed(x: f32, from: f32, size: f32, span: f32) -> f32 {
    let ahead = x - from - size * floor((x - from) / size);
    let behind = ahead - size;
    if -behind < max(ahead - span, 0.0) {
        return behind;
    }
    return ahead;
}

@vertex
fn vs_main(
    @builtin(vertex_index) vid: u32,
    @location(0) place: vec4<f32>,
    @location(1) src: vec4<f32>,
    @location(2) light: vec4<f32>,
    @location(3) lift: vec4<f32>,
) -> Point {
    var across = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var down = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    let k = vec2<f32>(across[vid % 6u], down[vid % 6u]);
    let copy = vid / 6u;
    let back = vec2<f32>(f32(copy % u32(1.0 + U.Wrap.z)), f32(copy / u32(1.0 + U.Wrap.z)));
    var m = place.xy;
    if U.Wrap.z > 0.5 {
        m.x = U.Window.x + windowed(m.x, U.Window.x, U.Wrap.x, U.Window.z);
    }
    if U.Wrap.w > 0.5 {
        m.y = U.Window.y + windowed(m.y, U.Window.y, U.Wrap.y, U.Window.w);
    }
    m -= back * U.Wrap.xy;
    let at = sunk(vec3<f32>(m, light.w));
    var p: Point;
    if at.w <= 0.0 {
        p.clip = vec4<f32>(0.0, 0.0, -1.0, 1.0); // behind the eye: none of it
        return p;
    }
    let mid = pixel(at);
    // the screen's step a world unit along x and along y
    let sx = pixel(sunk(vec3<f32>(m.x + 1.0, m.y, light.w))) - mid;
    let sy = pixel(sunk(vec3<f32>(m.x, m.y + 1.0, light.w))) - mid;
    let size = vec2<f32>(abs(place.z * sx.x) + abs(place.w * sy.x),
        abs(place.z * sx.y) + abs(place.w * sy.y));
    let shown = max(size, vec2<f32>(1.0));
    let px = mid + (k - vec2<f32>(0.5)) * shown;
    var depth = 0.0;
    if U.Deep > 0.5 {
        let d = sunk(vec3<f32>(m, lift.w));
        depth = d.z / d.w;
    }
    let ndc = vec2<f32>(px.x / U.ViewSize.x * 2.0 - 1.0, 1.0 - px.y / U.ViewSize.y * 2.0);
    p.clip = vec4<f32>(ndc, depth, 1.0);
    // read at its middle while under a pixel, spread to its corners by two
    let spread = clamp(size - vec2<f32>(1.0), vec2<f32>(0.0), vec2<f32>(1.0));
    p.src = mix(src.xy, src.zw, vec2<f32>(0.5) + (k - vec2<f32>(0.5)) * spread);
    p.light = light;
    p.fog = lift.x;
    p.world = m;
    p.shade = vec4<f32>(lift.y, lift.z, lift.w, 0.5 * min(place.z, place.w));
    p.cover = min(size.x, 1.0) * min(size.y, 1.0);
    return p;
}

@fragment
fn fs_main(p: Point) -> @location(0) vec4<f32> {
    var c = imageSrc0At(p.src) * vec4<f32>(p.light.rgb, 1.0);
    if p.shade.y > 0.0 {
        var ground = groundHeightAt(p.world);
        if ground == groundNone {
            ground = 0.0;
        }
        let over = p.shade.z - ground;
        var lit = 1.0;
        if takes(p.shade.x, takesRelief) {
            lit = min(lit, groundShadeAt(p.world, over, p.shade.w));
        }
        if takes(p.shade.x, takesBlocks) {
            lit = min(lit, standingShadeAt(p.world, over, p.shade.w));
        }
        c = vec4<f32>(c.rgb * (1.0 - p.shade.y * (1.0 - lit)), c.a);
    }
    c = vec4<f32>(mix(c.rgb, U.Fog * c.a, p.fog), c.a) * p.cover;
    if U.Deep > 0.5 && c.a < 0.5 / 255.0 {
        discard; // writing no depth where nothing shows
    }
    return c;
}
