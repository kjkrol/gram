// The clouds as heaps seen from under them (backdrop, with bodies on), the layers U.HeapsLayers
// says, the low before the high: every pixel of a view U.HeapsView pixels wide, worked out
// U.HeapsPixel screen pixels at a time, looks along its line of sight (U.LookDir + U.LookDX·x +
// U.LookDY·y from U.EyeAt) through the heaps (cloudHeap), lit from above and darker underneath,
// the thin edges shining towards the sun, and to the sheets of shreds over them; hazed with the
// distance (U.Visibility) and faded along the horizon; with U.HeapsBefore the low heaps only past
// the highest ground (groundHighest), those before it drawn before the ground. Premultiplied, for
// the sky to lay over.

struct Out {
    @builtin(position) clip: vec4<f32>,
    @location(0) px: vec2<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> Out {
    var corners = array<vec2<f32>, 3>(
        vec2<f32>(-1.0, -1.0),
        vec2<f32>(3.0, -1.0),
        vec2<f32>(-1.0, 3.0),
    );
    var o: Out;
    o.clip = vec4<f32>(corners[vid], 0.0, 1.0);
    o.px = vec2<f32>(corners[vid].x + 1.0, 1.0 - corners[vid].y) * 0.5 * U.HeapsView;
    return o;
}

@fragment
fn fs_main(o: Out) -> @location(0) vec4<f32> {
    let d = U.LookDir + U.LookDX * o.px.x + U.LookDY * o.px.y;
    let view = d / max(length(d), 1e-6);
    let fade = smoothstep(skyLift, 3.0 * skyLift, view.z);
    if fade <= 0.0 {
        return vec4<f32>(0.0);
    }
    // the steps put off at random a pixel: an even pattern showed as stripes
    let dither = fract(sin(dot(o.px, vec2<f32>(12.9898, 78.233))) * 43758.5453);
    var seen = vec4<f32>(0.0);
    if U.HeapsLayers.x > 0.5 {
        seen = heapsOf(o.px, d, dither, true);
    }
    if U.HeapsLayers.y > 0.5 && seen.a < 0.97 {
        seen += (1.0 - seen.a) * heapsOf(o.px, d, dither, false);
    }
    return seen * fade;
}

// heapsOf is the low layer's heaps if low, else the high one's sheets, along the line of sight d
// of the view's pixel px, the heaps' steps put off by dither: premultiplied, hazed with the
// distance.
fn heapsOf(px: vec2<f32>, d: vec3<f32>, dither: f32, low: bool) -> vec4<f32> {
    let span = cloudBodySpan(low);
    let enter = max((span.x - U.EyeAt.z) / d.z, 0.0);
    let leave = (span.y - U.EyeAt.z) / d.z;
    if leave <= enter {
        return vec4<f32>(0.0);
    }
    let len = length(d);
    let sky = U.Ambience * cloudsSkyLit;
    let sun = U.LightColor * U.LightStrength * cloudsSunLit;
    if !low {
        // a thin sheet where the line meets it, at its own height
        let t = cloudHighCross(U.EyeAt, d, enter, leave);
        if t < 0.0 {
            return vec4<f32>(0.0);
        }
        let p = U.EyeAt.xy + d.xy * t;
        let m = cloudHighHere(cloudHighMass(airAt(p)), p);
        if m.x <= 0.0 {
            return vec4<f32>(0.0);
        }
        let c = cloudHighAt(p, t * length(U.LookDX) * U.HeapsPixel);
        // thin wisps, thicker only in the middle
        let a = cloudCoverSoft(c.x, c.y, m) * heapsSheet * (0.5 + 0.5 * cloudThick(c.x, m));
        return vec4<f32>((sky + sun) * a, a) * (1.0 - heapsHaze(t * len));
    }
    var from = enter;
    if U.HeapsBefore > 0.5 {
        from = max(from, (groundHighest() - U.EyeAt.z) / d.z);
    }
    if leave <= from {
        return vec4<f32>(0.0);
    }
    let ray = cloudLowRay(U.EyeAt, d, from, leave);
    if ray.leave <= ray.enter {
        return vec4<f32>(0.0);
    }
    let m = cloudLowMass(airAt(U.EyeAt.xy + d.xy * 0.5 * (ray.enter + ray.leave)));
    if m.x <= 0.0 {
        return vec4<f32>(0.0);
    }
    let toward = max(dot(d / max(len, 1e-6), lightWay()), 0.0);
    let glow = toward * toward * toward * toward * toward * toward;
    let step = (ray.leave - ray.enter) / f32(heapsSteps);
    var seen = vec4<f32>(0.0);
    for (var i = 0; i < heapsSteps; i++) {
        let t = ray.enter + (f32(i) + dither) * step;
        let z = U.EyeAt.z + d.z * t;
        let p = U.EyeAt.xy + d.xy * t;
        let band = cloudRayBand(ray, t);
        let heap = cloudHeap(p, z, band, t * length(U.LookDX) * U.HeapsPixel, m);
        let a = heap.x * heapsDense;
        if a <= 0.0 {
            continue;
        }
        // as much of the sun as the heap over the point lets through: white at the top, grey at
        // the flat base
        let rgb = sky * mix(heapsUnder, 1.0, heap.y) + sun * mix(heapsBase, 1.0, heap.y) +
            U.LightColor * U.LightStrength * glow * (1.0 - heap.x) * heapsRim;
        seen += (1.0 - seen.a) * vec4<f32>(rgb * a, a);
        if seen.a > 0.97 {
            break;
        }
    }
    return seen * (1.0 - heapsHaze(ray.enter * len));
}

// heapsHaze is how much of what lies d world units off the air hides.
fn heapsHaze(d: f32) -> f32 {
    if U.Visibility <= 0.0 {
        return 0.0;
    }
    return 1.0 - exp(-d / U.Visibility);
}

// How many steps a line of sight takes through the heaps; how much of what is behind one step
// hides, and behind a sheet of shreds; how much darker a heap's base is than its top in the sky's
// light, and in the sun's; how brightly its thin edges shine to the sun.
const heapsSteps: i32 = 20;
const heapsSheet: f32 = 0.55;
const heapsDense: f32 = 0.28;
const heapsUnder: f32 = 0.7;
const heapsBase: f32 = 0.55;
const heapsRim: f32 = 0.8;
