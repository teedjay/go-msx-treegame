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
// the menu or for other n, and reports whether it started a game.
func (c *Console) CheatStart(n int) bool {
	if c.RAM[0x000] != 5 || n < 1 || n > 9 {
		return false
	}
	c.startStage = byte(n)
	c.RAM[0x004] = 0
	c.RAM[0x002] = romByte(0x45E7 + uint16(c.RAM[0x042]))
	c.RAM[0x000], c.RAM[0x001] = 0x08, 0x00
	return true
}

// applyStartStage overrides the stage variables loaded by newGameVars:
// RAM[$05C] stage index (0-8), RAM[$051] stage number (BCD), RAM[$05D]
// stage colour, chosen the way the stage advance at $741A-$743B does
// ($77 for the first and ninth stage, else the table at $745A), and
// RAM[$056], the map record type that marks the bottom of the map for
// scrolling down (scrollDownSpawn). Player 2's copy at $E080 gets the same
// in a 2-player game.
//
// RAM[$056] matters: new-game data sets $84, the bottom record of stage
// 1's map, and in normal play $69CA stores the type of the object reached
// at the top of a stage, which is the next stage's bottom type ($8A for
// stages 2-9). Without it a fall on a cheat-started stage scrolled past
// the end of the map.
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
	bottom := bottomRecordType(idx)
	set := func(base uint16) {
		c.RAM[base+0x0C] = idx    // $E05C / $E08C
		c.RAM[base+0x01] = n      // $E051 / $E081: 1-9 is the same in BCD
		c.RAM[base+0x0D] = col    // $E05D / $E08D
		c.RAM[base+0x06] = bottom // $E056 / $E086
	}
	set(0x050)
	if c.RAM[0x002]&0x20 != 0 {
		set(0x080)
	}
}

// bottomRecordType returns the type byte of the first map record of stage
// idx, read from the tables buildStageMap and drawMapSegment use: the
// stage list at $4E49, its first segment, and that segment's first record
// (2-byte form (b0, type), or $FF followed by 3 raw bytes (x, type, y)).
func bottomRecordType(idx byte) byte {
	list := uint16(romByte(0x4E49+uint16(idx*2))) | uint16(romByte(0x4E4A+uint16(idx*2)))<<8
	seg := romByte(list)
	t := 0x4D5E + uint16(seg+seg+seg)
	rec := uint16(romByte(t)) | uint16(romByte(t+1))<<8
	if romByte(rec) == 0xFF {
		return romByte(rec + 2)
	}
	return romByte(rec + 1)
}
