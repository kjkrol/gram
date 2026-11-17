// The wind over the ground: what stands on a cell leans with it and lightens as the air's gusts
// go over it (the air's gust.wgsl: the same gusts lean the trees). How much stands and how hard
// the gust blows are worked out at the lattice's corners (mesh.wgsl's vs_main) and blended
// between them.

// swayAmountAt is how much what stands round the lattice corner i sways: the mean of its four
// cells' (the flags' blue).
fn swayAmountAt(i: vec2<f32>) -> f32 {
    let o = imageSrc0Origin() + vec2<f32>(0.0, U.Corners.y) + 0.5;
    let last = U.Corners - 2.0;
    var sum = 0.0;
    for (var dy = -1.0; dy <= 0.0; dy += 1.0) {
        for (var dx = -1.0; dx <= 0.0; dx += 1.0) {
            let c = clamp(i + vec2<f32>(dx, dy), vec2<f32>(0.0), last);
            sum += imageSrc0At(o + c).b;
        }
    }
    return sum / 4.0;
}
