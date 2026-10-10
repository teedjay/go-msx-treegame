// Command rledecode decodes RLE streams from the cartridge and reports their
// extent and VRAM segments, to check tight packing between streams.
//
//	go run ./cmd/rledecode 47EF 4800 48C2:0600
//
// An argument ADDR decodes a stream that starts with its VRAM address; an
// argument ADDR:DEST decodes one that starts writing at DEST.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"go-msx-treegame/internal/rle"
	"go-msx-treegame/internal/rom"
)

func main() {
	romPath := flag.String("rom", rom.DefaultPath, "cartridge image")
	flag.Parse()
	r, err := rom.Load(*romPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	r.WarnIfUnverified()

	for _, arg := range flag.Args() {
		var addr, dest uint16
		withAddr := true
		if a, d, ok := strings.Cut(arg, ":"); ok {
			fmt.Sscanf(a, "%x", &addr)
			fmt.Sscanf(d, "%x", &dest)
			withAddr = false
		} else {
			fmt.Sscanf(arg, "%x", &addr)
		}
		s, err := rle.Decode(r.At, addr, withAddr, dest)
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Printf("$%04X-$%04X (%d bytes in ROM) -> %d bytes in %d segment(s)\n",
			s.Start, s.End-1, int(s.End-s.Start), s.Size(), len(s.Segments))
		for _, g := range s.Segments {
			fmt.Printf("    VRAM $%04X-$%04X %5d bytes  %s\n", g.VRAM, int(g.VRAM)+len(g.Data)-1, len(g.Data), vramTable(g.VRAM))
		}
	}
}

// vramTable names the table a VRAM address falls in, using the layout set
// by the register table at $452C.
func vramTable(a uint16) string {
	switch {
	case a < 0x1800:
		return fmt.Sprintf("colour table (third %d)", a/0x800)
	case a < 0x2000:
		return "sprite patterns"
	case a < 0x3800:
		return fmt.Sprintf("pattern generator (third %d)", (a-0x2000)/0x800)
	case a < 0x3B00:
		return "name table"
	case a < 0x3B80:
		return "sprite attributes"
	}
	return "unused VRAM"
}
