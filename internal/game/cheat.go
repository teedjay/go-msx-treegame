package game

// Cheat: start a game at any of the 9 stages from the menu.
//
// This is an extension, not ported code: the original has no level select.
// It touches the engine at one point only (state8, right after the new-game
// variables are loaded) and is inactive unless CheatStart is called, so the
// lockstep comparison against the original is unaffected.

// CheatStart starts a game from the menu (state 5) at stage n (1-9), in
// the mode of the currently selected menu line, exactly as a trigger press
// would ($45B4-$45CC), and remembers n for state8. It does nothing outside
// the menu or for other n.
func (c *Console) CheatStart(n int) {
	if c.RAM[0x000] != 5 || n < 1 || n > 9 {
		return
	}
	c.startStage = byte(n)
	c.RAM[0x004] = 0
	c.RAM[0x002] = romByte(0x45E7 + uint16(c.RAM[0x042]))
	c.RAM[0x000], c.RAM[0x001] = 0x08, 0x00
}

// applyStartStage overrides the stage variables loaded by newGameVars:
// RAM[$05C] stage index (0-8), RAM[$051] stage number (BCD), RAM[$05D]
// stage colour, chosen the way the stage advance at $741A-$743B does
// ($77 for the first and ninth stage, else the table at $745A). Player 2's
// copy at $E080 gets the same in a 2-player game.
func (c *Console) applyStartStage() {
	n := c.startStage
	c.startStage = 0
	if n == 0 {
		return
	}
	idx := n - 1
	col := byte(0x77)
	if idx != 0 && idx < 9 {
		col = romByte(0x745A + uint16(idx&7))
	}
	set := func(base uint16) {
		c.RAM[base+0x0C] = idx // $E05C / $E08C
		c.RAM[base+0x01] = n   // $E051 / $E081: 1-9 is the same in BCD
		c.RAM[base+0x0D] = col // $E05D / $E08D
	}
	set(0x050)
	if c.RAM[0x002]&0x20 != 0 {
		set(0x080)
	}
}
