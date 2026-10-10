package game

// Play: stage setup, the per-tick game loop and the demo.

// setupPlay prepares a new game or demo: clears the game variables, draws
// the HUD, loads and draws the stage, and points the demo script at its
// start.
//
// Ported from $53F0-$540B (High).
func (c *Console) setupPlay() {
	c.newGameVars()
	c.drawHUD()
	c.stageInit()
	c.RAM[0x00C] = 0
	c.RAM[0x001] = 0
	c.RAM[0x24D] = 0x96
	c.wr16(0xE24E, 0x5412)
}

// state7 runs the demo: scripted input, then a normal game tick.
//
// Ported from $414E-$4159 + $540C-$5411 (High). When the script ends
// (RAM[$00C] set) it moves to state 0 via $4219 with A=0.
func (c *Console) state7() {
	c.demoInput()
	c.gameTick()
	if c.RAM[0x00C] == 0 {
		return
	}
	c.setState(0)
}

// setState sets RAM[$000] and the state timer RAM[$004]=$20.
// Ported from $4219-$4221 (High).
func (c *Console) setState(s byte) {
	c.RAM[0x000] = s
	c.RAM[0x004] = 0x20
}

// demoInput feeds the demo script into RAM[$009]: (duration, input) pairs
// from $5412, ending at a $FF duration, which sets RAM[$00C].
//
// Ported from $53D0-$53EF (High).
func (c *Console) demoInput() {
	de := c.rd16(0xE24E)
	if c.dec(0xE24D) == 0 {
		de += 2
		a := c.rd(de)
		if a == 0xFF {
			c.RAM[0x00C] = a
			return
		}
		c.RAM[0x24D] = a
		c.wr16(0xE24E, de)
	}
	c.RAM[0x009] = c.rd(de + 1)
}

// gameTick is one tick of play.
//
// Ported from $520C-$5254 (High). While RAM[$001] is set (the player is
// dying or the stage is changing) only the player state machine runs.
func (c *Console) gameTick() {
	if c.RAM[0x001] != 0 {
		c.copySprites()
		c.playerUpdate()
		c.endGameTick()
		return
	}
	c.copySprites()
	if c.playerHit() {
		c.playSound(0x9C)
		c.RAM[0x1B2] = 8
		c.RAM[0x003] = 0
		c.RAM[0x001] = 1
		return
	}
	if hl, ok := c.playerOnBonus(); ok {
		c.collectBonus(hl)
	}
	c.playerUpdate()
	c.blinkItems()
	c.callBirds()
	c.updateWalkers()
	c.updateClimbers()
	c.spawnBeesForBirds()
	c.spawnFallers()
	c.spawnWorms()
	c.updateFallers()
	c.updateWorms()
	c.updateWormShots()
	c.updateShots()
	c.updateBees()
	c.updateScorePops()
	c.endGameTick()
}

// endGameTick redraws the height and clears the scroll request.
// Ported from $524D-$5254 (High).
func (c *Console) endGameTick() {
	c.drawHeight()
	c.RAM[0x1AA] = 0
}

// playerHit reports whether the player touches a deadly tile below them
// or one of the 8 sprites at $E0E4.
//
// Ported from $5270-$52B1 (High). Tiles $20-$2D are deadly; sprites with
// pattern $90 or $94 are not.
func (c *Console) playerHit() bool {
	c.readTilesBelow()
	switch c.RAM[0x1B2] {
	case 4:
		return false
	case 3:
		if c.RAM[0x1C8]&0x80 != 0 {
			return false
		}
	}
	for i := uint16(0); i < 2; i++ {
		if c.RAM[0x1BF+i]-0x20 < 0x0E {
			return true
		}
	}
	hl := uint16(0xE0E4)
	for i := 0; i < 8; i++ {
		if c.overlapsPlayer(hl, 0x10, 0x10) {
			if p := c.rd(hl + 2); p != 0x90 && p != 0x94 {
				return true
			}
		}
		hl += 4
	}
	return false
}

// overlapsPlayer tests the sprite (y, x) at hl against the player's
// position RAM[$1B3] (y), RAM[$1B4] (x), with box sizes d (x) and e (y).
//
// Ported from $52B2-$52CF (High).
func (c *Console) overlapsPlayer(hl uint16, d, e byte) bool {
	py, px := c.RAM[0x1B3], c.RAM[0x1B4]
	y := c.rd(hl)
	if !(py+8-y < e || py+0x18-y < e) {
		return false
	}
	x := c.rd(hl + 1)
	return px+6-x < d || px+0x0A-x < d
}

// copySprites copies the 32 sprite attributes at $E0B0 to VRAM $3B00.
// Ported from $52D0-$52DB (High).
func (c *Console) copySprites() {
	for i := uint16(0); i < 0x80; i++ {
		c.writeVRAM(0x3B00+i, c.RAM[0x0B0+i])
	}
}

// stageInit loads the stage graphics, clears the object tables, builds the
// stage map and the initial sprites.
//
// Ported from $5200-$520B (High).
func (c *Console) stageInit() {
	c.loadStageGraphics()
	c.clearObjects()
	c.initSprites()
	c.copySprites()
}

// clearObjects zeroes $E131-$E255, then builds the stage map.
// Ported from $52DC-$52E8 (High).
func (c *Console) clearObjects() {
	c.RAM[0x131] = 0
	c.ldir(0xE131, 0xE132, 0x124)
	c.buildStageMap()
}

// buildStageMap marks the 40 object slots at $E132 free ($D0) and writes
// the map for stage RAM[$05C] into spare VRAM from $3B80: a list of
// segment numbers from the table at $4E49, ending with segment $16 (which
// is drawn too).
//
// Ported from $52E9-$5313 (High).
func (c *Console) buildStageMap() {
	c.RAM[0x132] = 0xD0
	c.ldir(0xE132, 0xE133, 0x77)
	de := uint16(0x3B80)
	hl := c.rd16(0x4E49 + uint16(c.RAM[0x05C]*2))
	for {
		a := c.rd(hl)
		de = c.drawMapSegment(a, de)
		hl++
		if a == 0x16 {
			return
		}
	}
}

// drawMapSegment writes map segment n into VRAM at de as 3-byte records
// and returns the next address. The segment table at $4D5E holds a
// pointer and a record count per segment. A record is either $FF and 3
// raw bytes, or 2 bytes (b0, b1) stored as (b0&7, b1, b0&$F8).
//
// Ported from $5314-$534E (High). The index 3n is computed in 8 bits.
func (c *Console) drawMapSegment(n byte, de uint16) uint16 {
	t := 0x4D5E + uint16(n+n+n)
	hl := uint16(c.rd(t)) | uint16(c.rd(t+1))<<8
	for b := c.rd(t + 2); ; {
		a := c.rd(hl)
		if a == 0xFF {
			hl++
			for i := uint16(0); i < 3; i++ {
				c.writeVRAM(de+i, c.rd(hl+i))
			}
			hl += 3
		} else {
			c.writeVRAM(de, a&7)
			hl++
			c.writeVRAM(de+1, c.rd(hl))
			hl++
			c.writeVRAM(de+2, a&0xF8)
		}
		de += 3
		b--
		if b == 0 {
			return de
		}
	}
}

// initSprites scrolls the first map rows in, sets up the default sprite
// attributes and the player's start position.
//
// Ported from $534F-$5373 (High).
func (c *Console) initSprites() {
	c.scrollInMap()
	c.initSpriteTable()
	hl := uint16(0xE0D3)
	for de := uint16(0x53CB); de < 0x53D0; de++ {
		c.wr(hl, c.rd(de))
		hl += 4
	}
	c.ldir(0x53C6, 0xE1B3, 5)
	c.buildPlayerSprites()
}

// initSpriteTable copies the 4-byte template at $53C2 to sprites 0-3 and
// hides the other sprites (Y=$C3).
//
// Ported from $5374-$5390 (High).
func (c *Console) initSpriteTable() {
	de := uint16(0xE0B0)
	for i := 0; i < 4; i++ {
		c.ldir(0x53C2, de, 4)
		de += 4
	}
	c.wr(de, 0xC3)
	c.ldir(de, de+1, 0x6F)
}

// scrollInMap scrolls 26 rows of the map in from the stage start, keeping
// the scroll position, and requests a redraw.
//
// Ported from $5391-$53C1 (High). It reads the map through BIOS RDVRM.
func (c *Console) scrollInMap() {
	a := c.RAM[0x059]
	hl := c.rd16(0xE05A)
	c.wr16(0xE1C5, hl-3)
	c.RAM[0x1C4] = c.readVRAM(hl-3) - 1
	c.RAM[0x1AA] = 1
	for i := 0; i < 0x1A; i++ {
		c.scrollRowUp()
	}
	c.RAM[0x059] = a
	c.wr16(0xE05A, hl)
	c.RAM[0x1AB] |= 0x02
}

// newGameVars clears the game variables and loads the new-game values
// (from $51EF) into RAM[$050-$060], copied to $E080 for player 2 in a
// 2-player game.
//
// Ported from $51BC-$51EE (High).
func (c *Console) newGameVars() {
	hl, n := uint16(0xE049), 0xE7
	twoP := c.RAM[0x002]&0x20 != 0
	if twoP {
		hl, n = 0xE046, 0xEA
	}
	c.wr(hl, 0)
	c.ldir(hl, hl+1, n)
	c.ldir(0x51EF, 0xE050, 0x11)
	if twoP {
		c.ldir(0xE050, 0xE080, 0x20)
	}
}
