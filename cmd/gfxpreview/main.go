// Command gfxpreview loads the graphics through the ported engine routines
// and renders them through internal/msxrender as PNG sheets:
//
//	tiles.png      name table filled with 0-255 in each screen third
//	sprites_N.png  16x16 sprite patterns, 32 per sheet, 4 per row
//
// The output is ROM-derived: write it outside the repository.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"go-msx-treegame/internal/game"
	"go-msx-treegame/internal/msxrender"
)

func main() {
	out := flag.String("out", "/tmp/gfx", "output directory")
	stage := flag.Uint("stagecol", 0x00, "value for RAM[$05D] (stage colour substitution)")
	scale := flag.Int("scale", 3, "integer upscale factor")
	flag.Parse()
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fail(err)
	}

	c := game.New()
	c.DebugLoadGraphics(byte(*stage))

	// Tile sheet: every character of every third, no sprites.
	v := c.VRAM
	for i := 0; i < 768; i++ {
		v[0x3800+i] = byte(i)
	}
	v[0x3B00] = 0xD0
	save(filepath.Join(*out, "tiles.png"), msxrender.Render(&v, &c.Reg), *scale)

	// Sprite sheets: patterns blanked so only sprites show.
	for sheet := 0; sheet < 2; sheet++ {
		v := c.VRAM
		for i := 0x2000; i < 0x3800; i++ {
			v[i] = 0
		}
		for s := 0; s < 32; s++ {
			a := 0x3B00 + s*4
			v[a] = byte((s/4)*24 + 3) // displayed row is Y+1
			v[a+1] = byte((s%4)*24 + 8)
			v[a+2] = byte((sheet*32 + s) * 4)
			v[a+3] = 15
		}
		save(filepath.Join(*out, fmt.Sprintf("sprites_%d.png", sheet)), msxrender.Render(&v, &c.Reg), *scale)
	}
	fmt.Println("wrote", *out)
}

func save(path string, img *image.RGBA, k int) {
	b := img.Bounds()
	big := image.NewRGBA(image.Rect(0, 0, b.Dx()*k, b.Dy()*k))
	for y := 0; y < b.Dy()*k; y++ {
		for x := 0; x < b.Dx()*k; x++ {
			big.Set(x, y, img.At(x/k, y/k))
		}
	}
	f, err := os.Create(path)
	if err != nil {
		fail(err)
	}
	defer f.Close()
	if err := png.Encode(f, big); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
