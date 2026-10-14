// Ember is a living flame worked out per pixel from the entity's own state, which the renderer
// feeds the material: its middle and half its side in custom.xy/.z, the way it heads in custom.w
// (radians, 0 east, against the clock with y down), and in red how fast it moves, 0 to 1. A
// standing ember breathes on the composer's Clock; a moving one leans into a tail streaming
// against its heading, longer and wilder the faster it runs — three animations, no frames.
fn Ember(p: vec2<f32>, red: f32, fraction: f32, custom: vec4<f32>) -> vec4<f32> {
    let q = (p - custom.xy) / custom.z; // the box as -1..1, the middle at 0
    let h = vec2<f32>(cos(custom.w), -sin(custom.w)); // the way it heads, on a screen with y down
    let along = dot(q, h);
    let aside = dot(q, vec2<f32>(-h.y, h.x));

    // the flame's frame: behind the middle the tail stretches with speed, ahead it is blunt
    var s = along;
    if s < 0.0 {
        s = s / (1.0 + 2.5 * red);
    } else {
        s = s * (1.4 + red);
    }
    let d = length(vec2<f32>(s, aside * (1.2 + 0.6 * red)));

    // breath while standing, flicker while running
    let breath = 1.0 + (0.12 - 0.10 * red) * sin(U.Clock * 2.2);
    let flicker = 1.0 + (0.15 + 0.55 * red) * (noise(p * 0.35 + vec2<f32>(U.Clock * (1.0 + 6.0 * red), -U.Clock * 2.0)) - 0.5);
    let reach = 0.55 * breath;
    let glow = clamp(1.0 - d / reach, 0.0, 1.0) * flicker;
    if glow <= 0.0 {
        return vec4<f32>(0.0);
    }
    let heat = glow * glow;
    let tint = mix(vec3<f32>(0.9, 0.25, 0.05), vec3<f32>(1.0, 0.9, 0.55), heat); // orange rim, white-hot core
    let a = min(heat * 1.6, 1.0);
    return vec4<f32>(tint * a, a); // premultiplied: the flame glazes the meadow
}
