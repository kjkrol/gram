// Sprites drawn as instances (render.Sprites), six vertices each: the world rectangle they lie over
// (rect), the part of the atlas over it (src, the first image's pixels), the light it is drawn in
// (light.rgb) — as the composer draws a plain sprite — and the angle it is turned by about its
// middle on the screen (light.w, radians, 0 east against the clock with y growing down). The
// rectangle lies on the screen as the camera lays it (U.ViewProj, a viewport U.ViewSize large):
// the screen box round its corners; over a world wrapping where U.Wrap says, the copy of it
// nearest the camera's window (U.Window: where it begins, how far it reaches), and a world back
// past each seam: six vertices a copy. Deep (U.Deep), a sprite lies at its height (lift.x) for the
// depth test, and what is all but clear of it is left out.

struct Piece {
    @builtin(position) clip: vec4<f32>,
    @location(0) src: vec2<f32>,
    @location(1) @interpolate(flat) light: vec4<f32>,
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
) -> Piece {
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
    let a = onScreen(r.x, r.y);
    let b = onScreen(r.z, r.y);
    let c = onScreen(r.x, r.w);
    let d = onScreen(r.z, r.w);
    let lo = min(min(a, b), min(c, d));
    let hi = max(max(a, b), max(c, d));
    let centre = (lo + hi) * 0.5;
    let off = (k - vec2<f32>(0.5, 0.5)) * (hi - lo);
    let cs = cos(light.w);
    let sn = sin(light.w);
    let px = centre + vec2<f32>(off.x * cs + off.y * sn, off.y * cs - off.x * sn);
    var depth = 0.0;
    if U.Deep > 0.5 {
        let at = U.ViewProj * vec4<f32>((r.xy + r.zw) * 0.5, lift.x, 1.0);
        depth = at.z / at.w;
    }
    var p: Piece;
    let ndc = vec2<f32>(px.x / U.ViewSize.x * 2.0 - 1.0, 1.0 - px.y / U.ViewSize.y * 2.0);
    p.clip = vec4<f32>(ndc, depth, 1.0);
    p.src = mix(src.xy, src.zw, k);
    p.light = light;
    return p;
}

@fragment
fn fs_main(p: Piece) -> @location(0) vec4<f32> {
    let c = imageSrc0At(p.src) * vec4<f32>(p.light.rgb, 1.0);
    if U.Deep > 0.5 && c.a < 0.5 / 255.0 {
        discard; // writing no depth where nothing shows
    }
    return c;
}
