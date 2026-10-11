package main

import (
	"image"
	"image/png"
	"os"
	"testing"

	"go-msx-treegame/internal/game"
	"go-msx-treegame/internal/msxrender"
)

func TestOverlayLifetime(t *testing.T) {
	var o overlay
	if o.alpha() != 0 {
		t.Fatal("visible before show")
	}
	o.show("STAGE 5")
	if o.alpha() != 1 {
		t.Fatalf("alpha %v at start, want 1", o.alpha())
	}
	for i := 0; i < overlayFrames-overlayFade/2; i++ {
		o.tick()
	}
	if a := o.alpha(); a <= 0 || a >= 1 {
		t.Errorf("alpha %v mid-fade, want between 0 and 1", a)
	}
	for i := 0; i < overlayFrames; i++ {
		o.tick()
	}
	if o.alpha() != 0 {
		t.Error("still visible after its lifetime")
	}
}

// The overlay changes only the pixels inside its box, and the game's VRAM
// is untouched. Set OVERLAY_PNG to write a preview.
func TestOverlayDraw(t *testing.T) {
	c := game.New()
	c.Boot()
	for c.RAM[0] != 5 {
		c.Frame()
	}
	if !c.CheatStart(5) {
		t.Fatal("cheat refused on the menu")
	}
	for i := 0; i < 90; i++ {
		c.Frame()
	}
	vram := c.VRAM
	base := msxrender.Render(&c.VRAM, &c.Reg)
	img := image.NewRGBA(base.Rect)
	copy(img.Pix, base.Pix)
	var o overlay
	o.show("STAGE 5")
	o.draw(img)
	if vram != c.VRAM {
		t.Error("overlay changed VRAM")
	}
	changed := 0
	for i := range img.Pix {
		if img.Pix[i] != base.Pix[i] {
			changed++
		}
	}
	if changed == 0 {
		t.Error("overlay drew nothing")
	}
	// Corners stay untouched.
	for _, p := range []image.Point{{0, 0}, {255, 0}, {0, 191}, {255, 191}} {
		if img.RGBAAt(p.X, p.Y) != base.RGBAAt(p.X, p.Y) {
			t.Errorf("pixel %v changed", p)
		}
	}
	if path := os.Getenv("OVERLAY_PNG"); path != "" {
		f, _ := os.Create(path)
		png.Encode(f, img)
		f.Close()
	}
}
