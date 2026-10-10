package game

import (
	"go-msx-treegame/assets"
	"go-msx-treegame/internal/manifest"
)

// vdpRegs is the register table at $452C: Screen 2, 16x16 sprites, colour
// table $0000, patterns $2000, names $3800, sprite attributes $3B00, sprite
// patterns $1800, backdrop dark blue. Short constant, so kept inline.
var vdpRegs = [8]byte{0x02, 0xE2, 0x0E, 0x7F, 0x07, 0x76, 0x03, 0xE4}

// writeVDPRegs loads R0-R7 from the register table.
//
// Ported from $451B-$452B (High).
//
// Note: $40BE does RES 6,(HL) with HL=$452D to clear the display-enable bit
// in this table before it is written, but $452D is ROM, so the write has no
// effect. The port reproduces that: the table is constant.
func (c *Console) writeVDPRegs() {
	c.Reg = vdpRegs
}

// writeVRAM writes one byte. Ported from $401F-$4027 (High).
func (c *Console) writeVRAM(addr uint16, v byte) {
	c.VRAM[addr&0x3FFF] = v
}

// readVRAM reads one byte. Ported from $4028-$4030 (High).
func (c *Console) readVRAM(addr uint16) byte {
	return c.VRAM[addr&0x3FFF]
}

// fillVRAM writes n copies of v from addr. n=0 writes 65536 bytes, as the
// original's DEC BC / OR test would; VRAM wraps at 16KB.
//
// Ported from $4459-$4469 (High).
func (c *Console) fillVRAM(addr uint16, n uint16, v byte) {
	for {
		c.writeVRAM(addr, v)
		addr++
		n--
		if n == 0 {
			return
		}
	}
}

// copyToVRAM copies src to VRAM at addr.
//
// Ported from $446E-$447E (High).
func (c *Console) copyToVRAM(addr uint16, src []byte) {
	for i, b := range src {
		c.writeVRAM(addr+uint16(i), b)
	}
}

// copyVRAM copies n bytes from VRAM src to VRAM dst, one byte at a time and
// in ascending order, so overlapping copies propagate as in the original.
//
// Ported from $447F-$448E (High).
func (c *Console) copyVRAM(src, dst uint16, n uint16) {
	for ; n > 0; n-- {
		c.writeVRAM(dst, c.readVRAM(src))
		src++
		dst++
	}
}

// triplicatePatterns copies pattern third 0 to thirds 1 and 2.
//
// Ported from $448F-$449F (High): $2000->$2800 for $800 bytes, then the
// pointers continue, so $2800->$3000 for another $800.
func (c *Console) triplicatePatterns() {
	c.copyVRAM(0x2000, 0x2800, 0x800)
	c.copyVRAM(0x2800, 0x3000, 0x800)
}

// triplicateColours copies colour third 0 to thirds 1 and 2.
//
// Ported from $44A3-$44A9 + $4495-$449F (High).
func (c *Console) triplicateColours() {
	c.copyVRAM(0x0000, 0x0800, 0x800)
	c.copyVRAM(0x0800, 0x1000, 0x800)
}

// triplicateAll copies third 0 of patterns and colours to thirds 1 and 2.
//
// Ported from $44A0-$44A2 (High).
func (c *Console) triplicateAll() {
	c.triplicatePatterns()
	c.triplicateColours()
}

// writeRLE writes an RLE asset whose stream carries its own VRAM addresses,
// at the addresses the manifest records.
//
// Ported from $44C8-$44E9 (High); the decoding itself happens in
// cmd/extract, so here each decoded segment is copied to its address.
func (c *Console) writeRLE(name string) {
	e := manifest.Lookup(name)
	for i, f := range e.Files() {
		c.copyToVRAM(e.Segments[i].VRAM, assets.Get(f))
	}
}

// writeRLEAt writes an RLE asset that has no leading address at dest.
//
// Ported from $44CC-$44E9 (High).
func (c *Console) writeRLEAt(name string, dest uint16) {
	c.copyToVRAM(dest, assets.Get(name))
}
