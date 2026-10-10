package game

// Game flow: states 8-15 of the table at $40E2.

// state8 shows the title with the chosen menu line blinking, then starts
// the game.
//
// Ported from $422D-$4251 + $4252-$426C (High).
func (c *Console) state8() {
	if c.RAM[0x001] == 0 {
		c.playSound(0x96)
		c.RAM[0x004] = 0x50
		c.clearScreen()
		c.writeVDPReg(7, 0xE1)
		c.loadTitleLogo()
		c.revealTitle()
		c.RAM[0x001]++
		return
	}
	c.RAM[0x004]--
	if c.RAM[0x004] == 0 {
		c.newGameVars()
		c.advanceState(0x20)
		return
	}
	if c.RAM[0x004]&0x08 != 0 {
		c.revealTitle()
		return
	}
	c.fillVRAM(c.menuPointerAddr(), 0x1B, 0)
}

// state9 wipes the screen, takes a life and sets up the stage (unless
// RAM[$00D] says to keep going).
//
// Ported from $415A-$417C (High).
func (c *Console) state9() {
	if c.RAM[0x00D] == 0 {
		if !c.wipeColumn() {
			return
		}
		c.RAM[0x050]--
		c.drawHUD()
		if c.RAM[0x002]&0x20 != 0 {
			c.drawTextPlayer(0x47E4)
		}
		c.stageInit()
	}
	c.nextState()
}

// state10 waits for the tune, plays the stage tune and starts play.
// Ported from $417D-$4198 (High).
func (c *Console) state10() {
	if c.RAM[0x012] != 0 {
		return
	}
	if c.RAM[0x05E]&1 != 0 {
		c.playSound(0x09)
	} else {
		c.playSound(0x8C)
	}
	c.RAM[0x001] = 0
	c.RAM[0x00C], c.RAM[0x00D] = 0, 0
	c.advanceState(0x20)
}

// state11 plays until RAM[$00C] (life lost) or RAM[$00D] (section done).
// Ported from $41A3-$41B3 (High).
func (c *Console) state11() {
	c.gameTick()
	if c.RAM[0x00C] != 0 {
		c.advanceState(0x20)
		return
	}
	if c.RAM[0x00D] == 0 {
		return
	}
	c.setState(0x0F)
}

// state12: with lives left go on (state 9, or 13 to swap players in a
// 2-player game); otherwise show game over.
// Ported from $41B4-$41ED (High).
func (c *Console) state12() {
	if c.RAM[0x050] != 0 {
		if c.RAM[0x080] != 0 {
			c.nextState()
			return
		}
		c.setState(9)
		return
	}
	if c.RAM[0x012] != 0 {
		return
	}
	c.playSound(0x93)
	c.drawTextPlayer(0x47D7)
	c.drawText(0x426D, 0xFF)
	c.drawBCD(0xE1BD, 0x39F1, 2)
	c.drawHUD()
	c.RAM[0x000] = 0x0D
	c.advanceState(0)
}

// state13 swaps the two players' variables and continues at state 9.
// Ported from $41EE-$4201 (High) with $406B (swap B bytes).
func (c *Console) state13() {
	for i := uint16(0); i < 0x20; i++ {
		a, b := c.RAM[0x050+i], c.RAM[0x080+i]
		c.RAM[0x050+i], c.RAM[0x080+i] = b, a
	}
	c.RAM[0x002] ^= 0x80
	c.setState(9)
}

// state14 waits, then swaps to the other player if they have lives, or
// ends the game.
// Ported from $4202-$4218 (High).
func (c *Console) state14() {
	c.RAM[0x004]--
	if c.RAM[0x004] != 0 {
		return
	}
	if c.RAM[0x080] != 0 {
		c.setState(0x0D)
		return
	}
	c.RAM[0x002] &= 0xBF
	c.setState(0)
}

// state15 waits, then goes to state 9 (keeping the timer).
// Ported from $4222-$422C (High).
func (c *Console) state15() {
	c.RAM[0x004]--
	if c.RAM[0x004] != 0 {
		return
	}
	c.RAM[0x000] = 9
}
