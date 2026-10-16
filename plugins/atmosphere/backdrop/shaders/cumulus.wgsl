// The clouds as heaps seen from under them (backdrop, with bodies on): every pixel of a view
// U.HeapsView pixels wide, worked out U.HeapsPixel screen pixels at a time, looks along its line of
// sight (U.LookDir + U.LookDX·x + U.LookDY·y from U.EyeAt) through the layer U.CloudHeight up and
// heapsDeep clouds' widths deep: a cloud's base flat on it, its top domed as high as it is thick,
// lit from above and darker underneath, its thin edges shining towards the sun; hazed with the
// distance (U.Visibility) and faded along the horizon. Premultiplied, for the sky to lay over.

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
    let len = length(d);
    let view = d / max(len, 1e-6);
    let fade = smoothstep(skyLift, 3.0 * skyLift, view.z);
    if fade <= 0.0 {
        return vec4<f32>(0.0);
    }
    let deep = heapsDeep * cloudSize;
    let enter = max((U.CloudHeight - U.EyeAt.z) / d.z, 0.0);
    let leave = (U.CloudHeight + deep - U.EyeAt.z) / d.z;
    if leave <= enter {
        return vec4<f32>(0.0);
    }
    let m = airAt(U.EyeAt.xy + d.xy * 0.5 * (enter + leave));
    if m.x <= 0.0 {
        return vec4<f32>(0.0);
    }
    var haze = 0.0;
    if U.Visibility > 0.0 {
        haze = 1.0 - exp(-enter * len / U.Visibility);
    }
    let toward = max(dot(view, sunWay()), 0.0);
    let glow = toward * toward * toward * toward * toward * toward;
    let sky = U.Ambience * cloudsSkyLit;
    let sun = U.SunColor * U.SunStrength * cloudsSunLit;
    let dither = fract(52.9829189 * fract(dot(o.px, vec2<f32>(0.06711056, 0.00583715))));
    let step = (leave - enter) / f32(heapsSteps);
    var seen = vec4<f32>(0.0);
    for (var i = 0; i < heapsSteps; i++) {
        let t = enter + (f32(i) + dither) * step;
        let h = (U.EyeAt.z + d.z * t - U.CloudHeight) / deep;
        let p = U.EyeAt.xy + d.xy * t;
        let pixel = t * length(U.LookDX) * U.HeapsPixel;
        let lod = cloudTileLevel(pixel, cloudTile * cloudSize);
        let torn = cloudTileAt(p, lod, cloudTile * cloudSize).r;
        var core = 0.0;
        if m.y > 0.0 {
            let heapLod = cloudTileLevel(pixel, heapTile * heapSize);
            core = cloudTileAt(p, heapLod, heapTile * heapSize).g;
        }
        let field = cloudFromTile(torn, core, m.y);
        let cover = cloudCoverSoft(field, cloudTileSoft * lod, m);
        let thick = cloudThick(field, m);
        // the top domed over the thick middle, rounded off to the edge
        let top = sqrt(thick) + 0.15 * cover;
        let a = cover * (1.0 - smoothstep(top - heapsSoft, top, h)) * heapsDense;
        if a <= 0.0 {
            continue;
        }
        let rgb = sky * mix(heapsUnder, 1.0, h) + sun * (0.35 + 0.65 * h) +
            U.SunColor * U.SunStrength * glow * (1.0 - thick) * heapsRim;
        seen += (1.0 - seen.a) * vec4<f32>(rgb * a, a);
        if seen.a > 0.97 {
            break;
        }
    }
    return seen * (1.0 - haze) * fade;
}

// How many steps a line of sight takes through the layer, how deep the layer is in clouds'
// widths, over how much of it a top rounds off, how much of what is behind one step hides, how
// much darker a cloud's base is than its top, how brightly its thin edges shine towards the sun.
const heapsSteps: i32 = 24;
const heapsDeep: f32 = 0.5;
const heapsSoft: f32 = 0.2;
const heapsDense: f32 = 0.2;
const heapsUnder: f32 = 0.55;
const heapsRim: f32 = 0.8;
