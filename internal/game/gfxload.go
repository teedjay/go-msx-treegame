package game

import (
	"go-msx-treegame/assets"
	"go-msx-treegame/internal/manifest"
)

// loadFontAt fills $180 colour bytes at colAddr with colour, then copies
// the font to the matching pattern address (colAddr + $2000).
//
// Ported from $44B5-$44C7 (High). The font is copied as $180 bytes (48
// glyphs, character codes from colAddr/8), including the trailing bytes
// that the ROM shares with the text strings at $4743.
func (c *Console) loadFontAt(colAddr uint16, colour byte) {
	c.fillVRAM(colAddr, 0x180, colour)
	c.copyToVRAM(colAddr+0x2000, assets.Get("font_45eb"))
}

// loadFontWhite loads the font in white at characters $30-$5F and copies
// third 0 of patterns and colours to the other thirds.
//
// Ported from $44AB-$44B4 (High).
func (c *Console) loadFontWhite() {
	c.loadFontAt(0x0180, 0xF0)
	c.triplicateAll()
}

// loadStageGraphics loads the play-field tiles and sprites.
//
// Ported from $60FA-$615C (High). Order matters: the tiles are loaded into
// third 0 and triplicated before the sprites, and the mirrored sprites are
// a load-time derivation (manifest entry spr_5bfe_mirror).
//
// The original builds the mirrored sprites in a RAM buffer at $E0B0-$E1EF
// before copying them to VRAM, and later initialisation does not overwrite
// all of it ($E130 keeps a byte), so the port leaves the buffer in RAM
// too. Found by cmd/refcompare and an openMSX write-watchpoint on $E130,
// which named $612F (RR (IX+$00)).
func (c *Console) loadStageGraphics() {
	c.writeRLE("pat_5473")
	c.writeStageColours()
	c.triplicateAll()
	c.copyToVRAM(0x1800, assets.Get("spr_5bfe"))
	mirror := assets.Get("spr_5bfe_mirror")
	copy(c.RAM[0x0B0:], mirror)
	c.copyToVRAM(0x1940, mirror)
	c.writeRLE("spr_5d3e")
}

// writeStageColours writes the colour stream col_5968, replacing every
// colour nibble equal to 3 with the matching nibble of RAM[$05D].
//
// Ported from $615D-$619F (High). The two nibbles are tested separately:
// a high nibble of 3 becomes RAM[$05D]&$F0, a low nibble of 3 becomes
// RAM[$05D]&$0F.
func (c *Console) writeStageColours() {
	sub := c.RAM[0x05D]
	hiSub, loSub := sub&0xF0, sub&0x0F
	e := manifest.Lookup("col_5968")
	for i, f := range e.Files() {
		addr := e.Segments[i].VRAM
		for j, b := range assets.Get(f) {
			hi, lo := b&0xF0, b&0x0F
			if hi == 0x30 {
				hi = hiSub
			}
			if lo == 0x03 {
				lo = loSub
			}
			c.writeVRAM(addr+uint16(j), hi|lo)
		}
	}
}
