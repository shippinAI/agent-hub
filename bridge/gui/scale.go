package gui

import "image"

// scaleRGBA downsamples img so its longest edge is <= maxEdge (0 = no scaling), nearest-neighbour.
func scaleRGBA(img *image.RGBA, maxEdge int) *image.RGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if maxEdge <= 0 || (w <= maxEdge && h <= maxEdge) {
		return img
	}
	nw, nh := w, h
	if w >= h {
		nw = maxEdge
		nh = h * maxEdge / w
	} else {
		nh = maxEdge
		nw = w * maxEdge / h
	}
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	out := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		sy := y * h / nh
		for xx := 0; xx < nw; xx++ {
			sx := xx * w / nw
			out.SetRGBA(xx, y, img.RGBAAt(b.Min.X+sx, b.Min.Y+sy))
		}
	}
	return out
}
