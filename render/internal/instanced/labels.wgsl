// Text drawn as instances (render.Labels), six vertices a glyph: the point of the world it stands
// by, the height it is seen at and the height its depth is taken at (place), where its top-left
// lies from where the point shows and its size, in shown pixels (glyph), its top-left on the
// font's sheet (src.xy) and its colour (premultiplied). On the screen the point lies where the
// camera lays it (U.ViewProj, a viewport U.ViewSize large, sunk by U.Bend as the ground is), the
// glyph U.Scale camera pixels a shown one, its top-left on a whole pixel. Over a world wrapping
// where U.Wrap says, the copy nearest the camera's window (U.Window), and a world back past each
// seam: six vertices a copy. Deep (U.Deep), it stands upright on its point for the depth test,
// from its depth's height up as far as it shows over the point, and what is all but clear of it
// is left out. A badge is laid as a glyph, its shape (src.z, a disc 1, a diamond 2) of radius
// src.x within its square worked out a pixel at a time, filled in its colour within a rim src.w
// shown pixels wide in rim.

struct Glyph {
    @builtin(position) clip: vec4<f32>,
    // a glyph's place on the sheet; a badge's from its middle, in shown pixels
    @location(0) src: vec2<f32>,
    @location(1) @interpolate(flat) colour: vec4<f32>,
    // a badge's shape, none for a glyph, its radius and its rim's width; its rim's colour
    @location(2) @interpolate(flat) badge: vec4<f32>,
    @location(3) @interpolate(flat) rim: vec4<f32>,
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
    @location(1) glyph: vec4<f32>,
    @location(2) src: vec4<f32>,
    @location(3) colour: vec4<f32>,
    @location(4) rim: vec4<f32>,
) -> Glyph {
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
    let at = sunk(vec3<f32>(m, place.z));
    var g: Glyph;
    if at.w <= 0.0 {
        g.clip = vec4<f32>(0.0, 0.0, -1.0, 1.0); // behind the eye: none of it
        return g;
    }
    // on whole pixels as Font.Draw lays a glyph, rounding half up
    let corner = floor(pixel(at) + glyph.xy * U.Scale + vec2<f32>(0.5));
    let px = corner + k * glyph.zw * U.Scale;
    var depth = 0.0;
    if U.Deep > 0.5 {
        // as deep as it would lie standing upright on its point, as high over its depth's height
        // as it shows over the point: what lies behind the point hides none of it
        let up = sunk(vec3<f32>(m, place.z + 1.0));
        let rise = pixel(at).y - pixel(up).y;
        var high = 0.0;
        if up.w > 0.0 && rise > 0.0001 {
            high = max(pixel(at).y - px.y, 0.0) / rise;
        }
        var d = sunk(vec3<f32>(m, place.w + high));
        if d.w <= 0.0 {
            d = sunk(vec3<f32>(m, place.w));
        }
        depth = clamp(d.z / d.w, 0.0, 1.0);
    }
    let ndc = vec2<f32>(px.x / U.ViewSize.x * 2.0 - 1.0, 1.0 - px.y / U.ViewSize.y * 2.0);
    g.clip = vec4<f32>(ndc, depth, 1.0);
    g.src = src.xy + k * glyph.zw;
    g.badge = vec4<f32>(src.z, src.x, src.w, 0.0);
    if src.z > 0.5 {
        g.src = (k - vec2<f32>(0.5)) * glyph.zw;
    }
    g.colour = colour;
    g.rim = rim;
    return g;
}

// how far within the edge of a badge of shape and radius r p lies, in shown pixels from its
// middle: a disc, else a diamond
fn within(p: vec2<f32>, shape: f32, r: f32) -> f32 {
    if shape < 1.5 {
        return r - length(p);
    }
    return (r - abs(p.x) - abs(p.y)) * 0.70710678;
}

@fragment
fn fs_main(g: Glyph) -> @location(0) vec4<f32> {
    var c = imageSrc0At(g.src);
    if U.Scale != 1.0 {
        c = imageSrc0Linear(g.src);
    }
    c = c * g.colour;
    if g.badge.x > 0.5 {
        let e = within(g.src, g.badge.x, g.badge.y);
        let filled = clamp((e - g.badge.z) * U.Scale + 0.5, 0.0, 1.0);
        c = mix(g.rim, g.colour, filled) * clamp(e * U.Scale + 0.5, 0.0, 1.0);
    }
    if U.Deep > 0.5 && c.a < 0.5 / 255.0 {
        discard; // writing no depth where nothing shows
    }
    return c;
}
