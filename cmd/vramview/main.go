// Command vramview renders raw VRAM dumps through internal/msxrender, for
// comparing the port against reference state captured from an emulator.
//
//	go run ./cmd/vramview -scale 2 dump.vram ...
//
// Each NAME.vram (16384 bytes) needs a NAME.reg (R0-R7, 8 bytes) next to
// it; NAME.png is written alongside. If NAME.ram (game RAM $E000-$E3FF)
// exists, its first bytes are printed. Dumps are ROM-derived: keep them
// outside the repository.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"

	"go-msx-treegame/internal/msxrender"
)

func main() {
	scale := flag.Int("scale", 2, "integer upscale factor")
	flag.Parse()
	for _, path := range flag.Args() {
		base := strings.TrimSuffix(path, ".vram")
		vb, err := os.ReadFile(base + ".vram")
		check(err)
		rb, err := os.ReadFile(base + ".reg")
		check(err)
		var vram [msxrender.VRAMSize]byte
		var reg [8]byte
		copy(vram[:], vb)
		copy(reg[:], rb)
		img := msxrender.Render(&vram, &reg)
		save(base+".png", img, *scale)
		info := ""
		if ram, err := os.ReadFile(base + ".ram"); err == nil && len(ram) >= 16 {
			info = fmt.Sprintf("  RAM[$000-$00F]: % X", ram[:16])
		}
		fmt.Printf("%s.png  regs % X%s\n", base, reg, info)
	}
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
	check(err)
	defer f.Close()
	check(png.Encode(f, big))
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
