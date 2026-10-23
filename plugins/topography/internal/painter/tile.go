package painter

func (t *tile) baseTop() *cellTop {
	if t.base == nil {
		t.base = t.r.base(t.ID)
	}
	return t.base
}
