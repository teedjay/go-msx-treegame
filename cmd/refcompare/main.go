// Command refcompare runs the port in lockstep with a reference trace
// recorded from the original by scripts/refdump.tcl, and reports the first
// tick where RAM, VRAM or the VDP registers differ.
//
//	go run ./cmd/refcompare -trace ref.trace [-max 20] [-png dir]
//
// Between ticks it runs the sound sequencer as many times as the reference
// took interrupts, so tick pacing differences do not show up as diffs.
// RAM $E300-$E3FF (the Z80 stack) is not compared.
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"go-msx-treegame/internal/game"
	"go-msx-treegame/internal/msxrender"
)

const (
	ramSize  = 0x400
	recSize  = 4 + 8 + ramSize + msxrender.VRAMSize + 8
	stackLow = 0x300 // RAM from here up is the Z80 stack
)

type record struct {
	tick, irq int
	ram       []byte
	vram      []byte
	reg       []byte
}

func main() {
	tracePath := flag.String("trace", "ref.trace", "reference trace from scripts/refdump.tcl")
	maxDiffs := flag.Int("max", 20, "diffs to print at the first divergent tick")
	pngDir := flag.String("png", "", "if set, write ref.png and port.png of the divergent tick here")
	keysPath := flag.String("keys", "", "input schedule given to scripts/refdump.tcl (REFDUMP_KEYS format), replayed into the port")
	flag.Parse()

	data, err := os.ReadFile(*tracePath)
	if err != nil {
		fail(err)
	}
	if len(data)%recSize != 0 {
		fail(fmt.Errorf("%s: size %d is not a multiple of the record size %d", *tracePath, len(data), recSize))
	}
	n := len(data) / recSize

	rnd := loadRnd(*tracePath + ".rnd")
	curTick := 0
	rndMismatch := ""

	keys := loadKeys(*keysPath)

	c := game.New()
	c.Boot()
	c.RandR = func(pc uint16) byte {
		if len(rnd) == 0 {
			if rndMismatch == "" {
				rndMismatch = fmt.Sprintf("tick %d: port reads R at $%04X, reference log is exhausted", curTick, pc)
			}
			return 0
		}
		e := rnd[0]
		rnd = rnd[1:]
		if (e.tick != curTick || e.pc != pc) && rndMismatch == "" {
			rndMismatch = fmt.Sprintf("tick %d: port reads R at $%04X, reference read it at $%04X in tick %d", curTick, pc, e.pc, e.tick)
		}
		return e.val
	}
	prevIRQ := 0
	for i := 0; i < n; i++ {
		r := parse(data[i*recSize : (i+1)*recSize])
		curTick = r.tick
		for k := prevIRQ; k < r.irq; k++ {
			c.SoundTick()
		}
		prevIRQ = r.irq
		c.BeginTick()

		if d := diff(c, r); len(d) > 0 {
			fmt.Printf("tick %d (state RAM[$000]=%02X ref, %02X port): %d difference(s)\n",
				r.tick, r.ram[0], c.RAM[0], len(d))
			for j, s := range d {
				if j == *maxDiffs {
					fmt.Printf("  ... %d more\n", len(d)-j)
					break
				}
				fmt.Println(" ", s)
			}
			if *pngDir != "" {
				writePNGs(*pngDir, c, r)
			}
			os.Exit(1)
		}
		applyKeys(c, keys[r.tick])
		c.RunTick()
		if rndMismatch != "" {
			fmt.Println("RNG divergence:", rndMismatch)
			os.Exit(1)
		}
		if c.Unported != 0 {
			fmt.Printf("ticks 0-%d match; tick %d reached unported code at $%04X\n", r.tick, r.tick, c.Unported)
			return
		}
	}
	fmt.Printf("all %d ticks match\n", n)
}

type keyEvent struct {
	row  int
	mask byte
	down bool
}

// loadKeys parses "tick:row:mask:down|up" items separated by spaces or
// newlines, the format scripts/refdump.tcl reads from REFDUMP_KEYS.
func loadKeys(path string) map[int][]keyEvent {
	m := map[int][]keyEvent{}
	if path == "" {
		return m
	}
	b, err := os.ReadFile(path)
	if err != nil {
		fail(err)
	}
	for _, item := range strings.Fields(string(b)) {
		var t, row, mask int
		var dir string
		parts := strings.Split(item, ":")
		if len(parts) != 4 {
			fail(fmt.Errorf("bad key item %q", item))
		}
		fmt.Sscan(parts[0], &t)
		fmt.Sscan(parts[1], &row)
		fmt.Sscan(parts[2], &mask)
		dir = parts[3]
		m[t] = append(m[t], keyEvent{row: row, mask: byte(mask), down: dir == "down"})
	}
	return m
}

// applyKeys presses or releases keyboard matrix bits (active-low).
func applyKeys(c *game.Console, evs []keyEvent) {
	for _, e := range evs {
		if e.down {
			c.Input.KeyRow[e.row] &^= e.mask
		} else {
			c.Input.KeyRow[e.row] |= e.mask
		}
	}
}

type rndEntry struct {
	tick int
	pc   uint16
	val  byte
}

// loadRnd reads the LD A,R log written next to the trace (7-byte records:
// u32 tick, u16 pc, u8 value). A missing log means no R reads are expected.
func loadRnd(path string) []rndEntry {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []rndEntry
	for i := 0; i+7 <= len(b); i += 7 {
		out = append(out, rndEntry{
			tick: int(binary.LittleEndian.Uint32(b[i:])),
			pc:   binary.LittleEndian.Uint16(b[i+4:]),
			val:  b[i+6],
		})
	}
	return out
}

func parse(b []byte) record {
	if !bytes.Equal(b[:4], []byte("TICK")) {
		fail(fmt.Errorf("bad record magic"))
	}
	o := 12
	return record{
		tick: int(binary.LittleEndian.Uint32(b[4:])),
		irq:  int(binary.LittleEndian.Uint32(b[8:])),
		ram:  b[o : o+ramSize],
		vram: b[o+ramSize : o+ramSize+msxrender.VRAMSize],
		reg:  b[o+ramSize+msxrender.VRAMSize:],
	}
}

func diff(c *game.Console, r record) []string {
	var d []string
	for i := 0; i < stackLow; i++ {
		if c.RAM[i] != r.ram[i] {
			d = append(d, fmt.Sprintf("RAM  $%04X: ref %02X port %02X", 0xE000+i, r.ram[i], c.RAM[i]))
		}
	}
	for i := 0; i < 8; i++ {
		if c.Reg[i] != r.reg[i] {
			d = append(d, fmt.Sprintf("R%d: ref %02X port %02X", i, r.reg[i], c.Reg[i]))
		}
	}
	for i := 0; i < msxrender.VRAMSize; i++ {
		if c.VRAM[i] != r.vram[i] {
			d = append(d, fmt.Sprintf("VRAM $%04X: ref %02X port %02X", i, r.vram[i], c.VRAM[i]))
		}
	}
	return d
}

func writePNGs(dir string, c *game.Console, r record) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fail(err)
	}
	var v [msxrender.VRAMSize]byte
	var reg [8]byte
	copy(v[:], r.vram)
	copy(reg[:], r.reg)
	save(filepath.Join(dir, "ref.png"), msxrender.Render(&v, &reg))
	save(filepath.Join(dir, "port.png"), msxrender.Render(&c.VRAM, &c.Reg))
	fmt.Println("wrote", filepath.Join(dir, "ref.png"), "and port.png")
}

func save(path string, img *image.RGBA) {
	f, err := os.Create(path)
	if err != nil {
		fail(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
