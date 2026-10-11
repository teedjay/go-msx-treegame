package main

import (
	"image"
	"image/color"

	"go-msx-treegame/assets"
	"go-msx-treegame/internal/msxrender"
)

// The overlay is drawn by the frontend over the rendered frame. It never
// touches VRAM, so it cannot affect the game or the lockstep checks.

const (
	overlayFrames = 120 // about two seconds at 60 Hz
	overlayFade   = 30  // fade out over the last half second
	overlayScale  = 2   // glyphs drawn at double size
)

// overlay is a short-lived message such as "STAGE 5".
type overlay struct {
	text string
	left int // frames remaining
}

func (o *overlay) show(text string) { o.text, o.left = text, overlayFrames }

func (o *overlay) tick() {
	if o.left > 0 {
		o.left--
	}
}

// alpha is the overlay's opacity for the current frame, 0..1.
func (o *overlay) alpha() float64 {
	if o.left <= 0 {
		return 0
	}
	if o.left < overlayFade {
		return float64(o.left) / overlayFade
	}
	return 1
}

var (
	overlayBox    = color.RGBA{0, 0, 0, 200} // translucent black
	overlayBorder = msxrender.Palette[15]    // white
	overlayInk    = msxrender.Palette[7]     // cyan, as the menu text
)

// draw composites the overlay centred on dst.
func (o *overlay) draw(dst *image.RGBA) {
	a := o.alpha()
	if a == 0 {
		return
	}
	font := assets.Get("font_45eb")
	gw := 8 * overlayScale
	w := len(o.text)*gw + 2*gw
	h := 8*overlayScale + gw
	x0 := (msxrender.Width - w) / 2
	y0 := (msxrender.Height - h) / 2

	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			c := overlayBox
			if x == x0 || x == x0+w-1 || y == y0 || y == y0+h-1 {
				c = overlayBorder
			}
			blend(dst, x, y, c, a)
		}
	}
	tx, ty := x0+gw, y0+gw/2
	for i := 0; i < len(o.text); i++ {
		g := glyph(font, o.text[i])
		if g == nil {
			continue
		}
		for row := 0; row < 8; row++ {
			for col := 0; col < 8; col++ {
				if g[row]&(0x80>>col) == 0 {
					continue
				}
				for dy := 0; dy < overlayScale; dy++ {
					for dx := 0; dx < overlayScale; dx++ {
						blend(dst, tx+i*gw+col*overlayScale+dx, ty+row*overlayScale+dy, overlayInk, a)
					}
				}
			}
		}
	}
}

// glyph returns the 8 rows of a character from the game's font, which
// holds the codes '0'-'Z' ($30-$5A); space and anything else draw nothing.
func glyph(font []byte, ch byte) []byte {
	if ch < 0x30 || ch > 0x5A {
		return nil
	}
	o := int(ch-0x30) * 8
	return font[o : o+8]
}

// blend mixes c into dst at (x, y) with c's own alpha scaled by a.
func blend(dst *image.RGBA, x, y int, c color.RGBA, a float64) {
	if !(image.Point{x, y}.In(dst.Rect)) {
		return
	}
	k := a * float64(c.A) / 255
	i := dst.PixOffset(x, y)
	for j, v := range [3]uint8{c.R, c.G, c.B} {
		dst.Pix[i+j] = uint8(float64(dst.Pix[i+j])*(1-k) + float64(v)*k)
	}
}
