// What render.Sprites has uncovered (Sprites.Uncover), drawn before the cutouts into the depth
// what gives way reads (render.Target.Uncovered), the screen's colours left as they are. Each
// piece, an instance as sprites.wgsl's, is a disc round its middle reaching as far as its longer
// side, written as the library's uncoveredAt says of its depth at the pixels uncoveredOpen
// leaves open.

struct Bared {
    @builtin(position) clip: vec4<f32>,
    // where the pixel lies off the piece's middle, 1 at the disc's rim
    @location(0) off: vec2<f32>,
}

fn onScreen(x: f32, y: f32) -> vec2<f32> {
    let c = U.ViewProj * vec4<f32>(x, y, 0.0, 1.0);
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
    @location(0) rect: vec4<f32>,
    @location(1) src: vec4<f32>,
    @location(2) light: vec4<f32>,
    @location(3) lift: vec4<f32>,
) -> Bared {
    var across = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var down = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    let k = vec2<f32>(across[vid % 6u], down[vid % 6u]);
    let copy = vid / 6u;
    let back = vec2<f32>(f32(copy % u32(1.0 + U.Wrap.z)), f32(copy / u32(1.0 + U.Wrap.z)));
    var r = rect;
    if U.Wrap.z > 0.5 {
        r.x = U.Window.x + windowed(rect.x, U.Window.x, U.Wrap.x, U.Window.z);
        r.z = r.x + (rect.z - rect.x);
    }
    if U.Wrap.w > 0.5 {
        r.y = U.Window.y + windowed(rect.y, U.Window.y, U.Wrap.y, U.Window.w);
        r.w = r.y + (rect.w - rect.y);
    }
    let shift = -back * U.Wrap.xy;
    r += vec4<f32>(shift, shift);
    let centre = (r.xy + r.zw) * 0.5;
    let reach = max(abs(r.z - r.x), abs(r.w - r.y));
    let side = k * 2.0 - vec2<f32>(1.0);
    let px = onScreen(centre.x + side.x * reach, centre.y + side.y * reach);
    let at = U.ViewProj * vec4<f32>(centre, lift.x, 1.0);
    var p: Bared;
    let ndc = vec2<f32>(px.x / U.ViewSize.x * 2.0 - 1.0, 1.0 - px.y / U.ViewSize.y * 2.0);
    p.clip = vec4<f32>(ndc, uncoveredAt(at.z / at.w), 1.0);
    p.off = side;
    return p;
}

@fragment
fn fs_main(p: Bared) -> @location(0) vec4<f32> {
    if !uncoveredOpen(p.off, p.clip.xy) {
        discard;
    }
    return vec4<f32>(0.0);
}
