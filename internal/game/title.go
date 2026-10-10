package game

// Title sequence: states 0-6 and the attract-mode continuations.

// state0 loads the title graphics.
//
// Ported from $4102-$410E (High). On entry RAM[$004] is 0, so wipeColumn
// returns "done" at once (and cancels the dispatcher's RAM[$003]++).
func (c *Console) state0() {
	if !c.wipeColumn() {
		return
	}
	c.writeVDPRegs()
	c.loadTitleGraphics()
	c.nextState()
}

// state1 scrolls the publisher logo up one row every other frame, then
// draws the text under it.
//
// Ported from $410F-$4120 (High).
func (c *Console) state1() {
	if c.RAM[0x003]&1 == 0 {
		return
	}
	if !c.scrollLogo() {
		return
	}
	c.writeRLE("name_47ef")
	c.advanceState(0)
}

// state2 holds for 256 ticks, then clears the screen and sets a black
// backdrop.
//
// Ported from $4121-$4131 (High); falls through into state3's JR $419E.
func (c *Console) state2() {
	c.RAM[0x004]--
	if c.RAM[0x004] != 0 {
		return
	}
	c.RAM[0x00A] = 0
	c.clearScreen()
	c.writeVDPReg(7, 0xE1)
	c.nextState()
}

// state3 advances at once. Ported from $4132 (High): JR $419E.
func (c *Console) state3() { c.nextState() }

// state4 reveals the title logo one column per tick, draws the menu text,
// and waits until the column counter reaches $34.
//
// Ported from $4134-$413B (High).
func (c *Console) state4() {
	if c.revealTitleStep() {
		return
	}
	c.advanceState(0)
}

// state5 blinks the menu pointer for 256 ticks, then starts the demo.
//
// Ported from $413C-$4144 (High).
func (c *Console) state5() {
	c.RAM[0x004]--
	if c.RAM[0x004] != 0 {
		c.drawMenuPointer(c.RAM[0x004]&0x08 != 0)
		return
	}
	c.advanceState(0x20)
}

// state6 wipes the screen column by column, then sets up the demo.
//
// Ported from $4145-$414D (High).
func (c *Console) state6() {
	if !c.wipeColumn() {
		return
	}
	c.setupPlay()
	c.advanceState(0x20)
}

// wipeColumn clears one name-table column per call, alternating from the
// right and left edges towards the middle, and ends the sprite list. It
// returns true (S flag set) once RAM[$004] has counted below zero.
//
// Ported from $42E0-$4307 (High). RAM[$003] is decremented to cancel the
// dispatcher's increment.
func (c *Console) wipeColumn() bool {
	c.RAM[0x003]--
	c.RAM[0x004]--
	a := c.RAM[0x004]
	if a&0x80 != 0 {
		return true
	}
	col := a >> 1
	if a&1 == 0 {
		col ^= 0x1F
	}
	de := uint16(0x3800) | uint16(col)
	for i := 0; i < 24; i++ {
		c.writeVRAM(de, 0)
		de += 0x20
	}
	c.endSprites()
	return false
}

// endSprites writes the $D0 terminator to sprite 0's Y.
// Ported from $42FE-$4307 (High).
func (c *Console) endSprites() { c.writeVRAM(0x3B00, 0xD0) }

// clearScreen ends the sprite list and clears the name table.
//
// Ported from $444F-$4458 (High). It falls through into the fill at $4459
// with DE=$7800, which the VDP address masks to $3800.
func (c *Console) clearScreen() {
	c.endSprites()
	c.fillVRAM(0x7800, 0x300, 0)
}

// writeVDPReg writes one VDP register (BIOS WRTVDP).
//
// Ported from $4534-$4539 + $4058-$4060 (High). $4058 also stores $0000 to
// $413A, which is ROM: no effect.
func (c *Console) writeVDPReg(r, v byte) { c.Reg[r&7] = v }

// loadTitleGraphics loads the publisher logo, both fonts and the title
// logo.
//
// Ported from $427D-$4284 (High), continuing at $428A.
func (c *Console) loadTitleGraphics() {
	c.loadPublisherLogo()
	c.loadFontWhite()
	c.loadTitleLogo()
}

// loadPublisherLogo sets up the logo scroll and loads its tiles.
//
// Ported from $48CD-$48EE (High). $4010-$401B, which $48D8 jumps to, copies
// two bytes into $4121/$4122 (ROM: no effect) before jumping to $48DB.
func (c *Console) loadPublisherLogo() {
	c.RAM[0x00A] = 0x0E
	c.RAM[0x00E], c.RAM[0x00F] = 0xAA, 0x3A // $3AAA: one row below the start
	c.writeRLEAt("pat_4923", 0x2300)
	c.fillVRAM(0x0300, 0xD8, 0xF0)
}

// loadTitleLogo loads the cyan font into the bottom third and the title
// logo tiles with 16 copies of their colour block.
//
// Ported from $428A-$42AE (High).
func (c *Console) loadTitleLogo() {
	c.loadFontAt(0x1600, 0x70)
	c.writeRLE("pat_4800")
	for de := uint16(0x0600); de < 0x0700; de += 0x10 {
		c.writeRLEAt("col_48c2", de)
	}
}

// scrollLogo moves the publisher logo up one row: 26 tiles ($60-$79) in
// rows of 3, 11 and 12, then blanks the 12 tiles below. It returns true
// when the 14th step is done.
//
// Ported from $48EF-$4922 (High). The blanking fill uses BC as the tile
// loop leaves it: B=0 from DJNZ, C=$0C.
func (c *Console) scrollLogo() bool {
	hl := (uint16(c.RAM[0x00E]) | uint16(c.RAM[0x00F])<<8) - 0x20
	c.RAM[0x00E], c.RAM[0x00F] = byte(hl), byte(hl>>8)
	de := hl
	ch := byte(0x60)
	for _, n := range []int{3, 11, 12} {
		for i := 0; i < n; i++ {
			c.writeVRAM(de+uint16(i), ch)
			ch++
		}
		de += 0x20
	}
	c.fillVRAM(de, 0x000C, 0)
	c.RAM[0x00A]--
	return c.RAM[0x00A] == 0
}

// revealTitleStep draws one more column of the title logo, or, once all 15
// are drawn, the menu text (on the first call after) and then just counts.
// It returns true (carry) while state 4 should keep waiting.
//
// Ported from $4414-$444E (High).
func (c *Console) revealTitleStep() bool {
	a := c.RAM[0x00A]
	c.RAM[0x00A]++
	if a >= 0x0F {
		if a == 0x0F {
			c.drawText(0x475E, 0xFF)
		}
		return a < 0x34
	}
	de := 0x3889 + uint16(a)
	ch := a*2 + 0xC0
	for i := 0; i < 2; i++ {
		c.writeVRAM(de, ch)
		de += 0x20
		ch++
	}
	if t := byte(de) - 0xCC; t < 2 {
		c.writeVRAM(de, t+0xDE)
	}
	return true
}

// revealTitle draws the whole title logo and menu at once.
// Ported from $4409-$4413 (High).
func (c *Console) revealTitle() {
	c.RAM[0x00A] = 0
	for c.revealTitleStep() {
	}
}

// drawText draws a text table: a 2-byte VRAM address, then characters
// (ANDed with mask: $FF draws, $00 erases), $FE followed by a new address,
// $FF to end.
//
// Ported from $4079-$408D (High). The $4075 entry (mask $00) is
// unreferenced. It returns DE as the original leaves it: one past the last
// character written.
func (c *Console) drawText(addr uint16, mask byte) uint16 {
	hl := addr
	for {
		de := uint16(romByte(hl)) | uint16(romByte(hl+1))<<8
		hl += 2
		for {
			a := romByte(hl)
			hl++
			if a == 0xFF {
				return de
			}
			if a == 0xFE {
				break
			}
			c.writeVRAM(de, a&mask)
			de++
		}
	}
}

// drawMenuPointer draws (on=true) or erases the hand pointer beside the
// selected menu line RAM[$042].
//
// Ported from $42AF-$42C5 (High), with $42C6-$42D0 for the address:
// VRAM $3A00 | RRCA(RRCA(RAM[$042]+$14)), i.e. column 5 of rows 16-22.
func (c *Console) drawMenuPointer(on bool) {
	b, cc := byte(0), byte(0)
	if on {
		b, cc = 0x3E, 0x3F
	}
	de := c.menuPointerAddr()
	c.writeVRAM(de, b)
	c.writeVRAM(de+1, cc)
}

func (c *Console) menuPointerAddr() uint16 {
	return 0x3A00 | uint16(rrca(rrca(c.RAM[0x042]+0x14)))
}

// attractInput reads every controller while no game is running. A fresh
// press jumps to the menu from any title state; on the menu, a direction
// moves the pointer and a trigger starts a game in the selected mode.
//
// Ported from $458A-$45E6 (High).
func (c *Console) attractInput() {
	a := c.readJoystick(0x8F) | c.readJoystick(0xCF) | c.readKeyboard()
	prev := c.RAM[0x040]
	c.RAM[0x040] = a
	c.RAM[0x041] = prev
	pressed := (a ^ prev) & a
	if pressed == 0 {
		return
	}
	c.RAM[0x004] = 0
	if c.RAM[0x000] != 5 {
		c.RAM[0x000] = 5
		c.showMenu()
		return
	}
	if pressed >= 0x10 {
		c.RAM[0x002] = romByte(0x45E7 + uint16(c.RAM[0x042]))
		c.RAM[0x000], c.RAM[0x001] = 0x08, 0x00
		return
	}
	c.drawMenuPointer(false)
	sel := c.RAM[0x042]
	if pressed&BtnUp != 0 {
		sel--
	}
	if pressed&BtnDown != 0 {
		sel++
	}
	c.RAM[0x042] = sel & 3
}

// showMenu jumps straight to the finished menu screen.
//
// Ported from $423F-$4251 (High).
func (c *Console) showMenu() {
	c.playSound(0x9F)
	c.clearScreen()
	c.writeVDPReg(7, 0xE1)
	c.loadTitleLogo()
	c.revealTitle()
	c.RAM[0x001]++
}

// blinkPlayerText blinks the current player's label every 32 ticks during
// a game.
//
// Ported from $43D7-$43F0 (High): mask $00 (erase) when RAM[$003] bit 5 is
// clear, $FF (draw) when set; RAM[$002] bit 7 picks the 2P text.
func (c *Console) blinkPlayerText() {
	if c.RAM[0x003]&0x1F != 0 {
		return
	}
	mask := byte(0)
	if c.RAM[0x003]&0x20 != 0 {
		mask = 0xFF
	}
	addr := uint16(0x4752)
	if c.RAM[0x002]&0x80 != 0 {
		addr = 0x4758
	}
	c.drawText(addr, mask)
}
