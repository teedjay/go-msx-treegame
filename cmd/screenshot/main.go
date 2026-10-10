// Command screenshot drives a real console instance headlessly and writes
// stills through internal/msxrender, the renderer the game itself uses.
// The committed stills in screenshots/ are the regression baseline:
//
//	go run ./cmd/screenshot -out /tmp/shots && \
//	  for f in screenshots/*.png; do cmp "$f" "/tmp/shots/$(basename "$f")"; done
package main

import (
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"go-msx-treegame/internal/game"
	"go-msx-treegame/internal/msxrender"
)

// still is a frame to capture, counted in Frame calls after Boot.
type still struct {
	name  string
	frame int
}

var stills = []still{
	{"01-logo-scroll", 15},   // state 1: publisher logo rising
	{"02-logo", 200},         // state 2: logo and text held
	{"03-title-reveal", 290}, // state 4: title logo half revealed
	{"04-menu", 350},         // state 5: menu, pointer shown
	{"05-wipe", 610},         // state 6: column wipe
	{"06-stage1", 1100},      // state 7: the demo playing stage 1
}

func main() {
	out := flag.String("out", "screenshots", "output directory")
	flag.Parse()
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fail(err)
	}
	c := game.New()
	c.Boot()
	frame := 0
	for _, s := range stills {
		for ; frame < s.frame; frame++ {
			c.Frame()
		}
		path := filepath.Join(*out, s.name+".png")
		f, err := os.Create(path)
		if err != nil {
			fail(err)
		}
		if err := png.Encode(f, msxrender.Render(&c.VRAM, &c.Reg)); err != nil {
			fail(err)
		}
		f.Close()
		fmt.Printf("%s (frame %d, state %02X)\n", path, s.frame, c.RAM[0])
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
