package game

// The player. RAM (High for the offsets, Medium for the names):
// $1B2 player state; $1B3 y; $1B4 x; $1B5 animation frame; $1B6 input
// direction; $1B7 facing; $1B8 walk counter; $1B9-$1BB jump start y and
// height; $1BE last vertical step; $1BF-$1C0 tiles below the player;
// $1C7 jump direction; $1C8 jump step; $1C9-$1CA jump arc pointer;
// $1CB-$1CE object landed on (y, x, type, width); $1CF-$1D1 climbable
// object found, $1D2-$1D4 its table record, $1D9-$1DA its match.
// RAM[$057]/RAM[$058] are the right/left walking limits.

// playerUpdate runs the player state handler for RAM[$1B2], then rebuilds
// the player's sprites (the continuation $61D1, JP $65B5, pushed before
// the dispatch).
//
// Ported from $61A1-$61D3 (High) with the table at $61AB (19 entries).
func (c *Console) playerUpdate() {
	if h := playerHandlers[c.RAM[0x1B2]%19]; h != nil {
		h(c)
	}
	c.buildPlayerSprites()
}

var playerHandlers = [19]func(*Console){
	(*Console).pl61F0, // 0 standing
	(*Console).pl633C, // 1 jumping
	(*Console).pl6215, // 2 on a branch
	(*Console).pl633C, // 3 jumping
	(*Console).pl63F4, // 4 falling
	(*Console).pl6412, // 5 hanging
	(*Console).pl6988, // 6 climbing
	(*Console).pl6A80, // 7 on a ladder
	(*Console).pl7365, // 8 hit
	(*Console).pl7386, // 9 falling off
	nil,               // 10 $73B5: RET
	nil,               // 11 $73B5: RET
	(*Console).pl73D0, // 12 reached the top of a section
	(*Console).pl7462, // 13
	(*Console).pl7481, // 14 walk to the castle
	(*Console).pl74A1, // 15 stage clear
	(*Console).pl7533, // 16
	(*Console).pl756F, // 17
	(*Console).pl757C, // 18
}

func (c *Console) setPlayerState(s byte) { c.RAM[0x1B2] = s } // $64B7
func (c *Console) nextPlayerState()      { c.RAM[0x1B2]++ }   // $64BC

// readTilesBelow stores the two tiles under the player in RAM[$1BF-$1C0].
// Ported from $61D4-$61EF (High). The +$0408 is a 16-bit add.
func (c *Console) readTilesBelow() {
	hl := c.rd16(0xE1B3) + 0x0408
	de := nameAddr(byte(hl), byte(hl>>8))
	c.RAM[0x1BF] = c.readVRAM(de)
	c.RAM[0x1C0] = c.readVRAM(de + 0x20)
}

// onVine reports whether either tile below the player is $9B.
// Ported from $6496-$64A0 (High).
func (c *Console) onVine() bool {
	c.readTilesBelow()
	return c.RAM[0x1C0] == 0x9B || c.RAM[0x1BF] == 0x9B
}

// newPress returns the newly pressed bits of mask in RAM[$009].
// Ported from $648C-$6495 (mask 3) and $64E8-$64F1 (mask $30) (High).
func (c *Console) newPress(mask byte) byte {
	return ^c.RAM[0x008] & mask & c.RAM[0x009]
}

// climbStep moves the player 8 down the screen (up the tree view) with a
// redraw. Ported from $64A1-$64B5 (High). Returns the new y.
func (c *Console) climbStep() byte {
	c.RAM[0x1B3] += 8
	c.buildPlayerSprites()
	c.copySprites()
	c.scrollUp()
	return c.RAM[0x1B3]
}

// walk moves the player left or right from RAM[$009] and sets the walking
// frame (3 when standing still).
//
// Ported from $64C1-$64E7 (High).
func (c *Console) walk() {
	a := c.RAM[0x009] & 0x0C
	c.RAM[0x1B6] = a
	if a == 0 {
		c.RAM[0x1B5] = 3
		return
	}
	c.RAM[0x1B8]++
	c.RAM[0x1B7] = a
	c.stepX(0xE1B4, 1)
	c.RAM[0x1B5] = (c.RAM[0x1B8] & 0x0C) >> 2
}

// stepX moves the byte at hl by c in direction RAM[$1B6], within the
// limits RAM[$057] (right) and RAM[$058] (left).
//
// Ported from $6561-$657E (High).
func (c *Console) stepX(hl uint16, n byte) {
	d := c.RAM[0x1B6]
	if d == 0 {
		return
	}
	a := c.rd(hl)
	if d&0x08 != 0 {
		a += n
		if a >= c.RAM[0x057] {
			a -= n
		}
	} else {
		a -= n
		if a < c.RAM[0x058] {
			a += n
		}
	}
	c.wr(hl, a)
}

// startJump sets up a jump along the arc at hl. Ported from $657F-$65A1
// (High): $657F plays sound 3 and uses direction 0, $6589 uses 1.
func (c *Console) startJump(hl uint16, dir byte) {
	if dir == 0 {
		c.playSound(0x03)
	}
	c.wr16(0xE1C9, hl)
	c.RAM[0x1C7] = dir
	c.RAM[0x1C8] = 0
	c.RAM[0x1B9] = c.RAM[0x1B3]
	c.wr16(0xE1BA, c.rd16(0xE054))
}

// jump starts a jump (state 3). Ported from $630C-$6316 (High).
func (c *Console) jump(hl uint16) {
	c.startJump(hl, 0)
	c.setPlayerState(3)
}

// fall starts a fall (state 4). Ported from $6317-$6326 (High).
func (c *Console) fall() {
	c.startJump(0x65A3, 1)
	c.playSound(0x05)
	c.setPlayerState(4)
}

// arcStep advances the jump arc by c pixels per step: y follows the arc
// table, x moves with the input.
//
// Ported from $64F2-$6560 (High).
func (c *Console) arcStep(n byte) {
	b := c.RAM[0x1C7]
	if b&1 == 0 {
		c.RAM[0x1C8]++
	} else {
		c.RAM[0x1C8]--
	}
	frame := byte(3)
	if c.RAM[0x1C8] >= 8 {
		frame = 5
	}
	c.RAM[0x1B5] = frame
	de := c.rd16(0xE1C9)
	dy := c.rd(de)
	if b&1 == 0 {
		dy = -dy
	}
	step := dy
	y := dy + c.RAM[0x1B3]
	if y < 0x18 {
		y = c.RAM[0x1B3]
		step = 0
	}
	c.RAM[0x1B3] = y
	c.RAM[0x1BE] = step
	if b&1 != 0 {
		n = 1
	}
	c.addHeight(-step)
	c.stepX(0xE1B4, n)
	if b&1 == 0 {
		de++
	} else {
		de--
	}
	switch c.rd(de) {
	case 0xFF:
		de -= 2
		c.RAM[0x1C7] = 1
	case 0xFE:
		de++
	}
	c.wr16(0xE1C9, de)
}

// findClimbable finds the first object of type $87, $0A or $8A, copies it
// to RAM[$1CF-$1D1] and its record from the table at $6A73 to
// RAM[$1D2-$1D4].
//
// Ported from $6A34-$6A72 (High).
func (c *Console) findClimbable() bool {
	hl := uint16(0xE132)
	for i := 0; i < 0x28; i++ {
		y, t := c.rd(hl), c.rd(hl+2)
		if y != 0xD0 && (t == 0x87 || t == 0x0A || t == 0x8A) {
			c.ldir(hl, 0xE1CF, 3)
			for r := uint16(0x6A73); ; r += 4 {
				if c.rd(r) == t {
					c.ldir(r+1, 0xE1D2, 3)
					return true
				}
				if c.rd(r)+1 == 0 {
					return true // end of table: never reached for these types
				}
			}
		}
		hl += 3
	}
	return false
}

// nearClimbable checks the player's x against the record's offsets.
// Ported from $6A18-$6A33 (High).
func (c *Console) nearClimbable() bool {
	hl := uint16(0xE1D2)
	for b := c.rd(hl); ; {
		hl++
		if c.RAM[0x1B4]-c.rd(hl) < 8 {
			c.RAM[0x1DA] = c.rd(hl)
			c.RAM[0x1D9] = c.RAM[0x1CF]
			return true
		}
		b--
		if b == 0 {
			return false
		}
	}
}

// startClimb. Ported from $62E1-$62EA (High).
func (c *Console) startClimb() {
	c.RAM[0x1B5] = 6
	c.setPlayerState(6)
}

// pl61F0: standing. Ported from $61F0-$620E (High).
func (c *Console) pl61F0() {
	c.walk()
	if c.newPress(0x30) != 0 {
		c.jump(0x65A3)
		return
	}
	if c.RAM[0x009]&1 == 0 || !c.findClimbable() || !c.nearClimbable() {
		return
	}
	if c.RAM[0x1D1] == 0x0A {
		c.startClimb()
	}
}

// pl6215: on a branch, picking up items. Ported from $6215-$62E0 (High).
func (c *Console) pl6215() {
	if !c.onBranch() {
		c.fall()
		return
	}
	if c.RAM[0x009]&0x0C != 0 {
		c.pickUp6223()
	}
	c.walk()
	if c.newPress(0x30) != 0 {
		c.jump(0x65A3)
		return
	}
	p := c.newPress(0x03)
	if p == 0 {
		return
	}
	if !c.findClimbable() || !c.nearClimbable() {
		if p&2 == 0 {
			return
		}
		c.dropDown()
		return
	}
	if p&1 != 0 {
		hl := c.rd16(0xE1B3)
		if c.tileAt(byte(hl)-8, byte(hl>>8)+4)-0x78 < 3 {
			c.startClimb()
		}
		return
	}
	if c.RAM[0x1D9]-c.RAM[0x1B3] >= 0x24 {
		c.dropDown()
		return
	}
	c.descend4()
	c.startClimb()
}

// dropDown. Ported from $6304-$630B (High).
func (c *Console) dropDown() {
	c.descend4()
	c.setPlayerState(5)
}

// descend4 moves down 4 rows with redraws. Ported from $6327-$633B (High).
func (c *Console) descend4() {
	for i := 0; i < 4; i++ {
		c.movePlayerBy(8)
		c.buildPlayerSprites()
		c.copySprites()
		c.scrollDownIfLow()
	}
}

// onBranch reports whether the player's x is within the branch landed on.
// Ported from $66E0-$66EC (High).
func (c *Console) onBranch() bool {
	return c.RAM[0x1B4]-c.RAM[0x1CC]+8 < c.RAM[0x1CE]
}

// pickUp6223 collects an item object ($C5-$CA) next to the player.
// Ported from $6223-$62B7 (High).
func (c *Console) pickUp6223() {
	hl := uint16(0xE134)
	for i := 0; i < 0x28; i++ {
		if t := c.rd(hl); t-0xC5 < 6 {
			y := c.rd(hl - 2)
			if y != 0xD0 && y-c.RAM[0x1B3] < 0x18 && c.rd(hl-1)+8-c.RAM[0x1B4] < 0x10 {
				c.collect6254(hl - 2)
				return
			}
		}
		hl += 3
	}
}

func (c *Console) collect6254(slot uint16) {
	c.eraseObject(slot)
	y := c.rd(slot)
	c.wr(slot, 0xD0)
	x := c.rd(slot + 1)
	t := c.rd(slot + 2)
	if t-0xC5 < 3 {
		c.RAM[0x05F]--
	} else {
		// Mark the map record so the item does not come back: find the
		// row of y in the map, then the record of this type, and change it
		// to $CB.
		de := c.rd16(0xE05A)
		rows := ((0xB8 - y) & 0xF8) >> 3
		a := c.RAM[0x059] + rows
		for {
			v := c.readVRAM(de)
			a -= v
			if a == 0 {
				break
			}
			de += 3
		}
		for {
			de++
			if c.readVRAM(de) == t {
				c.writeVRAM(de, 0xCB)
				break
			}
			de += 2
			if c.readVRAM(de) != 0 {
				break
			}
		}
	}
	c.scorePop(c.rd(0x620F+uint16(t-0xC5)), x, y)
}

// pl633C: jumping. Ported from $633C-$63BE (High).
func (c *Console) pl633C() {
	c.arcStep(2)
	if c.onVine() {
		c.grabVine()
		return
	}
	if c.RAM[0x1C7] == 0 {
		if -c.RAM[0x1BE] >= 5 {
			c.scrollUpIfHigh()
		}
		return
	}
	c.scrollDownIfLow()
	hl := c.rd16(0xE1B3) + 0x18
	if !c.landOn(byte(hl), byte(hl>>8)) {
		c.landFloor()
		return
	}
	c.playSound(0x02)
	if c.RAM[0x1C8]&0x80 != 0 {
		// SBC HL,BC twice: the borrow of the first feeds the second, so
		// ending above the jump start also counts as a long fall.
		start, cur := c.rd16(0xE1BA), c.rd16(0xE054)
		borrow := uint16(0)
		if start < cur {
			borrow = 1
		}
		if !(uint32(start-cur) < 0x80+uint32(borrow)) {
			c.setPlayerState(4)
			return
		}
	}
	c.landBranch()
}

// landBranch puts the player on the branch at RAM[$1CB].
// Ported from $6365-$6374 (High).
func (c *Console) landBranch() {
	c.movePlayerY(c.RAM[0x1CB] - 0x20)
	c.scrollToTop()
	c.setPlayerState(2)
}

// landFloor stops at the bottom of the view (y=$98), state 0.
// Ported from $638A-$6398 (High).
func (c *Console) landFloor() {
	if c.RAM[0x1B3] <= 0x98 {
		return
	}
	c.movePlayerBy(0x98 - c.RAM[0x1B3])
	c.setPlayerState(0)
}

// scrollToTop climbs until the player is at y >= $48.
// Ported from $6399-$63A3 (High).
func (c *Console) scrollToTop() {
	for c.RAM[0x1B3] < 0x48 {
		c.climbStep()
	}
}

// grabVine. Ported from $63A4-$63BE (High).
func (c *Console) grabVine() {
	c.scrollToTop()
	c.playSound(0x02)
	c.RAM[0x1B4] = (c.RAM[0x1B4]+4)&0xF8 - 4
	c.RAM[0x1B5] = 0x0A
	c.setPlayerState(7)
}

// scrollUpIfHigh scrolls up one row while the player is at y <= $40.
// Ported from $63C7-$63DC (High).
func (c *Console) scrollUpIfHigh() {
	a := 0x48 - int(c.RAM[0x1B3])
	if a < 0 || a < 8 || c.RAM[0x1AB]&1 != 0 {
		return
	}
	c.RAM[0x1B3] += 8
	c.scrollUp()
}

// scrollDownIfLow scrolls down one row while the player is at y >= $90.
// Ported from $63DD-$63F3 (High).
func (c *Console) scrollDownIfLow() {
	a := int(c.RAM[0x1B3]) - 0x88
	if a < 8 || c.RAM[0x1AB]&2 != 0 {
		return
	}
	c.RAM[0x1B3] -= 8
	c.scrollDown()
}

// landOn finds a branch object (type $00-$0F) under (y, x); it copies it to
// RAM[$1CB-$1CD] with its width in pixels+1 in RAM[$1CE].
//
// Ported from $6696-$66D5 + $66D6-$66DF (High).
func (c *Console) landOn(y, x byte) bool {
	hl := uint16(0xE132)
	for i := 0; i < 0x28; i++ {
		if c.rd(hl) != 0xD0 && c.rd(hl+2)&0xF0 == 0 {
			_, _, w := c.shapeInfo(hl)
			wpx := w*8 + 1
			if y-c.rd(hl) < 9 && x-c.rd(hl+1)+8 < wpx {
				c.ldir(hl, 0xE1CB, 3)
				c.RAM[0x1CE] = wpx
				return true
			}
		}
		hl += 3
	}
	return false
}

// pl63F4: falling. Ported from $63F4-$6411 (High).
func (c *Console) pl63F4() {
	c.arcStep(1)
	c.scrollDownIfLow()
	hl := c.rd16(0xE1B3)
	if !c.landOn(byte(hl), byte(hl>>8)) {
		c.landFloor()
		return
	}
	c.movePlayerY(c.RAM[0x1CB])
	c.playSound(0x06)
	c.setPlayerState(5)
}

// pl6412: hanging from a branch. Ported from $6412-$648B (High).
func (c *Console) pl6412() {
	c.RAM[0x1B7] = c.RAM[0x009]
	if !c.onBranch() {
		c.fall()
		return
	}
	if c.RAM[0x003]&1 != 0 {
		c.walk()
	}
	if c.RAM[0x009]&0x0C != 0 {
		c.playSound(0x02)
	}
	frame := byte(8)
	if c.RAM[0x1B8]&0x04 == 0 && c.RAM[0x009] != 0 {
		frame = 9
	}
	c.RAM[0x1B5] = frame
	p := c.newPress(0x03)
	if p == 0 {
		return
	}
	if p&1 != 0 {
		for i := 0; i < 4; i++ {
			c.movePlayerBy(0xF8)
			c.buildPlayerSprites()
			c.copySprites()
			c.scrollUpIfHigh()
		}
		c.setPlayerState(2)
		return
	}
	if c.findClimbable() && c.nearClimbable() && c.RAM[0x1D9] == c.RAM[0x1B3] {
		c.startClimb()
		return
	}
	if c.onVine() {
		c.grabVine()
		return
	}
	c.startJump(0x65A3, 1)
	c.setPlayerState(3)
}

// climbAnim moves the player vertically from the input and sets the climb
// frame (base, or base+1 when y bit 2 is clear).
// Ported from $69E9-$69FF + $6A00-$6A17 (High).
func (c *Console) climbAnim(base byte) {
	a := c.RAM[0x009] & 3
	c.RAM[0x1B6] = a
	if a != 0 {
		var d byte
		switch a {
		case 1:
			d = 0xFF
		case 2:
			d = 1
		default:
			d = 0
		}
		c.movePlayerBy(d)
		c.playSound(0x02)
	}
	if c.RAM[0x1B3]&0x04 == 0 {
		base++
	}
	c.RAM[0x1B5] = base
}

// pl6988: climbing the trunk. Ported from $6988-$69E8 (High).
func (c *Console) pl6988() {
	if c.RAM[0x009]&1 != 0 && c.RAM[0x003]&1 == 0 {
		return
	}
	a := c.RAM[0x009] & 3
	if a == 0 {
		return
	}
	if a&1 == 0 {
		c.climbAnim(6)
		c.findClimbable()
		c.scrollDownIfLow()
		hl := c.rd16(0xE1B3)
		if !c.landOn(byte(hl)+0x24, byte(hl>>8)) {
			c.landFloor()
			return
		}
		c.landBranch()
		return
	}
	c.climbAnim(6)
	c.scrollUpIfHigh()
	c.findClimbable()
	b := c.RAM[0x1B3] + 8
	y := c.RAM[0x1CF]
	if y >= 0xC0 || y < b {
		return
	}
	_, _, w := c.shapeInfo(0xE1CF)
	c.ldir(0xE1CF, 0xE1CB, 3)
	c.RAM[0x1CE] = w*8 + 1
	t := c.RAM[0x1CD]
	if t == 0x0A {
		c.landBranch()
		return
	}
	c.RAM[0x056] = t
	c.reachTop()
}

// pl6A80: on a ladder or vine. Ported from $6A80-$6AE9 (High).
func (c *Console) pl6A80() {
	b := c.RAM[0x009]
	if b&0x0C != 0 && c.newPress(0x30) != 0 {
		hl := c.rd16(0xE1B3)
		dx := byte(0xFB)
		if b&0x08 != 0 {
			dx = 0x0D
		}
		hl = hl&0x00FF | uint16(byte(hl>>8)+dx)<<8
		c.wr16(0xE1B3, hl)
		c.RAM[0x1B5] = 5
		c.RAM[0x1B6] = b & 0x0C
		c.RAM[0x1B7] = b & 0x0C
		c.startJump(0x65A7, 0)
		c.setPlayerState(3)
		return
	}
	a := b & 3
	if a == 0 {
		return
	}
	if a&1 != 0 {
		if c.RAM[0x003]&1 != 0 {
			return
		}
		c.climbAnim(0x0A)
		c.scrollUpIfHigh()
		hl := c.rd16(0xE1B3) + 0x0C
		if c.landOn(byte(hl), byte(hl>>8)) {
			c.landBranch()
		}
		return
	}
	c.climbAnim(0x0A)
	c.scrollDownIfLow()
	c.readTilesBelow()
	if c.RAM[0x1BF] != 3 {
		return
	}
	c.movePlayerBy(8)
	c.fall()
}

// pl7365: hit. Ported from $7365-$7385 (High).
func (c *Console) pl7365() {
	c.RAM[0x1B5] = 0x0C
	if c.RAM[0x003]&0x1F != 0 {
		return
	}
	c.RAM[0x1B5]++
	c.RAM[0x1B6] = 0
	c.startJump(0x65A3, 1)
	c.RAM[0x032] = 0
	c.playSound(0x0B)
	c.nextPlayerState()
}

// pl7386: falling off the screen. Ported from $7386-$73B4 (High).
func (c *Console) pl7386() {
	f := c.RAM[0x003]
	if f&1 == 0 {
		return
	}
	c.arcStep(1)
	c.scrollDownIfLow()
	frame := byte(0x0D)
	if (f>>1)&0x04 != 0 {
		frame = 0x0C
	}
	c.RAM[0x1B5] = frame
	if c.RAM[0x1B3] <= 0x98 {
		return
	}
	c.movePlayerBy(0x98 - c.RAM[0x1B3])
	c.playSound(0x9F)
	c.RAM[0x1B3] = 0xC3
	c.RAM[0x00C] = 0xC3
}

// reachTop: the player reached the top of a section.
// Ported from $73B6-$73CF (High).
func (c *Console) reachTop() {
	c.playSound(0x9F)
	c.movePlayerY(c.RAM[0x1CB] - 0x20)
	c.RAM[0x1B5] = 1
	c.RAM[0x001] = 1
	c.setPlayerState(0x0C)
}

// pl73D0: scroll the player down to the bottom, clear the enemies, and
// either continue the stage or start the next one.
//
// Ported from $73D0-$7461 (High). The scroll loop runs within one tick.
func (c *Console) pl73D0() {
	for c.climbStep() != 0x98 {
	}
	c.RAM[0x1F9] = 0
	c.ldir(0xE1F9, 0xE1FA, 0x5C)
	c.RAM[0x0C0] = 0xC3
	c.ldir(0xE0C0, 0xE0C1, 0x0F)
	c.RAM[0x05E]++
	if c.RAM[0x05E]&1 != 0 {
		c.wr16(0xE057, 0x18DF)
	} else {
		c.wr16(0xE057, 0x03F0)
	}
	c.RAM[0x1AB] |= 0x02
	if c.readVRAM(c.rd16(0xE1C5)+1) != 0xFF {
		c.nextPlayerState()
		return
	}
	c.RAM[0x051], _ = addDAA(c.RAM[0x051], 1, false)
	c.RAM[0x05C]++
	b := c.RAM[0x05C]
	if b == 9 {
		c.wr16(0xE250, 0x77C5)
	} else {
		c.wr16(0xE250, 0x7709)
	}
	col := byte(0x77)
	if b < 9 {
		col = c.rd(0x745A + uint16(b&7))
	}
	c.RAM[0x05D] = col
	c.playSound(0x90)
	c.writeStageColours()
	c.triplicateColours()
	c.buildStageMap()
	c.drawHUD()
	c.setPlayerState(0x0E)
}

// pl7462. Ported from $7462-$7480 (High).
func (c *Console) pl7462() {
	c.RAM[0x003] = 0
	c.RAM[0x05F] = 4
	if c.RAM[0x05E]&1 != 0 {
		c.playSound(0x09)
	} else {
		c.playSound(0x8C)
	}
	c.resumePlay()
}

// resumePlay. Ported from $7478-$7480 (High).
func (c *Console) resumePlay() {
	c.RAM[0x001] = 0
	c.setPlayerState(0)
}

// pl7481: walk to the castle while its picture is drawn.
// Ported from $7481-$74A0 (High).
func (c *Console) pl7481() {
	c.revealColumn()
	in := byte(8)
	if c.RAM[0x1B4] >= 0x98 {
		in = 4
	}
	c.RAM[0x009] = in
	c.walk()
	if c.RAM[0x1B4] != 0x98 {
		return
	}
	c.RAM[0x1B7] = 8
	c.nextPlayerState()
}

// pl74A1: stage clear, bonus. Ported from $74A1-$7532 (High).
func (c *Console) pl74A1() {
	done := c.revealColumn()
	if c.RAM[0x05C] == 9 && c.RAM[0x004] < 0x15 {
		c.drawPicture(0x7629)
	}
	frame := byte(2)
	if c.RAM[0x003]&0x10 != 0 {
		frame = 1
	}
	c.RAM[0x1B5] = frame
	if !done || c.RAM[0x012] != 0 {
		return
	}
	if c.RAM[0x05C] == 9 {
		c.drawPicture(0x763B)
		c.drawText(0x75D0, 0xFF)
		c.endStageClear()
		return
	}
	c.drawText(0x75A0, 0xFF)
	c.wr16(0xE241, 0x2000)
	c.addScore(0x2000)
	c.drawBCD(0xE242, 0x3991, 2)
	c.drawShape(0x88, 0xB8, 0x75E2, 6, 8)
	bc := c.rd16(0x7627)
	lo, cy := subDAA(byte(bc), c.RAM[0x1BC], false)
	c.RAM[0x241] = lo
	c.RAM[0x242], _ = subDAA(byte(bc>>8), c.RAM[0x1BD], cy)
	c.drawBCD(0xE242, 0x3A79, 2)
	c.endStageClear()
}

// endStageClear. Ported from $7514-$7524 (High).
func (c *Console) endStageClear() {
	c.playSound(0x08)
	c.RAM[0x1B5] = 0
	c.RAM[0x003] = 0
	c.nextPlayerState()
}

// pl7533. Ported from $7533-$756E (High).
func (c *Console) pl7533() {
	if c.RAM[0x003]&0x7F != 0 {
		return
	}
	if c.RAM[0x05C] == 9 {
		c.RAM[0x003] = 0
		c.nextPlayerState()
		return
	}
	c.writeRLE("name_75b8")
	c.drawShape(0x88, 0xB8, 0x7615, 6, 8)
	c.restartSection(0x80)
}

// restartSection. Ported from $7554-$756E (High).
func (c *Console) restartSection(a byte) {
	c.RAM[0x1AB] = a
	c.drawHUD()
	c.RAM[0x059] = 0
	c.wr16(0xE05A, 0x3B83)
	c.scrollInMap()
	c.RAM[0x00D] = 1
	c.resumePlay()
}

// pl756F. Ported from $756F-$757B (High).
func (c *Console) pl756F() {
	if c.RAM[0x003] != 0 {
		return
	}
	c.RAM[0x004] = 0x20
	c.nextPlayerState()
}

// pl757C. Ported from $757C-$758D (High).
func (c *Console) pl757C() {
	if !c.wipeColumn() {
		return
	}
	c.ldir(0x51F3, 0xE054, 0x0D)
	c.restartSection(0)
}

// drawPicture draws two 3x3 tile blocks from hl.
// Ported from $758E-$759F (High).
func (c *Console) drawPicture(hl uint16) {
	c.copyRows(hl, 0x390A, 3, 3)
	c.copyRows(hl+9, 0x3912, 3, 3)
}

// revealColumn draws one column of the picture every 8 ticks, columns
// alternating from the edges as RAM[$004] counts down. It returns true
// once RAM[$004] has gone negative.
//
// Ported from $764D-$769B (High). Columns are vertical RLE streams from
// the pointer list at RAM[$250-$251].
func (c *Console) revealColumn() bool {
	if c.RAM[0x003]&7 != 0 {
		return false
	}
	if c.dec(0xE004)&0x80 != 0 {
		return true
	}
	a := c.RAM[0x004]
	col := a >> 1
	if a&1 == 0 {
		col ^= 0x1F
	}
	de := uint16(0x3800) | uint16(col+0x60)
	hl := c.rd16(c.rd16(0xE250))
	for {
		ctl := c.rd(hl)
		hl++
		n := ctl & 0x7F
		if n == 0 {
			break
		}
		lit := ctl&0x80 != 0
		for ; n > 0; n-- {
			c.writeVRAM(de, c.rd(hl))
			if lit {
				hl++
			}
			de += 0x20
		}
		if !lit {
			hl++
		}
	}
	c.wr16(0xE250, c.rd16(0xE250)+2)
	return false
}

// buildPlayerSprites lays out the player's sprites at $E0D0 from the
// position, state and frame: 4 sprites (2x2 of 16x16) and a hidden fifth,
// or in the castle states (>= $0F) 5 visible sprites.
//
// Ported from $65B5-$6629 + $667D-$6695 (High).
func (c *Console) buildPlayerSprites() {
	b := c.RAM[0x1B3]
	x := c.RAM[0x1B4]
	f := c.RAM[0x1B5]
	hl := uint16(0xE0D0)
	switch st := c.RAM[0x1B2]; {
	case st == 0x05:
		c.RAM[0x1B7] = 8
	case st >= 0x0F:
		b += 2
		de := 0x6669 + uint16(f*4+f) // 5f, 8-bit
		c.spritePair(&hl, b, x, &de)
		b += 0x10
		c.spritePair(&hl, b, x, &de)
		c.RAM[0x0D9] = x - 8
		c.spriteOne(&hl, b, x+8, &de)
		return
	default:
		b += 4
		if st-6 < 6 { // states 6-11
			c.RAM[0x1B7] = 8
		}
	}
	if f&1 != 0 {
		b++
	}
	de := 0x6630 + uint16(f*4) // 8-bit
	c.spritePair(&hl, b, x, &de)
	b += 0x10
	c.spritePair(&hl, b, x, &de)
	c.wr(hl, 0xC3)
	if c.RAM[0x1B5] < 6 {
		return
	}
	c.RAM[0x0D4] -= 3
	c.RAM[0x0DC] -= 3
}

// spritePair writes two sprite entries ($667D calls $6680 twice).
func (c *Console) spritePair(hl *uint16, y, x byte, de *uint16) {
	c.spriteOne(hl, y, x, de)
	c.spriteOne(hl, y, x, de)
}

// spriteOne writes one sprite's y, x and pattern; the pattern is offset by
// $28 when facing left (RAM[$1B7] bit 3 clear). The colour byte is left.
// Ported from $6680-$6695 (High).
func (c *Console) spriteOne(hl *uint16, y, x byte, de *uint16) {
	c.wr(*hl, y)
	c.wr(*hl+1, x)
	p := c.rd(*de)
	if c.RAM[0x1B7]&0x08 == 0 {
		p += 0x28
	}
	c.wr(*hl+2, p)
	*de++
	*hl += 4
}
