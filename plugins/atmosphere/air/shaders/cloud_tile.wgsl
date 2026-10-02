// A tile of the clouds' noise, baked once (air.BakeTile) for a shader to look up: every pixel holds
// the shreds (red) averaged over the world square round custom.xy, custom.z world units wide, and
// the heaps' cores (green, heapCoreLow up over heapCoreSpan) over the square four times as far
// out and as wide — their tile four times the shreds' — each from custom.w by custom.w points.

fn Fragment(dst: vec4<f32>, src: vec2<f32>, color: vec4<f32>, custom: vec4<f32>) -> vec4<f32> {
    let n = i32(custom.w);
    var torn = 0.0;
    var core = 0.0;
    for (var j = 0; j < n; j++) {
        for (var i = 0; i < n; i++) {
            let q = custom.xy + ((vec2<f32>(f32(i), f32(j)) + 0.5) / f32(n) - 0.5) * custom.z;
            torn += shreds(q / cloudSize);
            core += clamp((heapCores(4.0 * q / heapSize) - heapCoreLow) / heapCoreSpan, 0.0, 1.0);
        }
    }
    let k = 1.0 / f32(n * n);
    return vec4<f32>(clamp(torn * k, 0.0, 1.0), core * k, 0.0, 1.0);
}
