// Sprites drawn as instances (render.Sprites), six vertices each: the world rectangle they lie over
// (rect), the part of the atlas over it (src, the first image's pixels), the light it is drawn in
// (light.rgb) — as the composer draws a plain sprite — and the angle it is turned by about its
// middle in the world (light.w, radians, 0 east against the clock with y growing down). Each
// corner lies on the screen where the camera lays it (U.ViewProj, a viewport U.ViewSize large),
// turning with a view turned; over a world wrapping where U.Wrap says, the copy of it
// nearest the camera's window (U.Window: where it begins, how far it reaches), and a world back
// past each seam: six vertices a copy. Deep (U.Deep), a sprite lies at its height (lift.x) for the
// depth test, and what is all but clear of it is left out. A sprite the screen turns other than
// by quarters is read smooth and its border fades over a pixel; any other texel for texel.

struct Piece {
    @builtin(position) clip: vec4<f32>,
    @location(0) src: vec2<f32>,
    @location(1) @interpolate(flat) light: vec4<f32>,
    // the sprite's rectangle on the atlas, and 1 where it is read smooth
    @location(2) @interpolate(flat) part: vec4<f32>,
    @location(3) @interpolate(flat) smooth: f32,
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
    let centre = (r.xy + r.zw) * 0.5;
    let size = r.zw - r.xy;
    let cs = cos(light.w);
    let sn = sin(light.w);
    // the world's way along the sprite's width and its height, and the screen's step a unit along
    let across = vec2<f32>(cs, -sn);
    let down = vec2<f32>(sn, cs);
    let mid = onScreen(centre.x, centre.y);
    let sx = onScreen(centre.x + across.x, centre.y + across.y) - mid;
    let sy = onScreen(centre.x + down.x, centre.y + down.y) - mid;
    let smooth = min(abs(sx.x), abs(sx.y)) > 1e-3 * max(abs(sx.x), abs(sx.y));
    // a pixel's margin, in world units, for the border to fade in
    var grow = vec2<f32>(0.0);
    if smooth {
        grow = vec2<f32>(1.0 / max(length(sx), 1e-6), 1.0 / max(length(sy), 1e-6));
    }
    let side = k * 2.0 - vec2<f32>(1.0);
    let off = side * (size * 0.5 + grow);
    let corner = centre + off.x * across + off.y * down;
    let px = onScreen(corner.x, corner.y);
    var depth = 0.0;
    if U.Deep > 0.5 {
        let at = U.ViewProj * vec4<f32>((r.xy + r.zw) * 0.5, lift.x, 1.0);
        depth = at.z / at.w;
    }
    var p: Piece;
    let ndc = vec2<f32>(px.x / U.ViewSize.x * 2.0 - 1.0, 1.0 - px.y / U.ViewSize.y * 2.0);
    p.clip = vec4<f32>(ndc, depth, 1.0);
    p.src = mix(src.xy, src.zw, vec2<f32>(0.5) + off / max(size, vec2<f32>(1e-6)));
    p.light = light;
    p.part = src;
    p.smooth = select(0.0, 1.0, smooth);
    return p;
}

@fragment
fn fs_main(p: Piece) -> @location(0) vec4<f32> {
    let lo = min(p.part.xy, p.part.zw);
    let hi = max(p.part.xy, p.part.zw);
    // how far inside the sprite's edge, in pixels; asked of every pixel, as derivatives must be
    let inside = min(p.src - lo, hi - p.src) / max(fwidth(p.src), vec2<f32>(1e-6));
    var c = imageSrc0At(p.src);
    if p.smooth > 0.5 {
        let cover = clamp(0.5 + min(inside.x, inside.y), 0.0, 1.0);
        c = imageSrc0Linear(clamp(p.src, lo + 0.5, hi - 0.5)) * cover;
    }
    c *= vec4<f32>(p.light.rgb, 1.0);
    if U.Deep > 0.5 && c.a < 0.5 / 255.0 {
        discard; // writing no depth where nothing shows
    }
    return c;
}
