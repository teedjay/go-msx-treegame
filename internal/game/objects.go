package game

// Enemies, items and effects run from the game tick. Function names give
// the Z80 entry address; the role in each doc comment is a guess unless it
// says otherwise. Every body is traced instruction by instruction (High
// for the translation) unless a note says otherwise.

func (c *Console) blinkItems()        { c.upd6B53() }
func (c *Console) callBirds()         { c.upd6AEA() }
func (c *Console) updateWalkers()     { c.upd6BF0() }
func (c *Console) updateClimbers()    { c.upd6E03() }
func (c *Console) spawnBeesForBirds() { c.upd6D9E() }
func (c *Console) spawnFallers()      { c.upd7202() }
func (c *Console) spawnWorms()        { c.upd6FE8() }
func (c *Console) updateFallers()     { c.upd705A() }
func (c *Console) updateWorms()       { c.upd7248() }
func (c *Console) updateWormShots()   { c.upd731C() }
func (c *Console) updateShots()       { c.upd7871() }
func (c *Console) updateBees()        { c.upd794A() }
func (c *Console) updateScorePops()   { c.upd7A29() }

// spriteAt returns the address of sprite entry n of a 4-byte table.
// Ported from $4A69/$4A6F (High): base + 4n, 8-bit index arithmetic.
func spriteAt(base uint16, n byte) uint16 { return base + uint16(n*4) }

// slotAt returns object slot n. Ported from $4A75-$4A7D (High).
func slotAt(n byte) uint16 { return 0xE132 + uint16(n+n+n) }

// upd6B53, every 32 ticks: for the 3 tracked items at $E21C (flag, slot,
// counter) show or hide a marker sprite at $E104.
//
// Ported from $6B53-$6BEB (Medium for the role).
func (c *Console) upd6B53() {
	if c.RAM[0x003]&0x1F != 0 {
		return
	}
	hl := uint16(0xE21C)
	for i := byte(0); i < 3; i++ {
		if c.rd(hl) != 0 {
			de := slotAt(c.rd(hl + 1))
			keep := false
			if c.rd(de)-0x18 < 0x78 {
				if t := c.rd(de + 2); t == 0x80 || t == 0x81 {
					c.markItem(hl+1, de+2, i)
					keep = true
				}
			}
			if !keep {
				c.wr(hl+1, 0)
				c.wr(hl, 0)
				c.wr(hl+2, 0)
				c.wr(spriteAt(0xE104, i), 0xC3)
			}
		}
		hl += 3
	}
}

// markItem places (or hides) the marker sprite for tracked item n.
// Ported from $6B9F-$6BEB.
func (c *Console) markItem(hl, de uint16, n byte) {
	if c.rd(hl-1)&0x80 != 0 {
		return
	}
	cnt := c.inc(hl + 1)
	if c.RAM[0x23A] != 0 || cnt&0x40 == 0 {
		c.wr(spriteAt(0xE104, n), 0xC3)
		return
	}
	colour := c.rd(0x6BEC + uint16((cnt&6)>>1))
	slot := de - 2
	sp := spriteAt(0xE104, n)
	c.wr(sp, c.rd(slot)+0x0D)
	x := c.rd(slot + 1)
	b := byte(5)
	if c.rd(slot+2) == 0x80 {
		b = 4
	}
	c.wr(sp+1, x-b)
	c.wr(sp+2, 0xA0)
	c.wr(sp+3, colour)
}

// upd6AEA: while the height's low BCD byte is below $50, start tracking
// type $80/$81 objects (via the callback $6B2B).
//
// Ported from $6AEA-$6AF9 + $6B4B-$6B52.
func (c *Console) upd6AEA() {
	if c.RAM[0x1BC]&0xF0 >= 0x50 {
		return
	}
	if c.RAM[0x05F] == 0 || c.RAM[0x23A] != 0 {
		return
	}
	c.forTypePair(0x80, c.track6B2B)
}

// forTypePair calls f(slot, index) for each object slot 1-39 of type a or
// a+1 that is on the play area.
//
// Ported from $6AFD-$6B2A (High); the callback is reached through
// $6B29 (EX DE,HL / JP (HL)).
func (c *Console) forTypePair(a byte, f func(slot uint16, idx byte)) {
	hl := uint16(0xE137)
	for idx := byte(1); idx < 0x28; idx++ {
		if t := c.rd(hl); t == a || t == a+1 {
			if y := c.rd(hl - 2); y != 0xD0 && y-0x18 < 0x78 {
				f(hl-2, idx)
			}
		}
		hl += 3
	}
}

// track6B2B adds object idx to the tracked-item table at $E21C if it is not
// there and a free entry exists. Ported from $6B2B-$6B4A.
func (c *Console) track6B2B(_ uint16, idx byte) {
	if _, ok := c.find3(0xE21D, idx, 3); ok {
		return
	}
	hl, ok := c.find3(0xE21D, 0, 3)
	if !ok {
		return
	}
	c.wr(hl-1, 1)
	c.wr(hl, idx)
	c.wr(hl+1, 0x40)
}

// find3 searches n entries 3 bytes apart from hl for the value a.
// Ported from $6B3E-$6B4A and $6DF6-$6E02 (High).
func (c *Console) find3(hl uint16, a byte, n int) (uint16, bool) {
	for ; n > 0; n-- {
		if c.rd(hl) == a {
			return hl, true
		}
		hl += 3
	}
	return hl, false
}

// upd6D9E, every 256 ticks: for type $93/$94 objects, clone them into a
// free slot tracked in the table at $E1F9.
//
// Ported from $6D9E-$6DAA.
func (c *Console) upd6D9E() {
	if c.RAM[0x003] != 0 {
		return
	}
	c.forTypePair(0x93, c.clone6DAB)
}

// clone6DAB. Ported from $6DAB-$6DF5.
func (c *Console) clone6DAB(_ uint16, idx byte) {
	if _, ok := c.find3(0xE1F9, idx, 5); ok {
		return
	}
	ix, ok := c.find3(0xE1F9, 0, 5)
	if !ok {
		return
	}
	de := uint16(0xE135)
	free := byte(1)
	for ; free < 0x28; free++ {
		if c.rd(de) == 0xD0 {
			break
		}
		de += 3
	}
	if free == 0x28 {
		return
	}
	if _, ok := c.find3(0xE1FA, free, 5); ok {
		return
	}
	c.wr(ix+1, free)
	c.wr(ix, idx)
	c.ldir(slotAt(idx), de, 3)
	t, b := byte(0x8E), byte(0xA0)
	if c.rd(de+2) == 0x93 {
		t, b = 0x8B, 0x90
	}
	c.wr(de+2, t)
	c.wr(ix+2, b)
}

// upd6E03: run the 5 cloned objects in the table at $E1F9 (orig slot,
// clone slot, state).
//
// Ported from $6E03-$6E6B.
func (c *Console) upd6E03() {
	hl := uint16(0xE1FB)
	for i := 0; i < 5; i++ {
		if c.rd(hl) != 0 {
			c.run6E0F(hl)
		}
		hl += 3
	}
}

func (c *Console) run6E0F(hl uint16) {
	n := c.rd(hl - 1)
	ix := 0xE132 + uint16(n+n+n)
	if c.rd(ix) == 0xD0 {
		c.clear3(hl)
		return
	}
	if t := c.rd(ix+2) - 0x8B; !(t < 4 || t == 5) {
		c.clear3(hl)
		return
	}
	switch {
	case c.rd(hl)&0x80 != 0:
		c.anim6E93(hl, ix)
	case c.shotHits6E6C(ix):
		y, x := c.rd(ix), c.rd(ix+1)
		c.remove6F78(hl, ix)
		c.scorePop(0x04, x, y)
	case c.rd(hl)&0x40 != 0:
		c.walk6EE5(hl, ix)
	default:
		c.climb6F90(hl, ix)
	}
}

// clear3 zeroes the 3-byte entry ending at hl. Ported from $6ED6-$6EDC.
func (c *Console) clear3(hl uint16) {
	c.wr(hl, 0)
	c.wr(hl-1, 0)
	c.wr(hl-2, 0)
}

// shotHits6E6C tests object slot ix against the 3 shot sprites.
// Ported from $6E6C-$6E92.
func (c *Console) shotHits6E6C(ix uint16) bool {
	b, x, t := c.rd(ix), c.rd(ix+1), c.rd(ix+2)
	d, e := byte(4), byte(4)
	if t == 0x8C {
		d = 0
	}
	if t == 0x90 {
		e = 8
	}
	return c.shotHits(b-d, x-e, 0x10, 0x10)
}

// shotHits tests the box at (y, x) of size (d, e) against the 3 shot
// sprites at $E114. Ported from $70DA-$70F6.
func (c *Console) shotHits(y, x, d, e byte) bool {
	hl := uint16(0xE114)
	for i := 0; i < 3; i++ {
		sy := c.rd(hl)
		if sy < 0xC0 && sy-y < d && c.rd(hl+1)-x < e {
			return true
		}
		hl += 4
	}
	return false
}

// anim6E93. Ported from $6E93-$6ED5 and $6EDD-$6EE4.
func (c *Console) anim6E93(hl, ix uint16) {
	if c.RAM[0x003]&0x0F != 0 {
		return
	}
	st := c.inc(hl)
	if st&0x02 != 0 {
		c.drawObject(slotAt(c.rd(hl - 2)))
	} else {
		c.drawObject(ix)
	}
	st = c.rd(hl)
	lim := byte(0x0B)
	if c.RAM[0x051] >= 3 {
		lim = 6
	}
	if st&0x0F != lim {
		return
	}
	c.wr(hl, st&0x70|0x40)
	c.wr(ix, c.rd(ix)-8)
	dx := byte(0x08)
	if st&0x10 != 0 {
		dx = 0xE8
	}
	c.wr(ix+1, c.rd(ix+1)+dx)
	c.wr(ix+2, 0x8C)
	c.drawObject(ix)
}

// walk6EE5. Ported from $6EE5-$6F8F.
func (c *Console) walk6EE5(hl, ix uint16) {
	b := c.RAM[0x003]
	if b&0x0F != 0 {
		return
	}
	if b&0x1F == 0 && c.rd(hl)&0x0F != 0x0F {
		c.inc(hl)
	}
	dx := byte(0x14)
	if c.rd(hl)&0x10 != 0 {
		dx = 0xF8
	}
	x := dx + c.rd(ix+1)
	y := c.rd(ix)
	if y-0x28 < 0x78 {
		t := c.tileAt(y, x)
		if t != 3 {
			c.blocked6F67(hl, ix, t)
			return
		}
		de := nameAddr(y, x)
		for i := 0; i < 4; i++ {
			de += 0x20
			if t := c.readVRAM(de); t != 3 {
				c.turn6F85(hl, t)
				return
			}
		}
	}
	c.eraseObject(ix)
	step := byte(0)
	if c.rd(ix)-0x28 < 0x78 {
		step = 4
		if c.rd(hl)&0x10 != 0 {
			step = 0xFC
		}
	}
	nx := c.rd(ix+1) + step
	c.wr(ix+1, nx)
	if step&0x04 != 0 {
		c.wr(ix+2, 0x8C)
		if nx&0x04 != 0 {
			c.wr(ix+2, 0x8D)
		}
		c.drawMoved(ix)
		return
	}
	c.toggleFrame(ix)
}

// toggleFrame alternates type $8C/$90 and redraws. Ported from $6F54-$6F66.
func (c *Console) toggleFrame(ix uint16) {
	if c.rd(ix+2) == 0x8C {
		c.wr(ix+2, 0x90)
	} else {
		c.wr(ix+2, 0x8C)
	}
	c.drawMoved(ix)
}

// drawMoved redraws the object and plays sound 1. Ported from $6F5D-$6F66.
func (c *Console) drawMoved(ix uint16) {
	c.drawObject(ix)
	c.playSound(0x01)
}

// blocked6F67. Ported from $6F67-$6F8F.
func (c *Console) blocked6F67(hl, ix uint16, t byte) {
	if t == 0x86 || t == 0x8A {
		if c.rd(hl)&0x0F == 0x0F {
			c.remove6F78(hl, ix)
			return
		}
		c.flipDir(hl)
		return
	}
	c.turn6F85(hl, t)
}

// remove6F78 clears the entry and erases and frees the object.
// Ported from $6F78-$6F82.
func (c *Console) remove6F78(hl, ix uint16) {
	c.clear3(hl)
	c.eraseObject(ix)
	c.wr(ix, 0xD0)
}

// turn6F85. Ported from $6F85-$6F8F.
func (c *Console) turn6F85(hl uint16, t byte) {
	if t == 0x9B {
		c.wr(hl, c.rd(hl)&^0x40)
	}
	c.flipDir(hl)
}

// flipDir toggles bits 4 and 5. Ported from $6F8B-$6F8F.
func (c *Console) flipDir(hl uint16) { c.wr(hl, c.rd(hl)^0x30) }

// climb6F90. Ported from $6F90-$6FD1.
func (c *Console) climb6F90(hl, ix uint16) {
	if c.RAM[0x003]&0x0F != 0 {
		return
	}
	dy := byte(0x24)
	if c.rd(hl)&0x10 != 0 {
		dy = 0xF8
	}
	y := c.rd(ix) + dy
	x := c.rd(ix + 1)
	for i := 0; i < 2; i++ {
		if c.tileAt(y, x) != 3 {
			c.flipDir(hl)
			return
		}
		x += 8
	}
	c.eraseObject(ix)
	step := byte(4)
	if c.rd(hl)&0x10 != 0 {
		step = 0xFC
	}
	c.wr(ix, c.rd(ix)+step)
	c.toggleFrame(ix)
}

// eraseObject draws the blank shape (all tile 3) over an object.
// Ported from $6FD2-$6FDB.
func (c *Console) eraseObject(slot uint16) {
	_, b, w := c.shapeInfo(slot)
	c.drawShapeFrom(slot, 0x6FDC, b, w)
}

// upd6FE8: from stage 5, now and then start a falling object at a type
// $93/$94 object on screen.
//
// Ported from $6FE8-$7001.
func (c *Console) upd6FE8() {
	st := c.RAM[0x051]
	if st < 5 || c.RAM[0x05E]&1 != 0 {
		return
	}
	mask := byte(0xFF)
	if st >= 8 {
		mask = 0x7F
	}
	if c.RAM[0x003]&mask != 0 {
		return
	}
	c.spawn7002()
}

// spawn7002. Ported from $7002-$7059.
func (c *Console) spawn7002() {
	de := uint16(0xE132)
	for i := 0; i < 0x28; i++ {
		y := c.rd(de)
		if y != 0xD0 && y-0x18 < 0x88 && c.rd(de+2)-0x93 < 2 {
			c.start7034(de)
			return
		}
		de += 3
	}
}

func (c *Console) start7034(slot uint16) {
	hl := uint16(0xE208)
	n := byte(0)
	for ; n < 3; n++ {
		if c.rd(hl) == 0 {
			break
		}
		hl += 5
	}
	if n == 3 {
		return
	}
	sp := spriteAt(0xE0E4, n)
	c.ldir(slot, sp, 2)
	c.wr(sp+2, 0x90)
	c.wr(sp+3, 0x0F)
	c.wr(hl, 0x88)
	v := c.randR(0x704B) & 7
	if v == 0 {
		v = 5
	}
	c.wr(hl+1, v)
	c.wr(hl+2, v)
	c.wr(hl+3, 0)
}

// upd705A: run the 3 falling objects at $E208 (5 bytes each) with sprites
// at $E0E4.
//
// Ported from $705A-$70C7.
func (c *Console) upd705A() {
	de := uint16(0xE208)
	for n := byte(0); n < 3; n++ {
		if b := c.rd(de); b != 0 {
			c.run7066(de, n, b)
		}
		de += 5
	}
}

func (c *Console) run7066(ix uint16, n, b byte) {
	sp := spriteAt(0xE0E4, n)
	if !c.onScreen70D3(sp, ix) {
		return
	}
	if b&0x80 != 0 {
		c.dying71BF(sp, ix, b)
		return
	}
	if b&0x40 != 0 {
		c.walk70F7(sp, ix, b)
	} else {
		c.jump714A(sp, ix, b)
	}
	if c.rd(ix)&0x10 != 0 {
		return
	}
	y := c.rd(sp) - 8
	x := c.rd(sp+1) - 4
	if c.shotHits(y, x, 0x0C, 0x0C) {
		c.scorePop(0x04, x, y)
		c.wr(ix, 0x31)
		c.setArc(ix+2, 0x65A3)
	}
}

// onScreen70D3 hides the sprite and frees the entry when off the play area.
// Ported from $70D3-$70D9 + $70CD-$70D2.
func (c *Console) onScreen70D3(sp, de uint16) bool {
	if c.rd(sp)-8 < 0xB9 {
		return true
	}
	c.wr(sp, 0xC3)
	c.wr(de, 0)
	return false
}

// onScreen70C8 is the same with a $10 margin. Ported from $70C8-$70D2.
func (c *Console) onScreen70C8(sp, de uint16) bool {
	if c.rd(sp)-0x10 < 0xB1 {
		return true
	}
	c.wr(sp, 0xC3)
	c.wr(de, 0)
	return false
}

// walk70F7. Ported from $70F7-$7149.
func (c *Console) walk70F7(sp, ix uint16, b byte) {
	if c.RAM[0x003]&3 != 0 {
		return
	}
	if d := b & 0x0C; d != 0 {
		if d&0x08 != 0 {
			c.inc(sp + 1)
		} else {
			c.dec(sp + 1)
		}
	}
	x := c.rd(sp + 1)
	p := byte(0x24)
	if x&0x04 != 0 {
		p = 0x20
	}
	if b&0x04 != 0 {
		p += 0x28
	}
	c.wr(sp+2, p)
	if c.onLedge71A2(sp, ix) {
		return
	}
	c.wr(ix, c.rd(ix)^0x0C)
	if c.dec(ix+2) != 0 {
		return
	}
	f := byte(0x28)
	if c.rd(ix)&0x08 != 0 {
		f = 0x24
	}
	c.wr(ix, f)
	c.wr(ix+2, c.rd(ix+1))
	if c.randR(0x7139)&0x10 != 0 {
		c.setArc(ix+2, 0x65A7)
	} else {
		c.setArc(ix+2, 0x65A3)
	}
}

// setArc stores a jump-arc table pointer at hl+1. Ported from $7142-$7149.
func (c *Console) setArc(hl, p uint16) { c.wr16(hl+1, p) }

// onLedge71A2 reports whether the tile under the sprite's leading foot is
// a ledge ($CD-$D0). Ported from $71A2-$71BE.
func (c *Console) onLedge71A2(sp, de uint16) bool {
	y := c.rd(sp) + 0x10
	x := c.rd(sp + 1)
	if c.rd(de)&0x08 != 0 {
		x += 0x10
	}
	return c.tileAt(y, x)-0xCD < 4
}

// jump714A. Ported from $714A-$71A1.
func (c *Console) jump714A(sp, ix uint16, b byte) {
	if c.RAM[0x003]&1 == 0 {
		return
	}
	de := c.rd16(ix + 3)
	step := byte(1)
	dy := c.rd(de)
	if b&1 == 0 {
		dy = -dy
		step = 2
	}
	c.wr(sp, c.rd(sp)+dy)
	if d := b & 0x0C; d != 0 {
		if d&0x08 != 0 {
			c.wr(sp+1, c.rd(sp+1)+step)
		} else {
			c.wr(sp+1, c.rd(sp+1)-step)
		}
	}
	if b&1 != 0 {
		de--
	} else {
		de++
	}
	switch c.rd(de) {
	case 0xFF:
		de -= 2
		c.wr(ix, c.rd(ix)|1)
	case 0xFE:
		de++
	}
	c.wr16(ix+3, de)
	if !c.onLedge71A2(sp, ix) {
		return
	}
	c.wr(sp, c.rd(sp)&0xF8)
	c.wr(ix, c.rd(ix)&0x0C|0x40)
}

// dying71BF. Ported from $71BF-$71F3.
func (c *Console) dying71BF(sp, ix uint16, _ byte) {
	if c.RAM[0x003]&7 != 0 {
		return
	}
	b := c.animSprite(ix+3, sp)
	lim := byte(0x10)
	if c.RAM[0x051] >= 3 {
		lim = 5
	}
	if lim != b {
		return
	}
	c.wr(sp+2, 0x20)
	c.wr(sp+3, 1)
	if c.RAM[0x05D] == 0x11 {
		c.wr(sp+3, c.randR(0x71E6)&7+3)
	}
	c.wr(ix, c.rd(ix)&0x0F|0x40)
}

// animSprite counts the frame counter at de up and, on even steps, flips
// pattern bit 2 of the sprite at sp. It returns the new count.
// Ported from $71F4-$7201.
func (c *Console) animSprite(de, sp uint16) byte {
	b := c.inc(de)
	if b&0x02 == 0 {
		c.wr(sp+2, c.rd(sp+2)^0x04)
	}
	return b
}

// upd7202, once every 256 ticks on odd stages: start one of 2 objects at
// $E217 whose sprite pair at $E0C8/$E0C4 is low enough.
//
// Ported from $7202-$7247.
func (c *Console) upd7202() {
	if c.RAM[0x003] != 0xFF || c.RAM[0x05E]&1 == 0 {
		return
	}
	hl, de := uint16(0xE217), uint16(0xE0C8)
	for n := byte(0); n < 2; n++ {
		if c.rd(hl) == 0 && c.rd(de) >= 0x40 {
			c.wr(hl, c.randR(0x7227)&0x60|0x80)
			c.ldir(0x723A, 0xE0C0+uint16(n*8), 4)
			return
		}
		hl++
		de -= 4
	}
}

// upd7248: move the 2 objects at $E217 with their sprite pairs at $E0C0.
// Ported from $7248-$72AB.
func (c *Console) upd7248() {
	hl := uint16(0xE217)
	for n := byte(0); n < 2; n++ {
		if c.rd(hl) != 0 {
			c.run7254(hl, n)
		}
		hl++
	}
}

func (c *Console) run7254(e uint16, n byte) {
	b := c.RAM[0x003]
	if b&0x3F == 0 && c.randR(0x725C)&7 == 0 {
		c.wr(e, c.rd(e)|0x10)
	}
	sp := 0xE0C0 + uint16(n*8)
	c.onScreen70C8(sp, e)
	if b&7 == 0 {
		turn := false
		if a := c.rd(e) & 0x60; a != 0 {
			if a&0x40 != 0 {
				c.inc(sp + 1)
				b = 0xC7
			} else {
				c.dec(sp + 1)
				b = 0x18
			}
		}
		if c.rd(sp+1) == b {
			turn = true
		} else if c.RAM[0x051] >= 3 && c.randR(0x728C)&0x7F == 0 {
			turn = true
		}
		if turn {
			v := c.rd(e) ^ 0x60
			if v&0x60 == 0 {
				v |= 0x20
			}
			c.wr(e, v)
			c.inc(sp)
		}
	}
	c.pair72AC(sp, e)
}

// pair72AC mirrors the first sprite of a pair into the second, cycles the
// colours of a flashing object and, at the end of the flash, drops a shot.
// Ported from $72AC-$731B.
func (c *Console) pair72AC(sp, e uint16) {
	c.ldir(sp, sp+4, 4)
	c.wr(sp+6, 0x9C)
	c.wr(sp+5, c.rd(sp+5)+0x10)
	if c.RAM[0x003]&3 != 0 || c.rd(e)&0x10 == 0 {
		return
	}
	var a byte
	for de := sp + 3; de <= sp+7; de += 4 {
		a = (c.rd(de) + 1) & 0x0F
		if a == 0 {
			a = 1
		}
		c.wr(de, a)
	}
	if a != 0x0F {
		return
	}
	c.wr(e, c.rd(e)&^0x10)
	hl := uint16(0xE219)
	n := byte(0)
	for ; n < 3; n++ {
		if c.rd(hl) == 0 {
			break
		}
		hl++
	}
	if n == 3 {
		return
	}
	c.wr(hl, c.rd(e))
	c.wr(hl, c.randR(0x72FB)&3|c.rd(hl))
	s := spriteAt(0xE0F8, n)
	c.ldir(sp+4, s, 4)
	c.wr(s, c.rd(s)+0x10)
	c.wr(s+2, 0xF4)
	c.playSound(0x4A)
}

// upd731C: move the 3 dropped shots at $E219 with sprites at $E0F8.
// Ported from $731C-$7364.
func (c *Console) upd731C() {
	hl := uint16(0xE219)
	for n := byte(0); n < 3; n++ {
		if b := c.rd(hl); b != 0 {
			sp := spriteAt(0xE0F8, n)
			c.inc(sp)
			dx := b & 3
			if b&0x40 == 0 {
				dx = -dx
			}
			x := c.rd(sp+1) + dx
			c.wr(sp+1, x)
			col := c.inc(sp + 3)
			c.wr(sp+3, (col|0x08)&^0x80) // INC, SET 3, RES 7
			c.cull7350(sp, hl, x)
		}
		hl++
	}
}

// cull7350 hides a shot that leaves the play area. Ported from $7350-$7364.
func (c *Console) cull7350(sp, de uint16, x byte) {
	if c.rd(sp)-0x18 < 0xA9 && x >= 2 && x < 0xF0 {
		return
	}
	c.wr(sp, 0xC3)
	c.wr(de, 0)
}

// playerOnBonus finds a type $C0 object touching the player and returns
// its slot+1 (as the original leaves HL).
// Ported from $7805-$7821.
func (c *Console) playerOnBonus() (uint16, bool) {
	hl := uint16(0xE134)
	for i := 0; i < 0x28; i++ {
		if c.rd(hl) == 0xC0 && c.overlapsPlayer(hl-2, 0x08, 0x10) {
			return hl - 1, true
		}
		hl += 3
	}
	return 0, false
}

// collectBonus scores the object at slot hl-1 and turns it into a shot
// sprite at $E114 travelling in the player's direction.
// Ported from $7822-$7870.
func (c *Console) collectBonus(hl uint16) {
	x := c.rd(hl)
	hl--
	y := c.rd(hl)
	c.eraseObject(hl)
	c.scorePop(0x00, x-0x10, y-8)
	de := uint16(0xE225)
	n := byte(0)
	for ; n < 3; n++ {
		if c.rd(de) == 0 {
			break
		}
		de++
	}
	if n < 3 {
		dir := c.RAM[0x1B7]
		if c.RAM[0x1B2] == 5 {
			dir = c.RAM[0x1B6]
		}
		c.wr(de, dir|0xC0)
		sp := spriteAt(0xE114, n)
		c.wr(sp, c.rd(hl))
		c.wr(sp+1, c.rd(hl+1)-4)
		c.wr(sp+2, 0xAC)
		c.wr(sp+3, 0x06)
	}
	c.wr(hl, 0xD0)
}

// upd7871: move the 3 shots at $E225 with sprites at $E114.
// Ported from $7871-$789B.
func (c *Console) upd7871() {
	de := uint16(0xE225)
	for n := byte(0); n < 3; n++ {
		if b := c.rd(de); b != 0 {
			sp := spriteAt(0xE114, n)
			if c.onScreen70C8(sp, de) {
				if b&0x40 != 0 {
					c.fall789C(sp, de)
				} else {
					c.fly78BC(sp, de)
				}
			}
		}
		de++
	}
}

// fall789C. Ported from $789C-$78BB.
func (c *Console) fall789C(sp, de uint16) {
	y := c.rd(sp) + 4
	c.wr(sp, y)
	if y&0x04 != 0 {
		c.inc(sp + 1)
	} else {
		c.dec(sp + 1)
	}
	if c.tileAt(y+0x10, c.rd(sp+1)) == 3 {
		return
	}
	c.wr(de, c.rd(de)&^0x40)
}

// fly78BC. Ported from $78BC-$7949.
func (c *Console) fly78BC(sp, de uint16) {
	b := c.rd(de)
	dx := byte(0xFE)
	if b&0x08 != 0 {
		dx = 2
	}
	x := c.rd(sp+1) + dx
	c.wr(sp+1, x)
	if x&0x02 != 0 {
		c.dec(sp)
	} else {
		c.inc(sp)
	}
	ix := sp
	l := c.rd(sp) + 0x14
	h := c.rd(sp + 1)
	if b&0x08 == 0 {
		h += 0x10
	}
	if c.tileAt(l, h)-0xCD < 4 {
		return
	}
	l -= 8
	if c.tileAt(l, h)-0x8C >= 3 {
		c.wr(de, c.rd(de)|0x40)
		return
	}
	c.wr(de, 0)
	c.wr(ix, 0xC3)
	c.hitBird7905(l, h)
}

// hitBird7905: a shot hit a bird tile at (y, x); if a tracked item marker
// is near, mark it and release bees, else maybe start a faller.
// Ported from $7905-$7949.
func (c *Console) hitBird7905(y, x byte) {
	hl := uint16(0xE104)
	for n := byte(0); n < 3; n++ {
		if y-c.rd(hl) < 0x10 && x-c.rd(hl+1) < 0x10 {
			if c.rd(hl+3) == 0 {
				c.spawn7002()
				return
			}
			c.wr(0xE21C+uint16(n+n+n), 0x80)
			cnt := c.RAM[0x239] + 1
			if cnt >= 3 {
				return
			}
			c.RAM[0x239] = cnt
			if cnt == 2 {
				return
			}
			c.RAM[0x23A] = 0x80
			c.RAM[0x23B] = n
			c.wr(hl+2, 0x90)
			return
		}
		hl += 4
	}
	c.spawn7002()
}

// upd794A: run the falling item at RAM[$23A] (sprite at $E104+4n), and when
// it lands drop the bees.
// Ported from $794A-$79DE.
func (c *Console) upd794A() {
	b := c.RAM[0x23A]
	if b == 0 {
		return
	}
	sp := spriteAt(0xE104, c.RAM[0x23B])
	if !c.onScreen70C8(sp, 0xE23A) {
		c.RAM[0x239] = 0
		return
	}
	if b&0x80 != 0 {
		if c.RAM[0x003]&7 != 0 {
			return
		}
		b = c.animSprite(0xE23A, sp)
		if b&0x08 != 0 {
			c.RAM[0x23A] = 0x40
		}
		return
	}
	y := c.rd(sp)
	x := c.rd(sp + 1)
	off := byte(0xC8)
	if x&1 != 0 {
		off = 0xF8
	}
	typ := byte(0xC5)
	switch c.rd(sp + 3) {
	case 0x04:
	case 0x0F:
		typ = 0xC6
	default:
		typ = 0xC7
	}
	c.RAM[0x240] = typ
	c.RAM[0x23E], c.RAM[0x23F] = y, x+off
	c.wr(sp, 0xC3)
	hl := uint16(0xE23C)
	for {
		de := uint16(0xE132)
		n := byte(0)
		for ; n < 0x28; n++ {
			if c.rd(de) == 0xD0 {
				break
			}
			de += 3
		}
		if n < 0x28 {
			c.wr(hl, n)
			c.ldir(0xE23E, de, 3)
			nx := c.rd(de+1) + 0x18
			c.wr(de+1, nx)
			c.RAM[0x23F] = nx
			c.drawObject(de)
		}
		hl++
		c.RAM[0x239]--
		if c.RAM[0x239] == 0 {
			break
		}
	}
	c.RAM[0x23A] = 0
	c.playSound(0x07)
}

// scorePop shows score sprite typ at (y, x) in one of 4 slots at $E120 and
// adds its value (hundreds) to the score.
//
// Ported from $79DF-$7A18 (High). Quirk kept: with all 4 slots busy the
// sprite goes to slot 4, $E130-$E133, overlapping the object table.
func (c *Console) scorePop(typ, x, y byte) {
	hl := uint16(0xE252)
	n := byte(0)
	for ; n < 4; n++ {
		if c.rd(hl) == 0 {
			c.wr(hl, c.rd(hl)|0x80)
			break
		}
		hl++
	}
	sp := spriteAt(0xE120, n)
	t := 0x7A19 + uint16(typ+typ)
	c.wr(sp, y)
	c.wr(sp+1, x)
	c.wr(sp+2, c.rd(t))
	c.wr(sp+3, 0x0F)
	c.addScore(uint16(c.rd(t+1)) << 8)
	c.playSound(0x04)
}

// upd7A29, every 16 ticks: age the score sprites and hide them after 4
// steps. Ported from $7A29-$7A4D.
func (c *Console) upd7A29() {
	if c.RAM[0x003]&0x0F != 0 {
		return
	}
	hl := uint16(0xE252)
	for n := byte(0); n < 4; n++ {
		if c.rd(hl)&0x80 != 0 {
			if c.inc(hl)&0x04 != 0 {
				c.wr(hl, 0)
				c.wr(spriteAt(0xE120, n), 0xC3)
			}
		}
		hl++
	}
}

// upd6BF0: move the 3 special enemies at $E1DE (see spawnSpecial) when no
// scroll happens this tick.
//
// Ported from $6BF0-$6C62. Medium: when the enemy does not move this tick
// the original tests BIT 2,E with E left over from earlier code; the port
// uses the enemy's current low y byte, which is what E holds when it does
// move. The lockstep trace checks this.
func (c *Console) upd6BF0() {
	if c.RAM[0x1AA] != 0 {
		return
	}
	hl := uint16(0xE1DE)
	for i := 0; i < 3; i++ {
		if c.rd(hl) != 0 || c.rd(hl+1) != 0 {
			c.run6C05(hl + 2)
		}
		hl += 9
	}
}

func (c *Console) run6C05(e2 uint16) {
	fl := c.rd(e2)
	mask := byte(7)
	if fl&0x40 != 0 || fl&0x10 != 0 {
		mask = 3
	}
	if c.RAM[0x003]&mask != 0 || fl&0x20 != 0 {
		return
	}
	d := uint16(4)
	if fl&0x80 != 0 {
		d = 0xFFFC
	}
	e3 := e2 + 1
	if fl&0x08 == 0 {
		c.addBE16(e3, d)
	}
	e := c.rd(e3 + 1)
	c.wr(e3+3, 0xE1)
	if e&0x04 == 0 {
		c.wr(e3+3, 0xE0)
		if fl&0x40 != 0 || c.randR(0x6C42)&0x0F == 0 {
			c.wr(e3+3, 0xE2)
		}
	}
	c.redrawSpecial(e3)
	c.aim6CD2(e2)
	c.wall6D2D(e3)
	c.cull6D72(e3)
}

// addBE16 adds d to the big-endian word at hl (high byte first).
// Ported from $6CA3-$6CAC (High).
func (c *Console) addBE16(hl, d uint16) {
	v := uint16(c.rd(hl))<<8 | uint16(c.rd(hl+1))
	v += d
	c.wr(hl+1, byte(v))
	c.wr(hl, byte(v>>8))
}

// offScreen reports whether the big-endian y at hl is outside -16..$BF.
// Ported from $6CBF-$6CD1 (High): carry from (y-$C0)-$FF30.
func (c *Console) offScreen(hl uint16) bool {
	v := uint16(c.rd(hl))<<8 | uint16(c.rd(hl+1))
	return v-0xC0 < 0xFF30
}

// redrawSpecial draws the enemy if it is on screen, setting entry bit 5
// while it is off. Ported from $6CAD-$6CBE.
func (c *Console) redrawSpecial(e3 uint16) {
	off := c.offScreen(e3)
	c.wr(e3-1, c.rd(e3-1)|0x20)
	if off {
		return
	}
	c.wr(e3-1, c.rd(e3-1)&^0x20)
	c.drawObject(e3 + 1)
}

// aim6CD2 points the enemy at the player. Ported from $6CD2-$6D2C.
func (c *Console) aim6CD2(e2 uint16) {
	a := c.rd(e2)
	if a&0x20 != 0 {
		return
	}
	v := a & 3
	if v == 0 {
		return
	}
	y := c.rd(e2 + 2)
	b := c.rd(e2 + 3)
	c.wr(e2, c.rd(e2)&^0x40)
	if v == 1 {
		if c.RAM[0x1B4]-b >= 8 || c.RAM[0x1B2] != 7 {
			return
		}
		c.wr(e2, (c.rd(e2)|0xC0)&^0x08)
		return
	}
	d, e := byte(0x20), byte(0x40)
	if v == 3 {
		d, e = 0x18, 0x20
		c.wr(e2, c.rd(e2)|0x10)
	}
	b -= d
	diff := c.RAM[0x1B4] - b
	c.wr(e2, c.rd(e2)|0x08)
	if diff >= e {
		return
	}
	c.wr(e2, c.rd(e2)&^0x08)
	e = 4
	if diff >= 0x30 {
		e = 8
	}
	if c.RAM[0x1B7]&e != 0 {
		return
	}
	cy := c.RAM[0x1B3]-0x10 < y
	c.wr(e2, c.rd(e2)&^0x80)
	if cy {
		c.wr(e2, c.rd(e2)|0x80)
	}
}

// wall6D2D reverses the enemy at a wall or the screen edge.
// Ported from $6D2D-$6D71.
func (c *Console) wall6D2D(e3 uint16) {
	fl := c.rd(e3 - 1)
	if fl&0x20 != 0 {
		return
	}
	off := byte(0x28)
	if fl&0x80 != 0 {
		off = 0xF8
	}
	l := off + c.rd(e3+1)
	if l-0x18 >= 0xA8 {
		c.wr(e3-1, c.rd(e3-1)^0x80)
		return
	}
	h := c.rd(e3 + 2)
	n, step := 3, byte(8)
	if fl&0x80 != 0 {
		n, step = 2, 0x10
	}
	for ; n > 0; n-- {
		if c.tileAt(l, h) != 3 {
			c.wr(e3-1, c.rd(e3-1)^0x80)
			return
		}
		h += step
	}
}

// cull6D72 frees the entry when both its position and its spawn position
// are off screen. Ported from $6D72-$6D88 (the LDDR propagates the zero
// through the whole entry).
func (c *Console) cull6D72(e3 uint16) {
	if !c.offScreen(e3) || !c.offScreen(e3+4) {
		return
	}
	for a := e3 - 3; a <= e3+5; a++ {
		c.wr(a, 0)
	}
}

// scrollSpecials moves the special enemies and their spawn positions with
// the scroll. Ported from $6C63-$6CA2.
func (c *Console) scrollSpecials() {
	if c.RAM[0x1AA] == 0 {
		return
	}
	hl := uint16(0xE1DE)
	for i := 0; i < 3; i++ {
		if c.rd(hl) != 0 || c.rd(hl+1) != 0 {
			d := uint16(0xFFF8)
			if c.RAM[0x1AA]&1 != 0 {
				d = 8
			}
			e3 := hl + 3
			c.addBE16(e3, d)
			c.addBE16(e3+4, d)
			c.redrawSpecial(e3)
			c.cull6D72(e3)
		}
		hl += 9
	}
}
