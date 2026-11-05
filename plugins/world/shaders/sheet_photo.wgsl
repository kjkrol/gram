// What stands in the sectors of a round photographed from above (world's sheet_gpu.go) into a
// darkroom U.SheetTarget texels large, each sector in a slot of its own for its albedo and in one
// from x U.SheetSplit on for its relief, an instance a thing in a slot: its middle (x, y) and half
// its length and width (z, w); its sprite's corners on the atlas, the first image; its altitude,
// its height, 1 where it lies flat, and the way it is headed in radians; how far its sector's
// texels on the sheet, U.SheetPx texels a world unit, lie from its slot's (x, y), the most its
// sector's things reach up or down (z) and its order among them (w); its slot's texels
// (x0, y0, x1, y1). A quad over its length along that way and its width across, a texel at least,
// laid within its slot alone; drawn against a depth, primed, so the highest top at a texel alone is
// laid there: in the albedo the sprite's colour, premultiplied, where it covers half the texel or
// more, in the relief the top's height a float16 (its low byte red, its high byte green) and the
// way it faces as (n+1)/2 in blue and alpha. A thing lying is level at its top; one standing a
// dome, its altitude at the corners, its top in the middle. The depth is the top over the span, the
// later in order winning a tie. An instance lying flat by 2 is the backdrop: it covers its slot
// under every thing, clear in the albedo and facing up at no height in the relief.

struct Photo {
    @builtin(position) @invariant clip: vec4<f32>,
    // from -1 to 1 along the way it is headed and across it
    @location(0) local: vec2<f32>,
    @location(1) uv: vec2<f32>,
    @location(2) @interpolate(flat) sprite: vec4<f32>,
    @location(3) @interpolate(flat) z: vec4<f32>,
    @location(4) @interpolate(flat) half: vec2<f32>,
    // the span and the order
    @location(5) @interpolate(flat) order: vec2<f32>,
    @location(6) @interpolate(flat) slot: vec4<f32>,
}

@vertex
fn vs_main(
    @builtin(vertex_index) vid: u32,
    @location(0) shape: vec4<f32>,
    @location(1) sprite: vec4<f32>,
    @location(2) z: vec4<f32>,
    @location(3) place: vec4<f32>,
    @location(4) slot: vec4<f32>,
) -> Photo {
    var across = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var down = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    let k = vid % 6u;
    let f = vec2<f32>(across[k], down[k]);
    let l = f * 2.0 - 1.0;
    let half = max(shape.zw, vec2<f32>(0.5 / U.SheetPx));
    let way = vec2<f32>(cos(z.w), sin(z.w));
    let side = vec2<f32>(-way.y, way.x);
    let p = shape.xy + way * (l.x * half.x) + side * (l.y * half.y);
    var t = p * U.SheetPx - place.xy;
    if z.z > 1.5 {
        t = mix(slot.xy, slot.zw, f);
    }
    var out: Photo;
    out.clip = vec4<f32>(t.x / U.SheetTarget.x * 2.0 - 1.0, 1.0 - t.y / U.SheetTarget.y * 2.0, 0.0,
        1.0);
    out.local = l;
    out.uv = mix(sprite.xy, sprite.zw, f);
    out.sprite = sprite;
    out.z = z;
    out.half = half;
    out.order = place.zw;
    out.slot = slot;
    return out;
}

struct Laid {
    @location(0) color: vec4<f32>,
    @builtin(frag_depth) depth: f32,
}

// the share of a texel a sprite must cover to be laid there
const covering: f32 = 0.5;

// the backdrop's depth, under every thing's
const backdropDepth: f32 = 0.01;

// fs_prime lays the depth of the highest top at each texel alone.
@fragment
fn fs_prime(ph: Photo) -> Laid {
    // sampled before anything is discarded: the derivatives want every pixel round
    let c = covered(ph);
    let backdrop = ph.z.z > 1.5;
    if outside(ph) || (!backdrop && c.a < covering) {
        discard;
    }
    var depth = backdropDepth;
    if !backdrop {
        depth = depthOf(topOf(ph).w, ph.order);
    }
    return Laid(vec4<f32>(0.0), depth);
}

@fragment
fn fs_main(ph: Photo) -> Laid {
    let c = covered(ph);
    let backdrop = ph.z.z > 1.5;
    if outside(ph) || (!backdrop && c.a < covering) {
        discard;
    }
    if backdrop {
        var none = vec4<f32>(0.0);
        if ph.slot.x >= U.SheetSplit {
            none = vec4<f32>(0.0, 0.0, 128.0 / 255.0, 128.0 / 255.0);
        }
        return Laid(none, backdropDepth);
    }
    let t = topOf(ph);
    var out = Laid(c, depthOf(t.w, ph.order));
    if ph.slot.x >= U.SheetSplit {
        let bits = pack2x16float(vec2<f32>(t.w, 0.0));
        out.color = vec4<f32>(f32(bits & 0xffu) / 255.0, f32((bits >> 8u) & 0xffu) / 255.0,
            t.xy * 0.5 + 0.5);
    }
    return out;
}

// outside reports whether the texel ph lies at is outside its sector's slot.
fn outside(ph: Photo) -> bool {
    let p = ph.clip.xy;
    return p.x < ph.slot.x || p.y < ph.slot.y || p.x >= ph.slot.z || p.y >= ph.slot.w;
}

// covered is the sprite's colour over the texel, premultiplied: the mean of four samples across it,
// held within the sprite.
fn covered(ph: Photo) -> vec4<f32> {
    let dx = dpdx(ph.uv) * 0.25;
    let dy = dpdy(ph.uv) * 0.25;
    let lo = min(ph.sprite.xy, ph.sprite.zw) + 0.5;
    let hi = max(ph.sprite.xy, ph.sprite.zw) - 0.5;
    var c = imageSrc0Linear(clamp(ph.uv - dx - dy, lo, hi));
    c += imageSrc0Linear(clamp(ph.uv + dx - dy, lo, hi));
    c += imageSrc0Linear(clamp(ph.uv - dx + dy, lo, hi));
    c += imageSrc0Linear(clamp(ph.uv + dx + dy, lo, hi));
    return c * 0.25;
}

// topOf is the way the thing's top faces over the point ph lies at, and its height (w).
fn topOf(ph: Photo) -> vec4<f32> {
    if ph.z.z > 0.5 {
        return vec4<f32>(0.0, 0.0, 1.0, ph.z.x + ph.z.y);
    }
    let l = ph.local;
    let h = ph.z.x + ph.z.y * (1.0 - dot(l, l) * 0.5);
    // the dome's rise a world unit along the way it is headed and across it
    let along = -ph.z.y * l.x / ph.half.x;
    let across = -ph.z.y * l.y / ph.half.y;
    let way = vec2<f32>(cos(ph.z.w), sin(ph.z.w));
    let g = way * along + vec2<f32>(-way.y, way.x) * across;
    return vec4<f32>(normalize(vec3<f32>(-g.x, -g.y, 1.0)), h);
}

// depthOf is the depth of a top h high within a span and an order (x, y), the order breaking a
// tie: nearer the higher.
fn depthOf(h: f32, order: vec2<f32>) -> f32 {
    return clamp(0.5 + 0.4 * clamp(h / order.x, -1.0, 1.0) + order.y * 1e-6, 0.0, 1.0);
}
