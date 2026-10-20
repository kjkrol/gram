// The kinds' details: the fourth image is the board's kinds, U.Px pixels a cell, each pixel the
// detail its kind carries in red (painter.Kinds); GroundDetail the details baked (painter.Details),
// U.DetailPx square tiles under a header row, each bordered by a pixel of its far side, laid in
// as many a row as the sheet is wide; the header's pixel k - 1 holds tile k's cells over
// acrossScale. U.DetailPx is 0 without any.

const detailPitch = 2.0;
const acrossScale = 16.0;
// pixels a cell spans on the screen below which the detail has faded out, and from which it is
// whole: a tile's texels would shimmer with no mipmaps, so the ground falls back to its colour
const detailFar = 14.0;
const detailNear = 40.0;

// detailTile is the tile k's pixels (its top-left, inside the border) in GroundDetail.
fn detailTile(k: f32) -> vec2<f32> {
    let pitch = U.DetailPx + detailPitch;
    let cols = floor(f32(textureDimensions(GroundDetail).x) / pitch);
    let col = k - floor(k / cols) * cols;
    let row = floor(k / cols);
    return vec2<f32>(col * pitch + 1.0, 1.0 + row * pitch + 1.0);
}

// detailAcross is how many cells tile k spans (k from 1), from the header.
fn detailAcross(k: f32) -> f32 {
    let header = textureLoad(GroundDetail, vec2<i32>(i32(k - 1.0), 0), 0).r;
    return max(header * acrossScale, 1.0 / acrossScale);
}

// detailOf is tile k's tone at the world point p (k from 1), blended by the sampler, the border
// carrying it across the seam.
fn detailOf(k: f32, p: vec2<f32>) -> vec3<f32> {
    let size = vec2<f32>(textureDimensions(GroundDetail));
    let uv = fract(p / (U.Cell * detailAcross(k)));
    let at = detailTile(k - 1.0) + 0.5 + uv * U.DetailPx;
    return textureSampleLevel(GroundDetail, linear, at / size, 0.0).rgb;
}

// cellDetailAt is the detail of the kind of the cell whose flags are flags: 255 where its
// pixels hold more than one kind (painter.Dressed), to be read from the kinds.
fn cellDetailAt(flags: vec4<f32>) -> f32 {
    return floor(flags.a * 255.0 + 0.5);
}

// kindAt is the detail of the kind at the kinds' pixel t, 0 none.
fn kindAt(t: vec2<f32>) -> f32 {
    return floor(imageSrc3At(imageSrc3Origin() + t).r * 255.0 + 0.5);
}

// detailAt is the tone the details lay at p, round a half: the kinds of the four pixels round p
// blended between them as the albedo is, each its detail's tone, the kinds without one a half.
// A cell of one kind carries it in own (the flags' alpha, cellDetailAt); one kind round p, the
// common case among the rest, is told by one blended read: four kinds alike blend to their own
// number, two apart to one between.
fn detailAt(p: vec2<f32>, own: f32) -> vec3<f32> {
    if own < 254.5 {
        return detailToneOf(own, p);
    }
    let t = clamp(p / U.Cell * U.Px, vec2<f32>(0.5), (U.Corners - 1.0) * U.Px - 0.5);
    let r = imageSrc3Linear(imageSrc3Origin() + t).r * 255.0;
    let k = floor(r + 0.5);
    if abs(r - k) < 0.02 {
        return detailToneOf(k, p);
    }
    let i = floor(t - 0.5);
    let f = t - 0.5 - i;
    let k00 = kindAt(i + 0.5);
    let k10 = kindAt(i + vec2<f32>(1.5, 0.5));
    let k01 = kindAt(i + vec2<f32>(0.5, 1.5));
    let k11 = kindAt(i + vec2<f32>(1.5, 1.5));
    var tone = vec3<f32>(0.0);
    tone += detailToneOf(k00, p) * (1.0 - f.x) * (1.0 - f.y);
    tone += detailToneOf(k10, p) * f.x * (1.0 - f.y);
    tone += detailToneOf(k01, p) * (1.0 - f.x) * f.y;
    tone += detailToneOf(k11, p) * f.x * f.y;
    return tone;
}

fn detailToneOf(k: f32, p: vec2<f32>) -> vec3<f32> {
    if k < 0.5 {
        return vec3<f32>(0.5);
    }
    return detailOf(k, p);
}

// detailShow is how much of the detail shows where a cell spans span screen pixels.
fn detailShow(span: f32) -> f32 {
    return smoothstep(detailFar, detailNear, span);
}
