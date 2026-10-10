package game

import (
	"fmt"

	"go-msx-treegame/assets"
	"go-msx-treegame/internal/manifest"
)

// romByte returns the byte at a cartridge address, for the routines that
// address ROM tables arithmetically (embedded pointers, computed indexes).
// The address must fall inside a Raw manifest entry; anything else panics,
// so a read of data that was never extracted fails loudly instead of
// returning a plausible zero.
func romByte(addr uint16) byte {
	for i := range manifest.Entries {
		e := &manifest.Entries[i]
		if e.Kind == manifest.Raw && addr >= e.Addr && int(addr) < int(e.Addr)+e.Len {
			return assets.Get(e.Name)[addr-e.Addr]
		}
	}
	panic(fmt.Sprintf("romByte: $%04X is not covered by any Raw manifest entry", addr))
}

func isRAM(a uint16) bool { return a >= 0xE000 && a < 0xE400 }

// rd reads a CPU address the way the original's pointers can: game RAM for
// $E000-$E3FF, cartridge data for $4000-$7FFF.
func (c *Console) rd(a uint16) byte {
	if isRAM(a) {
		return c.RAM[a-0xE000]
	}
	return romByte(a)
}

// mem is rd, kept for the sound code's wording.
func (c *Console) mem(a uint16) byte { return c.rd(a) }

// wr writes a CPU address. Writes to the cartridge are ignored, as on the
// hardware (the original does this in a few places, see $4010 and $40BE).
func (c *Console) wr(a uint16, v byte) {
	switch {
	case isRAM(a):
		c.RAM[a-0xE000] = v
	case a >= 0x4000 && a < 0x8000:
	default:
		panic(fmt.Sprintf("wr: write to unmapped address $%04X", a))
	}
}

// rd16 reads a little-endian word.
func (c *Console) rd16(a uint16) uint16 { return uint16(c.rd(a)) | uint16(c.rd(a+1))<<8 }

// wr16 writes a little-endian word.
func (c *Console) wr16(a uint16, v uint16) {
	c.wr(a, byte(v))
	c.wr(a+1, byte(v>>8))
}

// ldir copies n bytes upwards one at a time, so an overlapping copy to
// dst=src+1 fills, as Z80 LDIR does. n=0 copies nothing (callers never
// pass BC=0).
func (c *Console) ldir(src, dst uint16, n int) {
	for ; n > 0; n-- {
		c.wr(dst, c.rd(src))
		src++
		dst++
	}
}

// lddr copies n bytes downwards, as Z80 LDDR does.
func (c *Console) lddr(src, dst uint16, n int) {
	for ; n > 0; n-- {
		c.wr(dst, c.rd(src))
		src--
		dst--
	}
}

// inc and dec are INC (HL) / DEC (HL) on a CPU address; they return the
// new value.
func (c *Console) inc(a uint16) byte { v := c.rd(a) + 1; c.wr(a, v); return v }
func (c *Console) dec(a uint16) byte { v := c.rd(a) - 1; c.wr(a, v); return v }

// add8 is ADD A,n returning the result and the carry.
func add8(a, n byte) (byte, bool) { r := uint16(a) + uint16(n); return byte(r), r > 0xFF }

// lt is the carry after CP n: true when a < n unsigned.
func lt(a, n byte) bool { return a < n }

// addDAA is ADD/ADC A,n followed by DAA, exactly as the Z80 computes it.
func addDAA(a, n byte, cin bool) (byte, bool) {
	ci := 0
	if cin {
		ci = 1
	}
	sum := int(a) + int(n) + ci
	h := int(a&0x0F)+int(n&0x0F)+ci > 0x0F
	return daa(byte(sum), h, sum > 0xFF, false)
}

// subDAA is SUB/SBC A,n followed by DAA, exactly as the Z80 computes it.
func subDAA(a, n byte, cin bool) (byte, bool) {
	ci := 0
	if cin {
		ci = 1
	}
	diff := int(a) - int(n) - ci
	h := int(a&0x0F)-int(n&0x0F)-ci < 0
	return daa(byte(diff), h, diff < 0, true)
}

// daa applies the Z80 DAA correction to r given the H and C flags and N
// (subtract).
func daa(r byte, h, c, sub bool) (byte, bool) {
	corr := byte(0)
	if c || r > 0x99 {
		corr |= 0x60
		c = true
	}
	if h || r&0x0F > 9 {
		corr |= 0x06
	}
	if sub {
		return r - corr, c
	}
	return r + corr, c
}
