package game

// Vertical scrolling, the stage map and background objects.
//
// The stage map lives in spare VRAM from $3B80 as 3-byte records (see
// buildStageMap). RAM[$059]/RAM[$05A-$05B] track the bottom edge of the
// view in the map, RAM[$1C4]/RAM[$1C5-$1C6] the top edge (Medium for the
// names). Background objects (branches, items, enemies drawn as tiles) are
// 40 slots of (y, x, type) at $E132; y=$D0 marks a free slot.
//
// RAM[$1AA]: scroll request this tick (1 = up, 2 = down). RAM[$1AB]:
// bit 0 top of map reached, bit 1 bottom reached, bit 7 freeze drawing
// (Medium).

// movePlayerY moves the player to y=a by adjusting RAM[$1B3] and height.
// Ported from $6737-$673A (High).
func (c *Console) movePlayerY(a byte) { c.movePlayerBy(a - c.RAM[0x1B3]) }

// movePlayerBy adds a to the player's y and subtracts it from the height.
// Ported from $673B-$6743 (High).
func (c *Console) movePlayerBy(a byte) {
	c.RAM[0x1B3] += a
	c.addHeight(-a)
}

// addHeight adds the signed byte a to the height RAM[$054-$055], clamping
// at 0.
//
// Ported from $6744-$675E (High).
func (c *Console) addHeight(a byte) {
	hl := c.rd16(0xE054)
	if a&0x80 == 0 {
		hl += uint16(a)
	} else {
		e := uint16(-a)
		if hl < e {
			hl = 0
		} else {
			hl -= e
		}
	}
	c.wr16(0xE054, hl)
}

// scrollUp and scrollDown request a one-row scroll. Ported from
// $675F-$6764 (High).
func (c *Console) scrollUp()   { c.requestScroll(1) }
func (c *Console) scrollDown() { c.requestScroll(2) }

// requestScroll scrolls one row in direction a (1 up, 2 down) unless the
// matching limit bit in RAM[$1AB] is set.
//
// Ported from $6765-$6779 (High). Quirk kept: RAM[$1AB] is overwritten
// with the masked bits, clearing the others, and with 0 when the scroll
// goes ahead.
func (c *Console) requestScroll(a byte) {
	c.RAM[0x1AA] = a
	m := a & c.RAM[0x1AB]
	c.RAM[0x1AB] = m
	if m != 0 {
		c.RAM[0x1AA] = 0
		return
	}
	if a&1 != 0 {
		c.scrollRowUp()
	} else {
		c.scrollRowDown()
	}
}

// scrollRowUp scrolls the view one row up the tree: new map rows enter at
// the top, objects move down.
//
// Ported from $677A-$67BF (High).
func (c *Console) scrollRowUp() {
	de := c.rd16(0xE1C5)
	n := c.inc(0xE1C4)
	if c.readVRAM(de) == n {
		de++
		if c.readVRAM(de) == 0xFF {
			c.RAM[0x1AB] |= 0x01
		} else {
			c.RAM[0x1C4] = 0
			de = c.spawnObjects(de)
			c.wr16(0xE1C5, de)
		}
	}
	c.moveObjects()
	de = c.rd16(0xE05A)
	if c.readVRAM(de) != c.inc(0xE059) {
		return
	}
	c.RAM[0x059] = 0
	for {
		de += 3
		if c.readVRAM(de) != 0 {
			break
		}
	}
	c.wr16(0xE05A, de)
}

// scrollRowDown scrolls the view one row down the tree.
//
// Ported from $67C0-$681C (High).
func (c *Console) scrollRowDown() {
	de := c.rd16(0xE05A)
	c.dec(0xE059)
	de = c.prevRecord(de)
	if a := c.RAM[0x059]; a == 0 {
		c.scrollDownSpawn(de)
	} else if a&0x80 != 0 {
		v := c.readVRAM(de) - 1
		c.RAM[0x059] = v
		c.wr16(0xE05A, de)
		if v == 0 {
			c.scrollDownSpawn(c.prevRecord(de))
		}
	}
	c.moveObjects()
	if c.dec(0xE1C4)&0x80 == 0 {
		return
	}
	de = c.prevRecord(c.rd16(0xE1C5))
	c.RAM[0x1C4] = c.readVRAM(de) - 1
	c.wr16(0xE1C5, de)
}

// scrollDownSpawn spawns the objects of the record at de (entering at the
// bottom), flagging the bottom of the map when its type matches RAM[$056].
// Ported from $67D5-$67E7 (High).
func (c *Console) scrollDownSpawn(de uint16) {
	de++
	if c.RAM[0x056] == c.readVRAM(de) {
		c.RAM[0x1AB] |= 0x02
	}
	c.spawnObjects(de)
}

// prevRecord steps back 3 bytes at a time to the previous map record with
// a non-zero first byte. Ported from the loops at $67C8, $67F4, $6812
// (High).
func (c *Console) prevRecord(de uint16) uint16 {
	for {
		de -= 3
		if c.readVRAM(de) != 0 {
			return de
		}
	}
}

// spawnObjects puts the objects of the map record at de (and of following
// records whose first byte is 0) into free object slots, entering at the
// top (y=$E8) when scrolling up or the bottom (y=$C0) when scrolling down.
// Type $E0-$FF objects go into the 3-entry table at $E1DE instead, keyed
// by their map address, with a random variant. It returns de pointing at
// the next record's first byte.
//
// Ported from $681D-$68B9 (High). The variant is LD A,R AND B at $686F.
func (c *Console) spawnObjects(de uint16) uint16 {
	for {
		slot := uint16(0xE132)
		found := false
		for i := 0; i < 0x28; i++ {
			if c.rd(slot) == 0xD0 {
				found = true
				break
			}
			slot += 3
		}
		if !found {
			return de
		}
		up := c.RAM[0x1AA]&1 != 0
		if up {
			c.wr(slot, 0xE8)
		} else {
			c.wr(slot, 0xC0)
		}
		c.wr16(0xE1DC, slot)
		typ := c.readVRAM(de)
		de++
		c.wr(slot+1, c.readVRAM(de))
		c.wr(slot+2, typ)
		if typ&0xE0 == 0xE0 {
			c.spawnSpecial(slot, de)
		}
		de++
		if c.readVRAM(de) != 0 {
			return de
		}
		de++
	}
}

// spawnSpecial moves a type $E0+ object from its slot into the table at
// $E1DE (9-byte entries: map address, variant, flag, y x type, flag y).
// Ported from $684E-$68B8 (High).
func (c *Console) spawnSpecial(slot, de uint16) {
	if _, ok := c.findSpecial(de); ok {
		c.wr(slot, 0xD0) // already active
		return
	}
	hl, ok := c.findSpecial(0)
	if !ok {
		return
	}
	c.wr(hl, byte(de>>8))
	c.wr(hl-1, byte(de))
	hl++
	stage := c.RAM[0x051]
	b := byte(3)
	if stage == 1 {
		b = 1
	}
	v := c.randR(0x686F) & b
	c.wr(hl, v)
	if v == 3 && stage == 2 {
		c.wr(hl, 2)
	}
	if c.RAM[0x1AA]&1 == 0 {
		c.wr(hl, c.rd(hl)|0x80)
	}
	hl++
	s := c.rd16(0xE1DC)
	if c.rd(s) == 0xE8 {
		c.wr(hl, 0xFF)
	} else {
		c.wr(hl, 0)
	}
	c.ldir(s, hl+1, 3)
	c.ldir(hl, hl+4, 2)
	c.wr(s, 0xD0)
}

// findSpecial looks for the $E1DE entry whose map address is id, and
// returns the address of its second byte (as the original leaves HL).
//
// Ported from $6D89-$6D9D (High).
func (c *Console) findSpecial(id uint16) (uint16, bool) {
	hl := uint16(0xE1DE)
	for i := 0; i < 3; i++ {
		e := c.rd(hl)
		hl++
		if e == byte(id) && c.rd(hl) == byte(id>>8) {
			return hl, true
		}
		hl += 8
	}
	return hl, false
}

// moveObjects moves every background object 8 pixels for this tick's
// scroll, erasing the row it leaves and redrawing it; objects with type
// bit 6 are drawn again in a second pass so they stay on top. Then the
// special enemies and the sprites follow the scroll.
//
// Ported from $68BA-$696B (High).
func (c *Console) moveObjects() {
	hl := uint16(0xE132)
	for i := 0; i < 0x28; i++ {
		if c.rd(hl) != 0xD0 {
			c.moveObject(hl)
		}
		hl += 3
	}
	if c.RAM[0x1AB]&0x80 != 0 {
		return
	}
	hl = 0xE134
	for i := 0; i < 0x28; i++ {
		if c.rd(hl)&0x40 != 0 && c.rd(hl-2) != 0xD0 {
			c.drawObject(hl - 2)
		}
		hl += 3
	}
	c.scrollSpecials()
	a := c.RAM[0x1AA] & 3
	if a == 0 {
		return
	}
	c.scrollSprites(0xE0E4, 0x11, a)
	c.scrollSprites(0xE0C0, 4, a)
	for i := uint16(0); i < 8; i++ {
		c.writeVRAM(0x3B54+i, c.RAM[0x104+i])
	}
}

// moveObject moves and redraws one object slot. Ported from $68C6-$691C
// (High).
func (c *Console) moveObject(hl uint16) {
	de, b, w := c.shapeInfo(hl)
	if dir := c.RAM[0x1AA]; dir != 0 {
		oldY := c.rd(hl)
		y := oldY - 8
		if dir&1 != 0 {
			y = oldY + 8
		}
		c.wr(hl, y)
		if y-0xC0 < 0x30 {
			c.wr(hl, 0xD0)
		}
		if c.RAM[0x1AB]&0x80 == 0 {
			x := c.rd(hl + 1)
			typ := c.rd(hl + 2)
			ey := oldY
			if dir&1 == 0 {
				ey += (b - 1) * 8
			}
			if typ&0x80 == 0 {
				c.eraseRow(ey, x, w)
			}
		}
	}
	if c.rd(hl+2)&0xC0 == 0xC0 || c.RAM[0x1AB]&0x80 != 0 {
		return
	}
	c.drawShape(c.rd(hl), c.rd(hl+1), de, b, w)
}

// scrollSprites moves n sprite entries at hl 8 pixels with the scroll
// (down for dir bit 1 clear, up when set), hiding those that leave.
//
// Ported from $696C-$6987 (High).
func (c *Console) scrollSprites(hl uint16, n int, dir byte) {
	for ; n > 0; n-- {
		y := c.rd(hl)
		if y-8 >= 0xB9 {
			c.wr(hl, 0xC3)
		} else if dir&2 != 0 {
			c.wr(hl, y-8)
		} else {
			c.wr(hl, y+8)
		}
		hl += 4
	}
}

// shapeInfo returns an object's shape: pointer to its row data, rows (b)
// and width in tiles (w). Types below $80 index the table at $515E; above,
// bits 5 and 6 pick $51B6, $519E or $5174 and bits 0-4 index it.
//
// Ported from $4A3C-$4A68 (High). The index doubling is 8-bit.
func (c *Console) shapeInfo(slot uint16) (de uint16, b, w byte) {
	a := c.rd(slot + 2)
	t := uint16(0x515E)
	if a&0x80 != 0 {
		switch {
		case a&0x20 != 0:
			t = 0x51B6
		case a&0x40 != 0:
			t = 0x519E
		default:
			t = 0x5174
		}
		a &= 0x1F
	}
	p := c.rd16(t + uint16(a+a))
	return p + 2, c.rd(p), c.rd(p + 1)
}

// nameAddr returns the name-table address of pixel (y, x).
// Ported from $49C0-$49D3 (High): $3800 + (y/8)*32 + x/8.
func nameAddr(y, x byte) uint16 {
	return 0x3800 + uint16(y>>3)*32 + uint16(x>>3)
}

// tileAt returns the name-table entry under pixel (y, x).
// Ported from $49BA-$49BF (High).
func (c *Console) tileAt(y, x byte) byte { return c.readVRAM(nameAddr(y, x)) }

// copyRows copies b rows of w tiles from src to the name table at de.
// Ported from $49D4-$49E2 (High).
func (c *Console) copyRows(src, de uint16, b, w byte) {
	for ; b > 0; b-- {
		for i := uint16(0); i < uint16(w); i++ {
			c.writeVRAM(de+i, c.rd(src+i))
		}
		src += uint16(w)
		de += 0x20
	}
}

// drawObject draws the object in slot hl. Ported from $49E3-$49E9 (High).
func (c *Console) drawObject(hl uint16) {
	de, b, w := c.shapeInfo(hl)
	c.drawShape(c.rd(hl), c.rd(hl+1), de, b, w)
}

// drawShapeFrom draws a shape whose position is the word at hl (y, x).
// Ported from $49E6-$49E9 (High).
func (c *Console) drawShapeFrom(hl, de uint16, b, w byte) {
	c.drawShape(c.rd(hl), c.rd(hl+1), de, b, w)
}

// drawShape draws b rows of a shape at pixel (y, x), skipping rows off the
// play area (y-$18 >= $A8). Shapes up to 6 tiles wide store raw rows;
// wider ones store each row as an RLE stream.
//
// Ported from $49EA-$4A2D (High). Skipping an RLE row steps over its
// control bytes: n<$80 skips one value byte, n>=$80 skips n&$7F bytes.
func (c *Console) drawShape(y, x byte, de uint16, b, w byte) {
	for ; b > 0; b-- {
		if y-0x18 < 0xA8 {
			dst := nameAddr(y, x)
			if w < 7 {
				for i := uint16(0); i < uint16(w); i++ {
					c.writeVRAM(dst+i, c.rd(de+i))
				}
				de += uint16(w)
			} else {
				de = c.writeRLEFrom(de, dst)
			}
		} else if w < 7 {
			de += uint16(w)
		} else {
			for {
				a := c.rd(de)
				de++
				if a == 0 {
					break
				}
				if a&0x80 == 0 {
					de++
				} else {
					de += uint16(a & 0x7F)
				}
			}
		}
		y += 8
	}
}

// writeRLEFrom writes the RLE stream at CPU address hl to VRAM at de and
// returns the address after its terminator.
//
// Ported from $44CC-$44E9 (High), for streams read at run time.
func (c *Console) writeRLEFrom(hl, de uint16) uint16 {
	for {
		ctl := c.rd(hl)
		hl++
		n := ctl & 0x7F
		switch {
		case ctl == 0:
			return hl
		case ctl == 0x80:
			de = c.rd16(hl)
			hl += 2
		case ctl&0x80 == 0:
			v := c.rd(hl)
			hl++
			for ; n > 0; n-- {
				c.writeVRAM(de, v)
				de++
			}
		default:
			for ; n > 0; n-- {
				c.writeVRAM(de, c.rd(hl))
				hl++
				de++
			}
		}
	}
}

// eraseRow fills w tiles at pixel (y, x) with blank tile 3, if on the play
// area. Ported from $4A2E-$4A3B (High).
func (c *Console) eraseRow(y, x, w byte) {
	if y-0x18 >= 0xA8 {
		return
	}
	c.fillVRAM(nameAddr(y, x), uint16(w), 3)
}
