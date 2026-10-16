// What falls before the eye (precipitation.Renderer), six vertices a drop: U.Drops streaks of rain,
// then U.Flakes flakes of snow, each where its number and the clock U.Clock put it on a screen
// U.FallView pixels wide, the wind slanting the rain U.FallDrift pixels a second, as thick as
// U.FallMask says rain or snow falls where it is; drawn in the premultiplied colours U.RainColor
// and U.SnowColor.

struct Drop {
    @builtin(position) clip: vec4<f32>,
    @location(0) color: vec4<f32>,
    @location(1) edges: vec4<f32>, // how far in from each side, in pixels; 9 a side left hard
}

// spot is where drop i starts, 0 to 1 across and down the screen, fixed for it.
fn spot(i: u32) -> vec2<f32> {
    var x = i * 2654435761u + 0x9e3779b9u;
    x ^= x >> 16u;
    x *= 0x85ebca6bu;
    x ^= x >> 13u;
    var y = x * 0xc2b2ae35u + 0x27d4eb2fu;
    y ^= y >> 15u;
    return vec2<f32>(f32(x & 0xffffu), f32(y & 0xffffu)) / 65536.0;
}

// wrapAround is v within 0 up to n.
fn wrapAround(v: f32, n: f32) -> f32 {
    return v - n * floor(v / n);
}

@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> Drop {
    var across = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var down = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    let i = vid / 6u;
    let a = across[vid % 6u];
    let b = down[vid % 6u];
    let t = U.Clock;
    var d: Drop;
    var p: vec2<f32>;
    if i < u32(U.Drops) {
        // a streak a pixel wide and half a pixel of fade either side, from its top down along the fall
        let uv = spot(i);
        let y = wrapAround(uv.y * U.FallView.y + t * rainSpeed, U.FallView.y + rainDrop) - rainDrop;
        let x = wrapAround(uv.x * U.FallView.x + t * U.FallDrift, U.FallView.x);
        let top = vec2<f32>(x, y);
        let way = vec2<f32>(U.FallDrift * rainDrop / rainSpeed, rainDrop);
        let side = normalize(vec2<f32>(-way.y, way.x));
        p = top + way * b + side * (a - 0.5) * 2.0;
        if keep(i) >= fallMask(top / U.FallView, 0u) {
            p = vec2<f32>(-1e4); // where none falls, drawn nowhere
        }
        d.color = U.RainColor;
        d.edges = vec4<f32>(2.0 * a, 2.0 - 2.0 * a, 9.0, 9.0);
    } else {
        let uv = spot(i - u32(U.Drops) + (1u << 20u));
        let y = wrapAround(uv.y * U.FallView.y + t * snowSpeed * (0.7 + 0.6 * uv.x), U.FallView.y + snowFlake) - snowFlake;
        let x = wrapAround(uv.x * U.FallView.x + t * U.FallDrift * 0.5 + 6.0 * sin(t * 1.3 + uv.x * 40.0), U.FallView.x);
        p = vec2<f32>(x, y) + vec2<f32>(a, b) * snowFlake;
        if keep(i) >= fallMask(vec2<f32>(x, y) / U.FallView, 1u) {
            p = vec2<f32>(-1e4);
        }
        d.color = U.SnowColor;
        d.edges = vec4<f32>(a, 1.0 - a, b, 1.0 - b) * snowFlake;
    }
    d.clip = vec4<f32>(p.x / U.FallView.x * 2.0 - 1.0, 1.0 - p.y / U.FallView.y * 2.0, 0.0, 1.0);
    return d;
}

// keep is a share fixed for drop i, 0 to 1: it falls where what falls there is more than its share
// of the most (fallMask), so the drops are as thick as the rain.
fn keep(i: u32) -> f32 {
    return spot(i + (1u << 21u)).x;
}

// fallMask is how much of the most rain (k 0) or snow (k 1) falls at uv, 0 to 1 across and down
// the screen: U.FallMask's grid of fallCols by fallRows points in the middle of its cells blended.
fn fallMask(uv: vec2<f32>, k: u32) -> f32 {
    let size = vec2<f32>(fallCols, fallRows);
    let g = clamp(uv * size - 0.5, vec2<f32>(0.0), size - 1.0);
    let lo = vec2<u32>(floor(g));
    let hi = min(lo + 1u, vec2<u32>(size) - 1u);
    let f = g - floor(g);
    let top = mix(maskAt(lo.x, lo.y, k), maskAt(hi.x, lo.y, k), f.x);
    let bottom = mix(maskAt(lo.x, hi.y, k), maskAt(hi.x, hi.y, k), f.x);
    return mix(top, bottom, f.y);
}

fn maskAt(x: u32, y: u32, k: u32) -> f32 {
    let n = k * u32(fallCols * fallRows) + y * u32(fallCols) + x;
    return U.FallMask[n / 4u][n % 4u];
}

const fallCols: f32 = 8.0;
const fallRows: f32 = 6.0;

// fs_main is the drop's colour, faded over a pixel in from its soft sides.
@fragment
fn fs_main(d: Drop) -> @location(0) vec4<f32> {
    let f = smoothstep(vec4<f32>(0.0), vec4<f32>(1.0), clamp(d.edges, vec4<f32>(0.0), vec4<f32>(1.0)));
    return d.color * (f.x * f.y * f.z * f.w);
}

// How fast rain and snow fall, pixels a second; how long a streak of rain is and how wide a flake
// (precipitation's, the same).
const rainSpeed: f32 = 700.0;
const snowSpeed: f32 = 55.0;
const rainDrop: f32 = 18.0;
const snowFlake: f32 = 3.0;
