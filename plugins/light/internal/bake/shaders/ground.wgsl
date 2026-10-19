// The ground the shade is baked over: the frame's heights (render.GroundHeights) as the ground
// is drawn, level at 0 in a frame without them.

fn bakedHeight(p: vec2<f32>) -> f32 {
    if U.GroundLattice.w < 0.5 {
        return 0.0;
    }
    return groundDrawnAt(p);
}

// bakedLow is the ground's lowest height, bakedTop its highest.
fn bakedLow() -> f32 {
    if U.GroundLattice.w < 0.5 {
        return 0.0;
    }
    return U.GroundSpan.z;
}

fn bakedTop() -> f32 {
    if U.GroundLattice.w < 0.5 {
        return 0.0;
    }
    return U.GroundSpan.z + U.GroundSpan.w;
}

// How many steps of three quarters of a cell a line towards the light takes, and how wide the
// shadows' penumbra is per unit towards the light: a cone of about 4.6°.
const shadowSteps: i32 = 16;
const shadowSoft: f32 = 0.08;
