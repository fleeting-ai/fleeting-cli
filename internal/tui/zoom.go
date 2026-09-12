package tui

func zoomOrigin(fc, fr, n, span int) (c0, r0 int) {
	if span < 1 {
		span = 1
	}
	if span > n {
		span = n
	}
	c0 = fc - (span - 1)
	r0 = fr - (span - 1)
	if c0 < 0 {
		c0 = 0
	}
	if r0 < 0 {
		r0 = 0
	}
	if c0 > n-span {
		c0 = n - span
	}
	if r0 > n-span {
		r0 = n - span
	}
	return c0, r0
}

func inZoom(c, r, c0, r0, span int) bool {
	return c >= c0 && c < c0+span && r >= r0 && r < r0+span
}
