// What the bake's passes share (painter's bake_gpu.go). A cell's block in the cells' image (image
// 2, 4×2 pixels a cell, cells.go) says what the bake reads of it; the sprites are the sheet's
// (image 0), found through their rectangles (image 1, two pixels a sprite, 256 a row: left and
// top, then right and bottom, a coordinate in two bytes). The board is U.PaintCells.x cells
// across and U.PaintCells.y down, running on along an axis U.PaintWrap says wraps.

// a cell's flags (cells.go)
const flagUnder: u32 = 1u;
const flagRaised: u32 = 2u;
const flagRuns: u32 = 4u;
const flagMixes: u32 = 8u;

// folded is the board's own cell c stands for, along an axis that wraps; z 0 off the board.
fn folded(c: vec2<f32>) -> vec3<f32> {
    var f = c;
    if U.PaintWrap.x > 0.5 {
        f.x = f.x - U.PaintCells.x * floor(f.x / U.PaintCells.x);
    }
    if U.PaintWrap.y > 0.5 {
        f.y = f.y - U.PaintCells.y * floor(f.y / U.PaintCells.y);
    }
    let on = f.x >= 0.0 && f.y >= 0.0 && f.x < U.PaintCells.x && f.y < U.PaintCells.y;
    return vec3<f32>(f, select(0.0, 1.0, on));
}

// texelOf is the pixel (dx, dy) of the board's cell b's block.
fn texelOf(b: vec2<f32>, dx: f32, dy: f32) -> vec4<f32> {
    return imageSrc2At(imageSrc2Origin() + b * vec2<f32>(4.0, 2.0) + vec2<f32>(dx, dy) + 0.5);
}

// byteIn is the byte k, 0 to 3, of the pixel t.
fn byteIn(t: vec4<f32>, k: i32) -> u32 {
    return u32(t[k] * 255.0 + 0.5);
}

// Top is what the bake reads of a cell to lay its ground: its kind's sprite, its flags, its
// spread (0 to a half) and its shine.
struct Top {
    sprite: u32,
    flags: u32,
    spread: f32,
    shine: f32,
}

// topOf is the cell b's Top.
fn topOf(b: vec2<f32>) -> Top {
    let t0 = texelOf(b, 0.0, 0.0);
    let t1 = texelOf(b, 1.0, 0.0);
    var out: Top;
    out.sprite = byteIn(t0, 0) | (byteIn(t0, 1) << 8u);
    out.flags = byteIn(t0, 2);
    out.spread = f32(byteIn(t0, 3)) / 510.0;
    out.shine = f32(byteIn(t1, 0)) / 255.0;
    return out;
}

// land reports whether a cell of top runs into the cells round it: no lying under them, a
// spread, no standing at a height.
fn land(top: Top) -> bool {
    return (top.flags & (flagUnder | flagRaised)) == 0u && top.spread > 0.0;
}

// rectOf is the sheet's rectangle of sprite id: left, top, right, bottom.
fn rectOf(id: u32) -> vec4<f32> {
    let at = imageSrc1Origin() + vec2<f32>(f32((id % 256u) * 2u), f32(id / 256u)) + 0.5;
    let a = imageSrc1At(at) * 255.0 + 0.5;
    let b = imageSrc1At(at + vec2<f32>(1.0, 0.0)) * 255.0 + 0.5;
    return vec4<f32>(floor(a.x) + floor(a.y) * 256.0, floor(a.z) + floor(a.w) * 256.0,
        floor(b.x) + floor(b.y) * 256.0, floor(b.z) + floor(b.w) * 256.0);
}

// sampled is the texel of sprite id a quad laid over a sprite samples at uv across it: half a
// texel in from the rectangle's edges.
fn sampled(id: u32, uv: vec2<f32>) -> vec4<f32> {
    let r = rectOf(id);
    return imageSrc0At(r.xy + 0.5 + uv * (r.zw - r.xy - 1.0));
}

// blendAt is a weight given at a quarter's corners (top-left, top-right, bottom-left,
// bottom-right) at s across and down it, as its two triangles from the top-left and from the
// bottom-right corner carry it.
fn blendAt(w: vec4<f32>, s: vec2<f32>) -> f32 {
    if s.x + s.y <= 1.0 {
        return w.x + s.x * (w.y - w.x) + s.y * (w.z - w.x);
    }
    return w.w + (1.0 - s.x) * (w.z - w.w) + (1.0 - s.y) * (w.y - w.w);
}

// shows is how much of a sprite blended over a weight w shows, fading in over soft either side
// of a half (render.Frame.SpriteBlend).
fn shows(w: f32, soft: f32) -> f32 {
    let mark = 100000.0 + clamp(soft, 0.01, 0.5); // as the composer's vertex carries it
    let s = mark - 100000.0;
    return smoothstep(0.5 - s, 0.5 + s, w);
}

// over is a laid over b, both premultiplied.
fn over(a: vec4<f32>, b: vec4<f32>) -> vec4<f32> {
    return a + b * (1.0 - a.a);
}
