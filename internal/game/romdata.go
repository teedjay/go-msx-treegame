package game

import (
	"fmt"

	"go-msx-treegame/assets"
	"go-msx-treegame/internal/manifest"
)

// romByte returns the byte at a cartridge address, for the few routines
// that address ROM tables arithmetically (embedded pointers, computed
// indexes). The address must fall inside a Raw manifest entry; anything
// else panics, so a read of data that was never extracted fails loudly
// instead of returning a plausible zero.
func romByte(addr uint16) byte {
	for i := range manifest.Entries {
		e := &manifest.Entries[i]
		if e.Kind == manifest.Raw && addr >= e.Addr && int(addr) < int(e.Addr)+e.Len {
			return assets.Get(e.Name)[addr-e.Addr]
		}
	}
	panic(fmt.Sprintf("romByte: $%04X is not covered by any Raw manifest entry", addr))
}

// mem reads a CPU address the way the original's data pointers can: game
// RAM for $E000-$E3FF, cartridge data otherwise.
func (c *Console) mem(addr uint16) byte {
	if addr >= 0xE000 && addr < 0xE400 {
		return c.RAM[addr-0xE000]
	}
	return romByte(addr)
}
