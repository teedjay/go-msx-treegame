package msxrender

import "testing"

// The cartridge's layout, from the register table at $452C.
var regs = [8]byte{0x02, 0xE2, 0x0E, 0x7F, 0x07, 0x76, 0x03, 0xE4}

func at(t *testing.T, v *[VRAMSize]byte, x, y int) [3]byte {
	t.Helper()
	img := Render(v, &regs)
	o := img.PixOffset(x, y)
	return [3]byte{img.Pix[o], img.Pix[o+1], img.Pix[o+2]}
}

func rgb(i int) [3]byte { p := Palette[i]; return [3]byte{p.R, p.G, p.B} }

func TestGraphic2Layout(t *testing.T) {
	var v [VRAMSize]byte
	// Character $41 in the middle third: pattern at $2000+$800+$41*8,
	// colour at $0000+$800+$41*8. Row 0 = left half set, white on red.
	v[0x2000+0x800+0x41*8] = 0xF0
	v[0x0000+0x800+0x41*8] = 0xF8
	v[0x3800+8*32+3] = 0x41 // tile (3, 8): first row of the middle third
	if got := at(t, &v, 24, 64); got != rgb(15) {
		t.Errorf("foreground pixel = %v, want white", got)
	}
	if got := at(t, &v, 28, 64); got != rgb(8) {
		t.Errorf("background pixel = %v, want red", got)
	}
	// Colour 0 shows the backdrop (dark blue from R7), not black.
	if got := at(t, &v, 100, 100); got != rgb(4) {
		t.Errorf("empty pixel = %v, want backdrop", got)
	}
}

func TestSprites(t *testing.T) {
	var v [VRAMSize]byte
	// Sprite 0: Y=9 (top row 10), X=20, pattern 5 (-> 4 with 16x16), colour 15.
	sat := 0x3B00
	v[sat], v[sat+1], v[sat+2], v[sat+3] = 9, 20, 5, 15
	v[sat+4] = 0xD0 // ends the list
	// Pattern 4 at $1800+4*8: TR quadrant (offset 16) row 0, leftmost pixel.
	v[0x1800+4*8+16] = 0x80
	if got := at(t, &v, 28, 10); got != rgb(15) {
		t.Errorf("TR quadrant pixel = %v, want white", got)
	}
	if got := at(t, &v, 20, 10); got != rgb(4) {
		t.Errorf("TL quadrant pixel = %v, want backdrop", got)
	}
	// Sprites after the $D0 terminator are not drawn.
	v[sat+8], v[sat+9], v[sat+10], v[sat+11] = 50, 50, 4, 15
	if got := at(t, &v, 58, 51); got != rgb(4) {
		t.Errorf("sprite after terminator drawn")
	}
}

func TestFifthSpriteDropped(t *testing.T) {
	var v [VRAMSize]byte
	for i := 0; i < 32; i++ {
		v[0x1800+i] = 0xFF
	}
	for s := 0; s < 5; s++ {
		a := 0x3B00 + s*4
		v[a], v[a+1], v[a+2], v[a+3] = 99, byte(s*20), 0, 15
	}
	v[0x3B00+20] = 0xD0
	if got := at(t, &v, 60, 100); got != rgb(15) {
		t.Errorf("fourth sprite missing")
	}
	if got := at(t, &v, 80, 100); got != rgb(4) {
		t.Errorf("fifth sprite drawn, want dropped")
	}
}
