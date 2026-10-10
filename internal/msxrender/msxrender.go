// Package msxrender composites TMS9918A VRAM into an image. It is the only
// renderer in the project: the game, cmd/screenshot and every preview tool
// draw through it, so a still cannot show something the game would not.
//
// Only Graphic 2 (Screen 2), the mode this cartridge uses, is implemented.
package msxrender

import (
	"image"
	"image/color"
)

const (
	Width  = 256
	Height = 192
	// VRAMSize is the TMS9918A's 16KB of video memory.
	VRAMSize = 0x4000
)

// Palette is the TMS9918A palette. Index 0 is transparent and is never
// drawn directly; it shows the backdrop colour (R7 low nibble).
var Palette = [16]color.RGBA{
	{0, 0, 0, 255},       // 0 transparent
	{0, 0, 0, 255},       // 1 black
	{33, 200, 66, 255},   // 2 medium green
	{94, 220, 120, 255},  // 3 light green
	{84, 85, 237, 255},   // 4 dark blue
	{125, 118, 252, 255}, // 5 light blue
	{212, 82, 77, 255},   // 6 dark red
	{66, 235, 245, 255},  // 7 cyan
	{252, 85, 84, 255},   // 8 medium red
	{255, 121, 120, 255}, // 9 light red
	{212, 193, 84, 255},  // 10 dark yellow
	{230, 206, 128, 255}, // 11 light yellow
	{33, 176, 59, 255},   // 12 dark green
	{201, 91, 186, 255},  // 13 magenta
	{204, 204, 204, 255}, // 14 grey
	{255, 255, 255, 255}, // 15 white
}

// Render composites a frame from VRAM and VDP registers R0-R7.
func Render(vram *[VRAMSize]byte, reg *[8]byte) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, Width, Height))
	RenderInto(img, vram, reg)
	return img
}

// RenderInto composites into an existing 256x192 image.
func RenderInto(img *image.RGBA, vram *[VRAMSize]byte, reg *[8]byte) {
	var idx [Width * Height]byte // palette index per pixel
	backdrop := reg[7] & 0x0F
	if reg[1]&0x40 != 0 { // display enabled
		renderGraphic2(&idx, vram, reg)
		renderSprites(&idx, vram, reg)
	}
	for i, c := range idx {
		if c == 0 {
			c = backdrop
		}
		p := Palette[c]
		o := i * 4
		img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = p.R, p.G, p.B, 255
	}
}

// renderGraphic2 draws the pattern layer. Table bases and masks follow the
// TMS9918A: in Graphic 2, R3 bit 7 selects the colour table half and R3
// bits 6-0 mask the colour address; R4 bit 2 selects the pattern table half
// and R4 bits 1-0 mask which screen thirds have their own patterns.
func renderGraphic2(idx *[Width * Height]byte, vram *[VRAMSize]byte, reg *[8]byte) {
	nameBase := int(reg[2]&0x0F) << 10
	colBase := int(reg[3]&0x80) << 6
	colMask := int(reg[3]&0x7F)<<6 | 0x3F
	patBase := int(reg[4]&0x04) << 11
	patMask := int(reg[4]&0x03)<<11 | 0x7FF
	for ty := 0; ty < 24; ty++ {
		third := ty / 8
		for tx := 0; tx < 32; tx++ {
			name := int(vram[nameBase+ty*32+tx])
			char := third<<8 | name
			for row := 0; row < 8; row++ {
				off := char<<3 | row
				pat := vram[patBase|off&patMask]
				col := vram[colBase|off&colMask]
				fg, bg := col>>4, col&0x0F
				o := (ty*8+row)*Width + tx*8
				for b := 0; b < 8; b++ {
					if pat&(0x80>>b) != 0 {
						idx[o+b] = fg
					} else {
						idx[o+b] = bg
					}
				}
			}
		}
	}
}

// renderSprites draws up to 32 hardware sprites. Y=$D0 ends the list; the
// displayed top row is Y+1; at most four sprites show on one line (the
// fifth and later are dropped, lower numbers having priority); EC (colour
// bit 7) shifts the sprite 32 pixels left. With 16x16 sprites the pattern
// number's low two bits are ignored and the four 8x8 quadrants are stored
// TL, BL, TR, BR.
func renderSprites(idx *[Width * Height]byte, vram *[VRAMSize]byte, reg *[8]byte) {
	satBase := int(reg[5]&0x7F) << 7
	sptBase := int(reg[6]&0x07) << 11
	size := 8
	if reg[1]&0x02 != 0 {
		size = 16
	}
	mag := 1
	if reg[1]&0x01 != 0 {
		mag = 2
	}
	span := size * mag

	var lineCount [Height]int
	var drawn [Width * Height]bool // a lower-numbered sprite already owns the pixel
	for s := 0; s < 32; s++ {
		a := satBase + s*4
		y := int(vram[a])
		if y == 0xD0 {
			break
		}
		top := (y + 1) & 0xFF
		if top > 0xE0 { // wrapped: partially above the screen
			top -= 256
		}
		x := int(vram[a+1])
		pat := int(vram[a+2])
		ec := vram[a+3]
		c := ec & 0x0F
		if ec&0x80 != 0 {
			x -= 32
		}
		if size == 16 {
			pat &^= 3
		}
		for dy := 0; dy < span; dy++ {
			sy := top + dy
			if sy < 0 || sy >= Height {
				continue
			}
			lineCount[sy]++
			if lineCount[sy] > 4 || c == 0 {
				continue
			}
			row := dy / mag
			for dx := 0; dx < span; dx++ {
				sx := x + dx
				if sx < 0 || sx >= Width {
					continue
				}
				col := dx / mag
				// Quadrant offset: TL 0, BL 8, TR 16, BR 24.
				off := pat*8 + (col/8)*16 + row
				if vram[sptBase+off&0x7FF]&(0x80>>(col%8)) == 0 {
					continue
				}
				p := sy*Width + sx
				if !drawn[p] {
					drawn[p] = true
					idx[p] = c
				}
			}
		}
	}
}
