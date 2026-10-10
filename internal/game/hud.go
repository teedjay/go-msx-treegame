package game

// HUD: scores, lives, stage and height on the top rows.
//
// RAM (High unless noted): $E049-$E04B player 1 score and $E046-$E048
// player 2 score (packed BCD, low byte first); $E043-$E045 hi-score;
// RAM[$050] lives; RAM[$051] stage (BCD); RAM[$052] next extra-life
// threshold (score top byte, BCD); RAM[$054-$055] height in pixels.

// drawHUD draws the score line, lives, height and stage.
//
// Ported from $4370-$439F (High). In a 2-player game the other player's
// score is drawn too (RAM[$002] with bit 7 flipped).
func (c *Console) drawHUD() {
	c.drawText(0x4743, 0xFF)
	if c.RAM[0x002]&0x20 != 0 {
		c.drawText(0x4758, 0xFF)
		c.drawPlayerScore(c.RAM[0x002] ^ 0x80)
	}
	c.drawLives()
	c.drawHeightLabel()
	c.drawStage()
	c.drawHiScore()
	c.drawPlayerScore(c.RAM[0x002])
}

// drawHiScore draws the 3-byte hi-score at $380F.
// Ported from $4394-$439C (High).
func (c *Console) drawHiScore() { c.drawBCD(0xE045, 0x380F, 3) }

// drawPlayerScore draws player 1's score at $3805, or player 2's at $3825
// when bit 7 of mode is set.
//
// Ported from $43A0-$43B0 (High).
func (c *Console) drawPlayerScore(mode byte) {
	if mode&0x80 != 0 {
		c.drawBCD(0xE048, 0x3825, 3)
	} else {
		c.drawBCD(0xE04B, 0x3805, 3)
	}
}

// drawStage draws the stage number at $381C. Ported from $43B2-$43B9 (High).
func (c *Console) drawStage() { c.drawBCD(0xE051, 0x381C, 1) }

// drawBCD draws n packed-BCD bytes, starting with the most significant at
// hl and moving down, as digit characters at VRAM de.
//
// Ported from $43BA-$43D6 (High).
func (c *Console) drawBCD(hl, de uint16, n int) {
	for ; n > 0; n-- {
		a := c.rd(hl)
		c.writeVRAM(de, a>>4|0x30)
		de++
		c.writeVRAM(de, a&0x0F|0x30)
		hl--
		de++
	}
}

// drawLives draws up to five life icons ($2E) right to left from $383D,
// blanking the rest.
//
// Ported from $43F1-$4408 (High). Once a blank is drawn A stays 0, so
// every later position is blank too.
func (c *Console) drawLives() {
	de := uint16(0x383D)
	n := c.RAM[0x050]
	a := byte(0x2E)
	for i := 0; i < 5; i++ {
		n--
		if n&0x80 != 0 {
			a = 0
		}
		c.writeVRAM(de, a)
		de--
	}
}

// addScore adds the BCD value de (D high, E low) to the current player's
// score during a game, awarding an extra life when the top byte passes
// RAM[$052] and updating the hi-score.
//
// Ported from $4308-$436F (High). Two quirks are kept: on overflow past
// 999999 the score wraps while the hi-score is set to 999999 by two
// overlapping 16-bit stores; and the extra-life threshold saturates at $FF
// when it would pass 99.
func (c *Console) addScore(de uint16) {
	mode := c.RAM[0x002]
	if mode&0x40 == 0 {
		return
	}
	hl := uint16(0xE049)
	if mode&0x80 != 0 {
		hl = 0xE046
	}
	e, cy := addDAA(c.rd(hl), byte(de), false)
	c.wr(hl, e)
	hl++
	d, cy := addDAA(c.rd(hl), byte(de>>8), cy)
	c.wr(hl, d)
	hl++
	de = uint16(d)<<8 | uint16(e)
	if cy {
		top, cy := addDAA(c.rd(hl), 1, false)
		c.wr(hl, top)
		if cy {
			c.wr16(0xE043, 0x9999)
			c.wr16(0xE044, 0x9999)
			c.drawHiScore()
			c.drawPlayerScore(c.RAM[0x002])
			return
		}
		if c.RAM[0x052] < top {
			t, cy := addDAA(c.RAM[0x052], 2, false)
			if cy {
				t = 0xFF
			}
			c.RAM[0x052] = t
			c.RAM[0x050]++
			c.drawLives()
			c.playSound(0x08)
		}
	}
	top := c.rd(hl)
	hi := c.RAM[0x045]
	switch {
	case hi < top:
	case hi > top:
		c.drawPlayerScore(c.RAM[0x002])
		return
	default:
		if c.rd16(0xE043) >= de {
			c.drawPlayerScore(c.RAM[0x002])
			return
		}
	}
	c.wr16(0xE043, de)
	c.RAM[0x045] = top
	c.drawHiScore()
	c.drawPlayerScore(c.RAM[0x002])
}

// drawHeightLabel draws "HEIGHT" and the current height.
// Ported from $66ED-$66F2 (High).
func (c *Console) drawHeightLabel() {
	c.drawText(0x6718, 0xFF)
	c.drawHeight()
}

// drawHeight converts the height RAM[$054-$055] / 16 to BCD in
// RAM[$1BC-$1BD] and draws it at $385A.
//
// Ported from $66F3-$6712 (High).
func (c *Console) drawHeight() {
	h := c.rd16(0xE054) >> 4
	c.wr16(0xE1BC, binToBCD(h))
	c.drawBCD(0xE1BD, 0x385A, 2)
}

// binToBCD converts a 16-bit value to 4 BCD digits (the top digits are
// lost for values over 9999).
//
// Ported from $6726-$6736 (High): 16 rounds of shift-in and BCD doubling.
func binToBCD(hl uint16) uint16 {
	var e, d byte
	for i := 0; i < 16; i++ {
		cy := hl&0x8000 != 0
		hl <<= 1
		e, cy = addDAA(e, e, cy)
		d, _ = addDAA(d, d, cy)
	}
	return uint16(d)<<8 | uint16(e)
}

// drawTextPlayer draws a text table and, for player 2, overwrites the
// character before the end with '2'.
//
// Ported from $42D1-$42DF (High).
func (c *Console) drawTextPlayer(addr uint16) {
	de := c.drawText(addr, 0xFF)
	if c.RAM[0x002]&0x80 != 0 {
		c.writeVRAM(de-1, 0x32)
	}
}
